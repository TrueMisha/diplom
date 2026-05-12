package handler

import (
	"encoding/json"
	"net/http"

	"github.com/brandmon/normalizer-service/internal/normalizer"
)

// ─── Request / Response types ────────────────────────────────────────────────

type NormalizeRequest struct {
	Text string `json:"text"`
}

type NormalizeResponse struct {
	Words       []string             `json:"words"`
	Frequencies []normalizer.WordFreq `json:"frequencies"`
}

type CooccurrenceRequest struct {
	Text   string `json:"text"`
	Target string `json:"target"`
	Window int    `json:"window"`
}

type CooccurrenceResponse struct {
	Target    string         `json:"target"`
	Neighbors map[string]int `json:"neighbors"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type Handler struct {
	mux *http.ServeMux
}

func New() *Handler {
	h := &Handler{mux: http.NewServeMux()}
	h.mux.HandleFunc("/normalize",    h.handleNormalize)
	h.mux.HandleFunc("/cooccurrence", h.handleCooccurrence)
	h.mux.HandleFunc("/health",       h.handleHealth)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ─── POST /normalize ─────────────────────────────────────────────────────────

func (h *Handler) handleNormalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req NormalizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	result, err := normalizer.Normalize(req.Text, normalizer.DefaultConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, NormalizeResponse{
		Words:       result.Words,
		Frequencies: result.Frequencies,
	})
}

// ─── POST /cooccurrence ───────────────────────────────────────────────────────

func (h *Handler) handleCooccurrence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CooccurrenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}

	if req.Window <= 0 {
		req.Window = 3 // default window
	}

	// Нормализуем текст перед построением co-occurrence
	result, err := normalizer.Normalize(req.Text, normalizer.DefaultConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	neighbors := normalizer.BuildCooccurrence(result.Words, req.Target, req.Window)

	writeJSON(w, http.StatusOK, CooccurrenceResponse{
		Target:    req.Target,
		Neighbors: neighbors,
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
