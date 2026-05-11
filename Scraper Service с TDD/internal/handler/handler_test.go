package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"diplom/scraper-service/internal/handler"
	"diplom/scraper-service/internal/scraper"
)

// ─── Stub dispatcher ──────────────────────────────────────────────────────────

type stubDispatcher struct {
	result *scraper.ScrapeResult
	err    error
}

func (s *stubDispatcher) Scrape(req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	r := *s.result
	r.Brand = req.Brand
	r.SourceType = req.SourceType
	return &r, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func testHandler() *handler.Handler {
	return handler.New(handler.Config{Mode: handler.ModeTest})
}

func postScrape(h *handler.Handler, req handler.ScrapeRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	return w
}

// ─── POST /scrape — ModeTest ──────────────────────────────────────────────────

func TestScrapeHandler_ValidRequest(t *testing.T) {
	w := postScrape(testHandler(), handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://example.com",
		SourceType: "html",
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "сбер", resp.Brand)
	assert.NotNil(t, resp.Reviews) // empty slice, not nil
}

// ─── POST /scrape — with real dispatcher (stub) ───────────────────────────────

func TestScrapeHandler_WithDispatcher_ReturnsReviews(t *testing.T) {
	rawText := `
[RATING:5][DATE:20.12.2025]
Отличный банк с хорошим обслуживанием
Долго пользуюсь и очень доволен качеством работы всех сервисов банка

[RATING:2][DATE:15.01.2026]
Ужасный опыт посещения отделения
`
	stub := &stubDispatcher{
		result: &scraper.ScrapeResult{
			SourceURL: "https://irecommend.ru/content/sberbank",
			Source:    "irecommend.ru/content/sberbank",
			Text:      rawText,
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := postScrape(h, handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://irecommend.ru/content/sberbank",
		SourceType: "js",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "сбер", resp.Brand)
	require.Len(t, resp.Reviews, 2)
	assert.Equal(t, "5", resp.Reviews[0].Rating)
	assert.Equal(t, "20.12.2025", resp.Reviews[0].Date)
	assert.Equal(t, "2", resp.Reviews[1].Rating)
}

func TestScrapeHandler_WithDispatcher_ParsesBankiru(t *testing.T) {
	rawText := `
Хорошее обслуживание клиентов в отделении
[RATING:4][DATE:01.03.2026]
Быстро оформили карту и объяснили все условия обслуживания
`
	stub := &stubDispatcher{
		result: &scraper.ScrapeResult{
			SourceURL: "https://banki.ru/services/banks/sberbank",
			Source:    "banki.ru/services/banks/sberbank",
			Text:      rawText,
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := postScrape(h, handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://banki.ru/services/banks/sberbank",
		SourceType: "js",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Reviews, 1)
	assert.Equal(t, "Хорошее обслуживание клиентов в отделении", resp.Reviews[0].Title)
	assert.Equal(t, "4", resp.Reviews[0].Rating)
}

func TestScrapeHandler_DispatcherError(t *testing.T) {
	stub := &stubDispatcher{err: assert.AnError}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := postScrape(h, handler.ScrapeRequest{
		Brand:      "тест",
		URL:        "https://example.com",
		SourceType: "html",
	})

	assert.Equal(t, http.StatusBadGateway, w.Code)

	var resp handler.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Error)
}

// ─── Validation errors ────────────────────────────────────────────────────────

func TestScrapeHandler_MissingBrand(t *testing.T) {
	w := postScrape(testHandler(), handler.ScrapeRequest{
		URL: "https://example.com", SourceType: "html",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp handler.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error, "brand")
}

func TestScrapeHandler_MissingURL(t *testing.T) {
	w := postScrape(testHandler(), handler.ScrapeRequest{
		Brand: "сбер", SourceType: "html",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScrapeHandler_InvalidSourceType(t *testing.T) {
	w := postScrape(testHandler(), handler.ScrapeRequest{
		Brand: "сбер", URL: "https://example.com", SourceType: "ftp",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp handler.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error, "source_type")
}

func TestScrapeHandler_InvalidURL(t *testing.T) {
	w := postScrape(testHandler(), handler.ScrapeRequest{
		Brand: "сбер", URL: "not-a-url", SourceType: "html",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── Method / JSON errors ─────────────────────────────────────────────────────

func TestScrapeHandler_WrongMethod(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/scrape", nil)
	testHandler().ServeHTTP(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScrapeHandler_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewReader([]byte("{")))
	r.Header.Set("Content-Type", "application/json")
	testHandler().ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── GET /health ──────────────────────────────────────────────────────────────

func TestHealthHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	testHandler().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

// ─── ValidateScrapeRequest — table-driven ────────────────────────────────────

func TestValidateScrapeRequest(t *testing.T) {
	cases := []struct {
		name    string
		req     handler.ScrapeRequest
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid html",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "https://example.com", SourceType: "html"},
			wantErr: false,
		},
		{
			name:    "valid api",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "https://example.com", SourceType: "api"},
			wantErr: false,
		},
		{
			name:    "valid js",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "https://example.com", SourceType: "js"},
			wantErr: false,
		},
		{
			name:    "empty brand",
			req:     handler.ScrapeRequest{URL: "https://example.com", SourceType: "html"},
			wantErr: true, errMsg: "brand",
		},
		{
			name:    "whitespace brand",
			req:     handler.ScrapeRequest{Brand: "   ", URL: "https://example.com", SourceType: "html"},
			wantErr: true, errMsg: "brand",
		},
		{
			name:    "empty url",
			req:     handler.ScrapeRequest{Brand: "сбер", SourceType: "html"},
			wantErr: true, errMsg: "url",
		},
		{
			name:    "invalid source type",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "https://example.com", SourceType: "ftp"},
			wantErr: true, errMsg: "source_type",
		},
		{
			name:    "relative url",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "/relative/path", SourceType: "html"},
			wantErr: true,
		},
		{
			name:    "url without scheme",
			req:     handler.ScrapeRequest{Brand: "сбер", URL: "example.com/page", SourceType: "html"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler.ValidateScrapeRequest(tc.req)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
