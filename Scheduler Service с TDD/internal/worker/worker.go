// Package worker содержит обработчик задач asynq.
// Он получает задачу из очереди, вызывает POST /scrape на Scraper Service,
// затем сохраняет отзывы и слова в Storage Service.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynq"

	"diplom/scheduler-service/internal/tasks"
)

// ─── Interfaces ───────────────────────────────────────────────────────────────

// HTTPDoer позволяет подменять http.Client в тестах.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ─── Response types от Scraper Service ───────────────────────────────────────

type scraperReview struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Rating string `json:"rating,omitempty"`
	Date   string `json:"date,omitempty"`
	Pros   string `json:"pros,omitempty"`
	Cons   string `json:"cons,omitempty"`
}

type scraperResponse struct {
	JobID     string          `json:"job_id"`
	Brand     string          `json:"brand"`
	Source    string          `json:"source"`
	URL       string          `json:"url"`
	ScrapedAt time.Time       `json:"scraped_at"`
	Reviews   []scraperReview `json:"reviews"`
}

// ─── ScrapeWorker ─────────────────────────────────────────────────────────────

// ScrapeWorker обрабатывает задачи TypeScrape.
type ScrapeWorker struct {
	scraperBaseURL string
	storageBaseURL string
	client         HTTPDoer
}

// New создаёт ScrapeWorker.
// scraperBaseURL — адрес Scraper Service, например "http://localhost:8082".
// storageBaseURL — адрес Storage Service, например "http://localhost:8083".
func New(scraperBaseURL, storageBaseURL string, client HTTPDoer) *ScrapeWorker {
	if client == nil {
		client = http.DefaultClient
	}
	return &ScrapeWorker{
		scraperBaseURL: scraperBaseURL,
		storageBaseURL: storageBaseURL,
		client:         client,
	}
}

// ProcessTask — точка входа для asynq. Регистрируется через mux.HandleFunc.
func (w *ScrapeWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	payload, err := tasks.ParseScrapePayload(t)
	if err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	log.Printf("[worker] processing brand=%s url=%s type=%s",
		payload.Brand, payload.URL, payload.SourceType)

	result, err := w.callScraper(ctx, payload)
	if err != nil {
		return fmt.Errorf("call scraper: %w", err)
	}

	log.Printf("[worker] scraped brand=%s reviews=%d", result.Brand, len(result.Reviews))

	if len(result.Reviews) > 0 && w.storageBaseURL != "" {
		if err := w.saveReviews(ctx, result); err != nil {
			// Не фатально — логируем и продолжаем
			log.Printf("[worker] save reviews failed: %v", err)
		} else {
			log.Printf("[worker] saved %d reviews to storage", len(result.Reviews))
		}
	}

	log.Printf("[worker] done brand=%s url=%s", payload.Brand, payload.URL)
	return nil
}

// callScraper отправляет POST /scrape на Scraper Service и возвращает распарсенный ответ.
func (w *ScrapeWorker) callScraper(ctx context.Context, p tasks.ScrapePayload) (*scraperResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"brand":       p.Brand,
		"url":         p.URL,
		"source_type": p.SourceType,
	})

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		w.scraperBaseURL+"/scrape",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scraper returned %d: %s", resp.StatusCode, respBody)
	}

	var result scraperResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode scraper response: %w", err)
	}
	return &result, nil
}

// saveReviews отправляет POST /reviews на Storage Service.
func (w *ScrapeWorker) saveReviews(ctx context.Context, result *scraperResponse) error {
	type reviewEntry struct {
		Title      string `json:"title"`
		Text       string `json:"text"`
		Rating     string `json:"rating,omitempty"`
		ReviewDate string `json:"review_date,omitempty"`
		Pros       string `json:"pros,omitempty"`
		Cons       string `json:"cons,omitempty"`
	}
	type saveReviewsReq struct {
		Brand     string        `json:"brand"`
		SourceURL string        `json:"source_url"`
		JobID     string        `json:"job_id,omitempty"`
		Reviews   []reviewEntry `json:"reviews"`
	}

	entries := make([]reviewEntry, len(result.Reviews))
	for i, r := range result.Reviews {
		entries[i] = reviewEntry{
			Title:      r.Title,
			Text:       r.Text,
			Rating:     r.Rating,
			ReviewDate: r.Date,
			Pros:       r.Pros,
			Cons:       r.Cons,
		}
	}

	payload := saveReviewsReq{
		Brand:     result.Brand,
		SourceURL: result.URL,
		JobID:     result.JobID,
		Reviews:   entries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal reviews: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		w.storageBaseURL+"/reviews",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build storage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("do storage request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("storage returned %d: %s", resp.StatusCode, b)
	}
	return nil
}
