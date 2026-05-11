package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"diplom/scraper-service/internal/parser"
	"diplom/scraper-service/internal/scraper"
)

// ─── Modes ────────────────────────────────────────────────────────────────────

type Mode string

const (
	ModeProduction Mode = "production"
	ModeTest       Mode = "test"
)

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	Mode       Mode
	Dispatcher scraper.Interface // nil допустим только в ModeTest
	Timeout    time.Duration     // макс. время скрапинга (0 → 10 минут)
}

// ─── Request / Response ───────────────────────────────────────────────────────

type ScrapeRequest struct {
	Brand      string `json:"brand"`
	URL        string `json:"url"`
	SourceType string `json:"source_type"`
}

// ScrapeResponse возвращается при успешном парсинге.
type ScrapeResponse struct {
	JobID     string          `json:"job_id"`
	Brand     string          `json:"brand"`
	Source    string          `json:"source"`
	URL       string          `json:"url"`
	ScrapedAt time.Time       `json:"scraped_at"`
	Reviews   []parser.Review `json:"reviews"`
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

// ValidateScrapeRequest — публичная функция, тестируется напрямую.
func ValidateScrapeRequest(req ScrapeRequest) error {
	if strings.TrimSpace(req.Brand) == "" {
		return errors.New("brand is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return errors.New("url is required")
	}
	u, err := url.Parse(req.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("url must be an absolute URL (e.g. https://example.com)")
	}
	if !validSourceTypes[req.SourceType] {
		return fmt.Errorf("source_type must be one of: html, api, js (got: %q)", req.SourceType)
	}
	return nil
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type Handler struct {
	mux *http.ServeMux
	cfg Config
}

func New(cfg Config) *Handler {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	h := &Handler{mux: http.NewServeMux(), cfg: cfg}
	h.mux.HandleFunc("/scrape", h.handleScrape)
	h.mux.HandleFunc("/health", h.handleHealth)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ─── POST /scrape ─────────────────────────────────────────────────────────────

func (h *Handler) handleScrape(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := ValidateScrapeRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	jobID := uuid.New().String()

	// В тестовом режиме возвращаем пустой результат без вызова скрапера
	if h.cfg.Mode == ModeTest || h.cfg.Dispatcher == nil {
		writeJSON(w, http.StatusOK, ScrapeResponse{
			JobID:     jobID,
			Brand:     req.Brand,
			Source:    req.URL,
			URL:       req.URL,
			ScrapedAt: time.Now().UTC(),
			Reviews:   []parser.Review{},
		})
		return
	}

	// Синхронный вызов скрапера + парсер
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.Timeout)
	defer cancel()

	result, err := h.cfg.Dispatcher.Scrape(scraper.ScrapeRequest{
		URL:        req.URL,
		SourceType: scraper.SourceType(req.SourceType),
		Brand:      req.Brand,
	})
	if err != nil {
		// Контекст истёк
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "scraping timed out")
			return
		}
		writeError(w, http.StatusBadGateway, "scrape error: "+err.Error())
		return
	}

	reviews := parser.ParseByURL(req.URL, result.Text)

	writeJSON(w, http.StatusOK, ScrapeResponse{
		JobID:     jobID,
		Brand:     result.Brand,
		Source:    result.Source,
		URL:       result.SourceURL,
		ScrapedAt: result.ScrapedAt,
		Reviews:   reviews,
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
