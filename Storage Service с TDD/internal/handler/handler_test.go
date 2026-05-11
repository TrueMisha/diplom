package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandmon/storage-service/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── POST /jobs ───────────────────────────────────────────────────────────────

func TestCreateJobHandler_Valid(t *testing.T) {
	req := handler.CreateJobRequest{
		Brand:      "сбер",
		URL:        "https://banki.ru/sber",
		SourceType: "html",
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New(handler.WithMemoryStore())
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp handler.JobResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "pending", resp.Status)
	assert.Equal(t, "сбер", resp.Brand)
}

func TestCreateJobHandler_InvalidPayload(t *testing.T) {
	cases := []struct {
		name string
		body handler.CreateJobRequest
	}{
		{"missing brand", handler.CreateJobRequest{URL: "https://x.com", SourceType: "html"}},
		{"missing url", handler.CreateJobRequest{Brand: "сбер", SourceType: "html"}},
		{"invalid source_type", handler.CreateJobRequest{Brand: "сбер", URL: "https://x.com", SourceType: "ftp"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")

			h := handler.New(handler.WithMemoryStore())
			h.ServeHTTP(w, r)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// ─── GET /jobs/:id ────────────────────────────────────────────────────────────

func TestGetJobHandler_Found(t *testing.T) {
	h := handler.New(handler.WithMemoryStore())

	// Сначала создаём job
	createBody, _ := json.Marshal(handler.CreateJobRequest{
		Brand: "сбер", URL: "https://banki.ru", SourceType: "html",
	})
	wCreate := httptest.NewRecorder()
	rCreate := httptest.NewRequest(http.MethodPost, "/jobs", bytes.NewReader(createBody))
	rCreate.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(wCreate, rCreate)

	var created handler.JobResponse
	require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))

	// Теперь получаем по ID
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/jobs/"+created.ID, nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.JobResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, created.ID, resp.ID)
}

func TestGetJobHandler_NotFound(t *testing.T) {
	h := handler.New(handler.WithMemoryStore())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/jobs/00000000-0000-0000-0000-000000000000", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── POST /words ──────────────────────────────────────────────────────────────

func TestSaveWordsHandler_Valid(t *testing.T) {
	req := handler.SaveWordsRequest{
		Brand:     "сбер",
		SourceURL: "https://banki.ru",
		Words: []handler.WordEntry{
			{Word: "банк", Frequency: 15},
			{Word: "надёжный", Frequency: 8},
		},
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/words", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New(handler.WithMemoryStore())
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestSaveWordsHandler_EmptyWords(t *testing.T) {
	req := handler.SaveWordsRequest{
		Brand: "сбер", SourceURL: "https://banki.ru",
		Words: []handler.WordEntry{},
	}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/words", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h := handler.New(handler.WithMemoryStore())
	h.ServeHTTP(w, r)

	// Пустой список — ок, просто ничего не сохраняем
	assert.Equal(t, http.StatusCreated, w.Code)
}

// ─── GET /words?brand=сбер&limit=10 ──────────────────────────────────────────

func TestGetTopWordsHandler(t *testing.T) {
	h := handler.New(handler.WithMemoryStore())

	// Сохраняем слова
	saveBody, _ := json.Marshal(handler.SaveWordsRequest{
		Brand: "сбер", SourceURL: "https://banki.ru",
		Words: []handler.WordEntry{
			{Word: "банк", Frequency: 50},
			{Word: "надёжный", Frequency: 30},
		},
	})
	wSave := httptest.NewRecorder()
	rSave := httptest.NewRequest(http.MethodPost, "/words", bytes.NewReader(saveBody))
	rSave.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(wSave, rSave)

	// Получаем топ слов
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/words?brand=сбер&limit=10", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.TopWordsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Words)
	assert.Equal(t, "банк", resp.Words[0].Word)
}

func TestGetTopWordsHandler_MissingBrand(t *testing.T) {
	h := handler.New(handler.WithMemoryStore())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/words", nil) // нет параметра brand
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── GET /health ─────────────────────────────────────────────────────────────

func TestHealthHandler(t *testing.T) {
	h := handler.New(handler.WithMemoryStore())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}
