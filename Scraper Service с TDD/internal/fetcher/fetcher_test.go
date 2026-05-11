package fetcher_test

// TDD: определяем контракт через тесты ДО написания реализации.
// Fetcher — интерфейс для получения страниц. Это позволяет
// в тестах подменять реальный HTTP на мок-сервер.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"diplom/scraper-service/internal/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── HTTPFetcher tests ────────────────────────────────────────────────────────

func TestHTTPFetcher_FetchesPage(t *testing.T) {
	// Поднимаем локальный сервер — не зависим от интернета
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body><p>Сбербанк — лучший банк</p></body></html>"))
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{
		Timeout:   5 * time.Second,
		UserAgent: "TestBot/1.0",
	})

	result, err := f.Fetch(srv.URL)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Contains(t, result.Body, "Сбербанк")
	assert.Equal(t, srv.URL, result.URL)
}

func TestHTTPFetcher_SetsUserAgent(t *testing.T) {
	var receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{
		Timeout:   5 * time.Second,
		UserAgent: "CustomBot/2.0",
	})

	_, err := f.Fetch(srv.URL)

	require.NoError(t, err)
	assert.Equal(t, "CustomBot/2.0", receivedUA)
}

func TestHTTPFetcher_Returns404AsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})

	_, err := f.Fetch(srv.URL)

	require.Error(t, err)
	assert.ErrorIs(t, err, fetcher.ErrNotFound)
}

func TestHTTPFetcher_Returns429AsTooManyRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})

	_, err := f.Fetch(srv.URL)

	require.Error(t, err)
	assert.ErrorIs(t, err, fetcher.ErrRateLimited)
}

func TestHTTPFetcher_ReturnsBlockedOnAntiBot(t *testing.T) {
	// Сайт вернул 200 но с CAPTCHA-страницей
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Access Denied. Please verify you are human.</body></html>"))
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})

	_, err := f.Fetch(srv.URL)

	require.Error(t, err)
	assert.ErrorIs(t, err, fetcher.ErrBlocked)
}

func TestHTTPFetcher_TimeoutReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // медленный ответ
	}))
	defer srv.Close()

	f := fetcher.NewHTTPFetcher(fetcher.Config{
		Timeout: 50 * time.Millisecond, // очень короткий таймаут
	})

	_, err := f.Fetch(srv.URL)

	require.Error(t, err)
	assert.ErrorIs(t, err, fetcher.ErrTimeout)
}

func TestHTTPFetcher_InvalidURLReturnsError(t *testing.T) {
	f := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})

	_, err := f.Fetch("not-a-valid-url")

	require.Error(t, err)
}

// ─── RetryFetcher tests ───────────────────────────────────────────────────────

func TestRetryFetcher_RetriesOnRateLimit(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<p>success</p>"))
	}))
	defer srv.Close()

	base := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})
	retry := fetcher.NewRetryFetcher(base, fetcher.RetryConfig{
		MaxAttempts: 3,
		Delay:       1 * time.Millisecond, // минимальная задержка в тестах
	})

	result, err := retry.Fetch(srv.URL)

	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
	assert.Contains(t, result.Body, "success")
}

func TestRetryFetcher_StopsAfterMaxAttempts(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	base := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})
	retry := fetcher.NewRetryFetcher(base, fetcher.RetryConfig{
		MaxAttempts: 3,
		Delay:       1 * time.Millisecond,
	})

	_, err := retry.Fetch(srv.URL)

	require.Error(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetryFetcher_DoesNotRetryOn404(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	base := fetcher.NewHTTPFetcher(fetcher.Config{Timeout: 5 * time.Second})
	retry := fetcher.NewRetryFetcher(base, fetcher.RetryConfig{
		MaxAttempts: 3,
		Delay:       1 * time.Millisecond,
	})

	_, err := retry.Fetch(srv.URL)

	require.Error(t, err)
	assert.Equal(t, 1, attempts) // только 1 попытка — 404 не ретраится
}
