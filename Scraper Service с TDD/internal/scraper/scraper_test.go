package scraper_test

// TDD: тесты парсера HTML написаны ДО реализации.
// MockFetcher позволяет тестировать парсинг без реального HTTP.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"diplom/scraper-service/internal/fetcher"
	"diplom/scraper-service/internal/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── MockFetcher — тестовый двойник ──────────────────────────────────────────

type MockFetcher struct {
	result *fetcher.Result
	err    error
}

func (m *MockFetcher) Fetch(_ string) (*fetcher.Result, error) {
	return m.result, m.err
}

func mockOK(body string) *MockFetcher {
	return &MockFetcher{
		result: &fetcher.Result{Body: body, StatusCode: http.StatusOK},
	}
}

func mockErr(err error) *MockFetcher {
	return &MockFetcher{err: err}
}

// ─── HTMLScraper ──────────────────────────────────────────────────────────────

func TestHTMLScraper_ExtractsTextFromParagraphs(t *testing.T) {
	html := `<html><body>
		<p>Сбербанк — отличный банк</p>
		<p>Надёжный и удобный сервис</p>
	</body></html>`

	s := scraper.NewHTMLScraper(mockOK(html))
	result, err := s.Scrape("https://example.com", scraper.Options{})

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Сбербанк")
	assert.Contains(t, result.Text, "Надёжный")
	assert.Equal(t, "https://example.com", result.SourceURL)
}

func TestHTMLScraper_ExtractsTextFromReviewSelectors(t *testing.T) {
	html := `<html><body>
		<div class="review">Отличный банк, рекомендую!</div>
		<span class="comment">Быстрое обслуживание</span>
		<div class="review-text">Хорошие условия по кредиту</div>
	</body></html>`

	s := scraper.NewHTMLScraper(mockOK(html))
	result, err := s.Scrape("https://example.com", scraper.Options{})

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Отличный банк")
	assert.Contains(t, result.Text, "Быстрое обслуживание")
}

func TestHTMLScraper_ExcludesNavAndFooter(t *testing.T) {
	html := `<html><body>
		<nav>Главная | О нас | Контакты</nav>
		<p>Полезный контент</p>
		<footer>Copyright 2024</footer>
	</body></html>`

	s := scraper.NewHTMLScraper(mockOK(html))
	result, err := s.Scrape("https://example.com", scraper.Options{})

	require.NoError(t, err)
	assert.NotContains(t, result.Text, "Главная | О нас")
	assert.NotContains(t, result.Text, "Copyright 2024")
	assert.Contains(t, result.Text, "Полезный контент")
}

func TestHTMLScraper_EmptyPage(t *testing.T) {
	s := scraper.NewHTMLScraper(mockOK("<html><body></body></html>"))
	result, err := s.Scrape("https://example.com", scraper.Options{})

	require.NoError(t, err)
	assert.Empty(t, result.Text)
}

func TestHTMLScraper_PropagatesFetchError(t *testing.T) {
	s := scraper.NewHTMLScraper(mockErr(fetcher.ErrBlocked))
	_, err := s.Scrape("https://example.com", scraper.Options{})

	require.Error(t, err)
	assert.ErrorIs(t, err, fetcher.ErrBlocked)
}

func TestHTMLScraper_WithCustomSelectors(t *testing.T) {
	html := `<html><body>
		<article>Основная статья о банке</article>
		<aside>Реклама сбоку</aside>
	</body></html>`

	s := scraper.NewHTMLScraper(mockOK(html))
	result, err := s.Scrape("https://example.com", scraper.Options{
		Selectors: []string{"article"},
	})

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Основная статья")
	assert.NotContains(t, result.Text, "Реклама")
}

func TestHTMLScraper_MetadataPopulated(t *testing.T) {
	html := `<html>
		<head><title>Отзывы о Сбербанке</title></head>
		<body><p>Текст отзыва</p></body>
	</html>`

	s := scraper.NewHTMLScraper(mockOK(html))
	result, err := s.Scrape("https://banki.ru/sber", scraper.Options{})

	require.NoError(t, err)
	assert.Equal(t, "Отзывы о Сбербанке", result.Title)
	assert.Equal(t, "banki.ru/sber", result.Source)
	assert.False(t, result.ScrapedAt.IsZero())
}

// ─── Интеграционный тест с реальным httptest.Server ──────────────────────────

func TestHTMLScraper_Integration_RealHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>
			<h1>Сбербанк Онлайн</h1>
			<p class="review">Очень удобное приложение</p>
			<p class="review">Хорошая поддержка клиентов</p>
		</body></html>`))
	}))
	defer srv.Close()

	// Используем реальный fetcher с реальным HTTP-сервером
	f := fetcher.NewHTTPFetcher(fetcher.Config{})
	s := scraper.NewHTMLScraper(f)

	result, err := s.Scrape(srv.URL, scraper.Options{})

	require.NoError(t, err)
	assert.Contains(t, result.Text, "Сбербанк Онлайн")
	assert.Contains(t, result.Text, "Очень удобное приложение")
}

// ─── ScrapeResult validation ──────────────────────────────────────────────────

func TestScrapeResult_IsValid(t *testing.T) {
	cases := []struct {
		name    string
		result  scraper.ScrapeResult
		wantErr bool
	}{
		{
			name:    "valid result",
			result:  scraper.ScrapeResult{SourceURL: "https://example.com", Text: "some text"},
			wantErr: false,
		},
		{
			name:    "missing url",
			result:  scraper.ScrapeResult{Text: "some text"},
			wantErr: true,
		},
		{
			name:    "empty text",
			result:  scraper.ScrapeResult{SourceURL: "https://example.com", Text: ""},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.result.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ─── SourceType routing ───────────────────────────────────────────────────────

func TestScrapeDispatcher_RoutesToCorrectScraper(t *testing.T) {
	html := `<html><body><p>тест</p></body></html>`
	f := mockOK(html)

	d := scraper.NewDispatcher(scraper.DispatcherConfig{
		HTMLFetcher: f,
	})

	// html source type
	result, err := d.Scrape(scraper.ScrapeRequest{
		URL:        "https://example.com",
		SourceType: scraper.SourceTypeHTML,
		Brand:      "сбер",
	})

	require.NoError(t, err)
	assert.Equal(t, "сбер", result.Brand)
	assert.Equal(t, scraper.SourceTypeHTML, result.SourceType)
}

func TestScrapeDispatcher_UnknownSourceTypeReturnsError(t *testing.T) {
	d := scraper.NewDispatcher(scraper.DispatcherConfig{})

	_, err := d.Scrape(scraper.ScrapeRequest{
		URL:        "https://example.com",
		SourceType: "unknown",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, scraper.ErrUnknownSourceType)
}

// Проверяем что ошибки fetcher правильно пробрасываются через dispatcher
func TestScrapeDispatcher_PropagatesFetcherError(t *testing.T) {
	d := scraper.NewDispatcher(scraper.DispatcherConfig{
		HTMLFetcher: mockErr(fetcher.ErrRateLimited),
	})

	_, err := d.Scrape(scraper.ScrapeRequest{
		URL:        "https://example.com",
		SourceType: scraper.SourceTypeHTML,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, fetcher.ErrRateLimited))
}
