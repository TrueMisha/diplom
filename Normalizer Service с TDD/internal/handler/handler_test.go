package handler_test

// TDD: тесты HTTP-слоя написаны ДО реализации handler.
// Используем httptest — никаких реальных портов.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandmon/normalizer-service/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── POST /normalize ─────────────────────────────────────────────────────────

func TestNormalizeHandler_Success(t *testing.T) {
	req := handler.NormalizeRequest{
		Text: "Сбербанк — хороший и надёжный банк!",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/normalize", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.NormalizeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.Words)
	assert.NotEmpty(t, resp.Frequencies)

	wordSet := toSet(resp.Words)
	assert.Contains(t, wordSet, "сбербанк")
	assert.Contains(t, wordSet, "надёжный")
	assert.NotContains(t, wordSet, "и") // стоп-слово
}

func TestNormalizeHandler_EmptyText(t *testing.T) {
	req := handler.NormalizeRequest{Text: ""}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/normalize", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.NormalizeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Words)
}

func TestNormalizeHandler_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/normalize", bytes.NewReader([]byte("not json")))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNormalizeHandler_WrongMethod(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/normalize", nil)

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// ─── POST /cooccurrence ───────────────────────────────────────────────────────

func TestCooccurrenceHandler_Success(t *testing.T) {
	req := handler.CooccurrenceRequest{
		Text:   "сбер хороший банк сбер надёжный",
		Target: "сбер",
		Window: 2,
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/cooccurrence", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.CooccurrenceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.Neighbors)
	assert.Contains(t, resp.Neighbors, "банк")
}

func TestCooccurrenceHandler_MissingTarget(t *testing.T) {
	req := handler.CooccurrenceRequest{
		Text:   "сбер банк хороший",
		Target: "", // пустой target — ошибка
		Window: 2,
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/cooccurrence", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── GET /health ─────────────────────────────────────────────────────────────

func TestHealthHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)

	h := handler.New()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toSet(words []string) map[string]bool {
	s := make(map[string]bool)
	for _, w := range words {
		s[w] = true
	}
	return s
}
