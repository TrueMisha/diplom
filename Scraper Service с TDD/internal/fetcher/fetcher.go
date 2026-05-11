package fetcher

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ─── Sentinel errors ─────────────────────────────────────────────────────────

var (
	ErrNotFound    = errors.New("page not found (404)")
	ErrRateLimited = errors.New("rate limited (429)")
	ErrBlocked     = errors.New("blocked by anti-bot")
	ErrTimeout     = errors.New("request timed out")
)

// ─── Interface ────────────────────────────────────────────────────────────────

// Fetcher — контракт для получения страниц.
// Любая реализация (HTTP, mock, retry-обёртка) должна его соблюдать.
type Fetcher interface {
	Fetch(url string) (*Result, error)
}

type Result struct {
	URL        string
	Body       string
	StatusCode int
	Headers    http.Header
}

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	Timeout   time.Duration
	UserAgent string
}

// ─── HTTPFetcher ──────────────────────────────────────────────────────────────

type HTTPFetcher struct {
	client    *http.Client
	userAgent string
}

func NewHTTPFetcher(cfg Config) *HTTPFetcher {
	ua := cfg.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (compatible; BrandMonBot/1.0)"
	}
	return &HTTPFetcher{
		client:    &http.Client{Timeout: cfg.Timeout},
		userAgent: ua,
	}
}

func (f *HTTPFetcher) Fetch(url string) (*Result, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		// Оборачиваем таймаут в наш sentinel
		if errors.Is(err, http.ErrHandlerTimeout) || isTimeoutError(err) {
			return nil, ErrTimeout
		}
		// Все остальные сетевые ошибки (включая таймаут контекста)
		if isTimeoutError(err) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case http.StatusForbidden:
		return nil, ErrBlocked
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	body := string(bodyBytes)

	// Эвристика: страница с CAPTCHA / anti-bot
	if isAntiBot(body) {
		return nil, ErrBlocked
	}

	return &Result{
		URL:        url,
		Body:       body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
	}, nil
}

// isAntiBot проверяет характерные признаки страниц блокировки.
func isAntiBot(body string) bool {
	lower := strings.ToLower(body)
	patterns := []string{
		"access denied",
		"please verify you are human",
		"captcha",
		"cloudflare",
		"you have been blocked",
		"robot check",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "context deadline")
}

// ─── RetryFetcher ─────────────────────────────────────────────────────────────

type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

type RetryFetcher struct {
	base Fetcher
	cfg  RetryConfig
}

func NewRetryFetcher(base Fetcher, cfg RetryConfig) *RetryFetcher {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.Delay <= 0 {
		cfg.Delay = 1 * time.Second
	}
	return &RetryFetcher{base: base, cfg: cfg}
}

func (f *RetryFetcher) Fetch(url string) (*Result, error) {
	var lastErr error

	for attempt := 1; attempt <= f.cfg.MaxAttempts; attempt++ {
		result, err := f.base.Fetch(url)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Ретраим только временные ошибки
		if !isRetryable(err) {
			return nil, err
		}

		if attempt < f.cfg.MaxAttempts {
			time.Sleep(f.cfg.Delay)
		}
	}

	return nil, lastErr
}

// isRetryable — только rate limit и таймаут стоит ретраить.
func isRetryable(err error) bool {
	return errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTimeout)
}
