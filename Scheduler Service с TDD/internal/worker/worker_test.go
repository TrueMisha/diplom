package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"diplom/scheduler-service/internal/tasks"
	"diplom/scheduler-service/internal/worker"
)

// ─── Stub HTTP client ─────────────────────────────────────────────────────────

type stubClient struct {
	mu       sync.Mutex
	calls    []stubCall // все вызовы по порядку
	scraperStatus  int
	storageStatus  int
	scraperBody    string
}

type stubCall struct {
	url  string
	body []byte
}

func (c *stubClient) Do(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	c.mu.Lock()
	c.calls = append(c.calls, stubCall{url: req.URL.String(), body: body})
	c.mu.Unlock()

	if strings.Contains(req.URL.String(), "/scrape") {
		return &http.Response{
			StatusCode: c.scraperStatus,
			Body:       io.NopCloser(strings.NewReader(c.scraperBody)),
		}, nil
	}
	// /reviews
	return &http.Response{
		StatusCode: c.storageStatus,
		Body:       io.NopCloser(strings.NewReader(`{"saved":1}`)),
	}, nil
}

const scraperOKBody = `{
	"job_id":"abc",
	"brand":"сбер",
	"source":"irecommend.ru",
	"url":"https://example.com",
	"scraped_at":"2024-01-01T00:00:00Z",
	"reviews":[{"title":"Отлично","text":"Хороший банк","rating":"5","date":"2024-01-01"}]
}`

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestScrapeWorker_ProcessTask_OK(t *testing.T) {
	stub := &stubClient{
		scraperStatus: http.StatusOK,
		storageStatus: http.StatusCreated,
		scraperBody:   scraperOKBody,
	}
	w := worker.New("http://scraper", "http://storage", stub)

	task, err := tasks.NewScrapeTask(tasks.ScrapePayload{
		Brand:      "сбер",
		URL:        "https://example.com",
		SourceType: "html",
	})
	require.NoError(t, err)

	err = w.ProcessTask(context.Background(), task)
	require.NoError(t, err)

	// Должно быть 2 вызова: /scrape и /reviews
	require.Len(t, stub.calls, 2)

	// Первый — к скраперу
	assert.Contains(t, stub.calls[0].url, "/scrape")
	var scraperReq map[string]string
	require.NoError(t, json.Unmarshal(stub.calls[0].body, &scraperReq))
	assert.Equal(t, "сбер", scraperReq["brand"])
	assert.Equal(t, "https://example.com", scraperReq["url"])
	assert.Equal(t, "html", scraperReq["source_type"])

	// Второй — к storage /reviews
	assert.Contains(t, stub.calls[1].url, "/reviews")
	var storageReq map[string]interface{}
	require.NoError(t, json.Unmarshal(stub.calls[1].body, &storageReq))
	assert.Equal(t, "сбер", storageReq["brand"])
}

func TestScrapeWorker_ProcessTask_NoReviews_SkipsStorage(t *testing.T) {
	stub := &stubClient{
		scraperStatus: http.StatusOK,
		storageStatus: http.StatusCreated,
		scraperBody: `{"job_id":"abc","brand":"тест","url":"https://example.com",
			"scraped_at":"2024-01-01T00:00:00Z","reviews":[]}`,
	}
	w := worker.New("http://scraper", "http://storage", stub)

	task, _ := tasks.NewScrapeTask(tasks.ScrapePayload{
		Brand: "тест", URL: "https://example.com", SourceType: "html",
	})

	err := w.ProcessTask(context.Background(), task)
	require.NoError(t, err)

	// Только 1 вызов — к скраперу, storage не вызывался (нет отзывов)
	assert.Len(t, stub.calls, 1)
}

func TestScrapeWorker_ProcessTask_ScraperError(t *testing.T) {
	stub := &stubClient{scraperStatus: http.StatusInternalServerError, scraperBody: `{"error":"boom"}`}
	w := worker.New("http://scraper", "http://storage", stub)

	task, _ := tasks.NewScrapeTask(tasks.ScrapePayload{
		Brand:      "test",
		URL:        "https://example.com",
		SourceType: "html",
	})

	err := w.ProcessTask(context.Background(), task)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestScrapeWorker_ProcessTask_StorageError_NotFatal(t *testing.T) {
	stub := &stubClient{
		scraperStatus: http.StatusOK,
		storageStatus: http.StatusInternalServerError,
		scraperBody:   scraperOKBody,
	}
	w := worker.New("http://scraper", "http://storage", stub)

	task, _ := tasks.NewScrapeTask(tasks.ScrapePayload{
		Brand: "сбер", URL: "https://example.com", SourceType: "html",
	})

	// Ошибка Storage не должна фейлить задачу
	err := w.ProcessTask(context.Background(), task)
	require.NoError(t, err)
}
