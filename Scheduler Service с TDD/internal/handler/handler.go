// Package handler реализует HTTP-эндпоинт POST /schedule.
// Принимает задание и кладёт его в очередь Redis через asynq.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hibiken/asynq"

	"diplom/scheduler-service/internal/tasks"
)

// ─── Enqueuer interface ───────────────────────────────────────────────────────

// Enqueuer позволяет подменять asynq.Client в тестах.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// ─── Request / Response ───────────────────────────────────────────────────────

type ScheduleRequest struct {
	Brand      string `json:"brand"`
	URL        string `json:"url"`
	SourceType string `json:"source_type"`
}

type ScheduleResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ─── Validation ───────────────────────────────────────────────────────────────

var validSourceTypes = map[string]bool{
	"html": true,
	"api":  true,
	"js":   true,
}

func validateRequest(req ScheduleRequest) error {
	if strings.TrimSpace(req.Brand) == "" {
		return errors.New("brand is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(req.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("url must be an absolute URL")
	}
	if !validSourceTypes[req.SourceType] {
		return fmt.Errorf("source_type must be one of: html, api, js (got: %q)", req.SourceType)
	}
	return nil
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type Handler struct {
	mux      *http.ServeMux
	enqueuer Enqueuer
}

func New(enqueuer Enqueuer) *Handler {
	h := &Handler{
		mux:      http.NewServeMux(),
		enqueuer: enqueuer,
	}
	h.mux.HandleFunc("/schedule", h.handleSchedule)
	h.mux.HandleFunc("/health", h.handleHealth)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ─── POST /schedule ───────────────────────────────────────────────────────────

func (h *Handler) handleSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := validateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := tasks.NewScrapeTask(tasks.ScrapePayload{
		Brand:      req.Brand,
		URL:        req.URL,
		SourceType: req.SourceType,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create task: "+err.Error())
		return
	}

	info, err := h.enqueuer.Enqueue(task)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "enqueue: "+err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, ScheduleResponse{
		TaskID: info.ID,
		Status: "enqueued",
	})
}

// ─── GET /health ─────────────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
