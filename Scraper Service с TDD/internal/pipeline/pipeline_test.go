// Package pipeline содержит сквозной интеграционный тест цепочки:
// POST /scrape → stub-скрапер → parser → JSON с отзывами.
package pipeline_test

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
	"diplom/scraper-service/internal/parser"
	"diplom/scraper-service/internal/scraper"
)

// ─── Stub scraper ─────────────────────────────────────────────────────────────

type stubScraper struct {
	result *scraper.ScrapeResult
	err    error
}

func (s *stubScraper) Scrape(req scraper.ScrapeRequest) (*scraper.ScrapeResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	r := *s.result
	r.Brand = req.Brand
	r.SourceType = req.SourceType
	return &r, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func doPost(h *handler.Handler, req handler.ScrapeRequest) *httptest.ResponseRecorder {
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	return w
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestPipeline_FullChain — 3 reviews in raw text, verify all 3 returned with correct fields.
func TestPipeline_FullChain(t *testing.T) {
	rawText := `
[RATING:5][DATE:20.12.2025]
Отличный банк с удобным мобильным приложением
Долго пользуюсь сервисами банка и очень доволен качеством обслуживания

[RATING:2][DATE:15.01.2026]
Заблокировали карту без предупреждения и объяснений
Очень неприятная ситуация, пришлось долго ждать разблокировки через поддержку

[RATING:4][DATE:10.02.2026]
Хорошее отделение рядом с домом и вежливый персонал
`

	stub := &stubScraper{
		result: &scraper.ScrapeResult{
			SourceURL: "https://irecommend.ru/content/sberbank",
			Source:    "irecommend.ru/content/sberbank",
			Title:     "Сбербанк отзывы",
			Text:      rawText,
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := doPost(h, handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://irecommend.ru/content/sberbank",
		SourceType: "js",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "сбер", resp.Brand)
	assert.Equal(t, "irecommend.ru/content/sberbank", resp.Source)
	assert.NotEmpty(t, resp.ScrapedAt)

	require.Len(t, resp.Reviews, 3)

	assert.Equal(t, "5", resp.Reviews[0].Rating)
	assert.Equal(t, "20.12.2025", resp.Reviews[0].Date)
	assert.NotEmpty(t, resp.Reviews[0].Title)
	assert.NotEmpty(t, resp.Reviews[0].Text)

	assert.Equal(t, "2", resp.Reviews[1].Rating)
	assert.Equal(t, "15.01.2026", resp.Reviews[1].Date)
	assert.NotEmpty(t, resp.Reviews[1].Title)

	assert.Equal(t, "4", resp.Reviews[2].Rating)
	assert.Equal(t, "10.02.2026", resp.Reviews[2].Date)
	assert.NotEmpty(t, resp.Reviews[2].Title)
}

// TestPipeline_BankiruRouting — URL is banki.ru, title is before marker.
func TestPipeline_BankiruRouting(t *testing.T) {
	rawText := `
Хорошее обслуживание клиентов в отделении
[RATING:4][DATE:01.03.2026]
Быстро оформили карту и объяснили все условия обслуживания

Проблема с переводами за границу
[RATING:2][DATE:15.03.2026]
Несколько раз возникали задержки при международных переводах без объяснений
`

	stub := &stubScraper{
		result: &scraper.ScrapeResult{
			SourceURL: "https://banki.ru/services/banks/bank/sberbank/",
			Source:    "banki.ru/services/banks/bank/sberbank",
			Text:      rawText,
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := doPost(h, handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://banki.ru/services/banks/bank/sberbank/",
		SourceType: "js",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Reviews, 2)

	// bankiru parser: title is the line BEFORE the marker
	assert.Equal(t, "Хорошее обслуживание клиентов в отделении", resp.Reviews[0].Title)
	assert.Equal(t, "4", resp.Reviews[0].Rating)
	assert.Equal(t, "01.03.2026", resp.Reviews[0].Date)

	assert.Equal(t, "Проблема с переводами за границу", resp.Reviews[1].Title)
	assert.Equal(t, "2", resp.Reviews[1].Rating)
}

// TestPipeline_OtzovikRouting — URL is otzovik.com, pros field populated.
func TestPipeline_OtzovikRouting(t *testing.T) {
	rawText := `
[RATING:5][DATE:2025-12-01]
Надёжный банк с хорошей репутацией
Достоинства: Быстрое обслуживание и вежливый персонал
Недостатки: Высокая комиссия за снятие наличных в банкоматах

[RATING:3][DATE:2026-01-20]
Обычный банк ничего особенного
`

	stub := &stubScraper{
		result: &scraper.ScrapeResult{
			SourceURL: "https://otzovik.com/reviews/sberbank/",
			Source:    "otzovik.com/reviews/sberbank",
			Text:      rawText,
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := doPost(h, handler.ScrapeRequest{
		Brand:      "сбер",
		URL:        "https://otzovik.com/reviews/sberbank/",
		SourceType: "js",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Reviews, 2)

	assert.Equal(t, "Надёжный банк с хорошей репутацией", resp.Reviews[0].Title)
	assert.Equal(t, "5", resp.Reviews[0].Rating)
	// otzovik parser: pros and cons extracted
	assert.Equal(t, "Быстрое обслуживание и вежливый персонал", resp.Reviews[0].Pros)
	assert.Equal(t, "Высокая комиссия за снятие наличных в банкоматах", resp.Reviews[0].Cons)

	assert.Equal(t, "3", resp.Reviews[1].Rating)
}

// TestPipeline_ScraperError — dispatcher error → 502.
func TestPipeline_ScraperError(t *testing.T) {
	stub := &stubScraper{err: assert.AnError}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := doPost(h, handler.ScrapeRequest{
		Brand:      "тест",
		URL:        "https://example.com",
		SourceType: "html",
	})

	assert.Equal(t, http.StatusBadGateway, w.Code)

	var resp handler.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Error)
}

// TestPipeline_EmptyText — text with no markers → 200, reviews:[].
func TestPipeline_EmptyText(t *testing.T) {
	stub := &stubScraper{
		result: &scraper.ScrapeResult{
			SourceURL: "https://example.com",
			Source:    "example.com",
			Text:      "просто текст без маркеров рейтинга",
			ScrapedAt: time.Now().UTC(),
		},
	}

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: stub,
		Timeout:    5 * time.Second,
	})

	w := doPost(h, handler.ScrapeRequest{
		Brand:      "тест",
		URL:        "https://example.com",
		SourceType: "html",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var resp handler.ScrapeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Reviews)
	assert.Empty(t, resp.Reviews)
}

// ─── Ensure parser.Review struct fields are all exported ─────────────────────

func TestReviewFields(t *testing.T) {
	r := parser.Review{
		Title:  "Тест",
		Text:   "Текст",
		Rating: "5",
		Date:   "01.01.2026",
		Pros:   "Плюсы",
		Cons:   "Минусы",
	}
	assert.Equal(t, "Тест", r.Title)
	assert.Equal(t, "Текст", r.Text)
	assert.Equal(t, "5", r.Rating)
	assert.Equal(t, "01.01.2026", r.Date)
	assert.Equal(t, "Плюсы", r.Pros)
	assert.Equal(t, "Минусы", r.Cons)
}
