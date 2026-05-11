package antiblock

import (
	"errors"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoProxies = errors.New("no available proxies")

// ─── UserAgentRotator ─────────────────────────────────────────────────────────

var defaultAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/122.0.0.0",
}

type UserAgentRotator struct {
	agents  []string
	counter atomic.Uint64
}

func NewUserAgentRotator(agents []string) *UserAgentRotator {
	if len(agents) == 0 {
		agents = defaultAgents
	}
	return &UserAgentRotator{agents: agents}
}

// Next возвращает следующий User-Agent по кругу (thread-safe).
func (r *UserAgentRotator) Next() string {
	idx := r.counter.Add(1) - 1
	return r.agents[idx%uint64(len(r.agents))]
}

// ─── ProxyRotator ─────────────────────────────────────────────────────────────

type ProxyRotator struct {
	mu      sync.RWMutex
	proxies []string
	bad     map[string]bool
	counter int
}

func NewProxyRotator(proxies []string) *ProxyRotator {
	return &ProxyRotator{
		proxies: proxies,
		bad:     make(map[string]bool),
	}
}

// Next возвращает следующий рабочий прокси.
func (r *ProxyRotator) Next() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	good := r.goodProxies()
	if len(good) == 0 {
		return "", ErrNoProxies
	}
	proxy := good[r.counter%len(good)]
	r.counter++
	return proxy, nil
}

// MarkBad помечает прокси как нерабочий — он будет пропускаться.
func (r *ProxyRotator) MarkBad(proxy string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bad[proxy] = true
}

func (r *ProxyRotator) goodProxies() []string {
	result := make([]string, 0, len(r.proxies))
	for _, p := range r.proxies {
		if !r.bad[p] {
			result = append(result, p)
		}
	}
	return result
}

// ─── RateLimiter ──────────────────────────────────────────────────────────────

type RateLimiterConfig struct {
	RequestsPerSecond float64
	MinDelay          time.Duration
	MaxDelay          time.Duration
}

type RateLimiter struct {
	cfg     RateLimiterConfig
	mu      sync.Mutex
	domains map[string]time.Time // последний запрос к домену
}

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = 1
	}
	return &RateLimiter{
		cfg:     cfg,
		domains: make(map[string]time.Time),
	}
}

// Wait блокирует до тех пор, пока не пройдёт минимальный интервал между
// запросами к одному домену. Thread-safe.
func (rl *RateLimiter) Wait(domain string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	minInterval := time.Duration(float64(time.Second) / rl.cfg.RequestsPerSecond)

	// Добавляем случайный jitter чтобы не выглядеть как бот
	jitter := time.Duration(0)
	if rl.cfg.MaxDelay > rl.cfg.MinDelay {
		jitter = rl.cfg.MinDelay + time.Duration(
			rand.Int63n(int64(rl.cfg.MaxDelay-rl.cfg.MinDelay)),
		)
	}

	interval := minInterval + jitter

	if last, ok := rl.domains[domain]; ok {
		elapsed := time.Since(last)
		if elapsed < interval {
			time.Sleep(interval - elapsed)
		}
	}

	rl.domains[domain] = time.Now()
}

// ─── DomainExtractor ─────────────────────────────────────────────────────────

// ExtractDomain извлекает корневой домен из URL (убирает www. и субдомены).
func ExtractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}

	host := u.Hostname() // без порта
	host = strings.TrimPrefix(host, "www.")

	// Оставляем только последние два уровня: sub.example.com → example.com
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, ".")
}
