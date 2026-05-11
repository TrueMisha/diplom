package antiblock_test

// TDD: тесты защиты от блокировок написаны ДО реализации.
// Ротация UA и прокси — детерминированная логика, легко тестируется.

import (
	"testing"

	"diplom/scraper-service/internal/antiblock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── UserAgentRotator ─────────────────────────────────────────────────────────

func TestUserAgentRotator_ReturnsAgents(t *testing.T) {
	agents := []string{"Bot/1.0", "Bot/2.0", "Bot/3.0"}
	r := antiblock.NewUserAgentRotator(agents)

	ua := r.Next()
	assert.NotEmpty(t, ua)
	assert.Contains(t, agents, ua)
}

func TestUserAgentRotator_RotatesSequentially(t *testing.T) {
	agents := []string{"A", "B", "C"}
	r := antiblock.NewUserAgentRotator(agents)

	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		seen[r.Next()]++
	}

	// За 9 вызовов каждый из 3 агентов должен появиться ровно 3 раза
	for _, agent := range agents {
		assert.Equal(t, 3, seen[agent], "agent %s should appear 3 times", agent)
	}
}

func TestUserAgentRotator_EmptyListUsesDefault(t *testing.T) {
	r := antiblock.NewUserAgentRotator([]string{})
	ua := r.Next()
	assert.NotEmpty(t, ua) // должен вернуть дефолтный UA
}

func TestUserAgentRotator_SingleAgent(t *testing.T) {
	r := antiblock.NewUserAgentRotator([]string{"OnlyBot/1.0"})
	assert.Equal(t, "OnlyBot/1.0", r.Next())
	assert.Equal(t, "OnlyBot/1.0", r.Next()) // всегда одно и то же
}

// ─── ProxyRotator ─────────────────────────────────────────────────────────────

func TestProxyRotator_ReturnsProxy(t *testing.T) {
	proxies := []string{
		"http://proxy1:8080",
		"http://proxy2:8080",
		"http://proxy3:8080",
	}
	r := antiblock.NewProxyRotator(proxies)

	proxy, err := r.Next()
	require.NoError(t, err)
	assert.Contains(t, proxies, proxy)
}

func TestProxyRotator_RotatesAllProxies(t *testing.T) {
	proxies := []string{"http://p1:80", "http://p2:80"}
	r := antiblock.NewProxyRotator(proxies)

	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		p, err := r.Next()
		require.NoError(t, err)
		seen[p] = true
	}

	assert.True(t, seen["http://p1:80"])
	assert.True(t, seen["http://p2:80"])
}

func TestProxyRotator_NoProxiesReturnsError(t *testing.T) {
	r := antiblock.NewProxyRotator([]string{})

	_, err := r.Next()
	require.Error(t, err)
	assert.ErrorIs(t, err, antiblock.ErrNoProxies)
}

func TestProxyRotator_MarkBad_SkipsBadProxy(t *testing.T) {
	proxies := []string{"http://good:80", "http://bad:80"}
	r := antiblock.NewProxyRotator(proxies)

	r.MarkBad("http://bad:80")

	// За 10 итераций плохой прокси не должен попасться
	for i := 0; i < 10; i++ {
		p, err := r.Next()
		require.NoError(t, err)
		assert.NotEqual(t, "http://bad:80", p)
	}
}

func TestProxyRotator_MarkBad_AllBadReturnsError(t *testing.T) {
	proxies := []string{"http://p1:80"}
	r := antiblock.NewProxyRotator(proxies)
	r.MarkBad("http://p1:80")

	_, err := r.Next()
	require.Error(t, err)
	assert.ErrorIs(t, err, antiblock.ErrNoProxies)
}

// ─── RateLimiter ──────────────────────────────────────────────────────────────

func TestRateLimiter_AllowsRequests(t *testing.T) {
	// 10 запросов в секунду — в тестах просто проверяем что Wait не блокирует надолго
	rl := antiblock.NewRateLimiter(antiblock.RateLimiterConfig{
		RequestsPerSecond: 100, // высокий лимит, не блокирует
		MinDelay:          0,
		MaxDelay:          0,
	})

	// Не должен паниковать или блокироваться
	assert.NotPanics(t, func() {
		rl.Wait("example.com")
	})
}

func TestRateLimiter_SeparateLimitsPerDomain(t *testing.T) {
	rl := antiblock.NewRateLimiter(antiblock.RateLimiterConfig{
		RequestsPerSecond: 100,
	})

	// Разные домены — независимые счётчики
	assert.NotPanics(t, func() {
		rl.Wait("site-a.com")
		rl.Wait("site-b.com")
		rl.Wait("site-a.com")
	})
}

// ─── DomainExtractor ─────────────────────────────────────────────────────────

func TestExtractDomain(t *testing.T) {
	cases := []struct {
		url      string
		expected string
	}{
		{"https://www.banki.ru/products/reviews/", "banki.ru"},
		{"http://sber.ru", "sber.ru"},
		{"https://sub.example.com/path?q=1", "example.com"},
		{"not-a-url", ""},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			result := antiblock.ExtractDomain(tc.url)
			assert.Equal(t, tc.expected, result)
		})
	}
}
