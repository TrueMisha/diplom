// Package tasks определяет типы задач и их полезную нагрузку для asynq.
package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// TypeScrape — имя задачи скрапинга.
const TypeScrape = "scrape:run"

// ScrapePayload — данные задачи.
type ScrapePayload struct {
	Brand      string `json:"brand"`
	URL        string `json:"url"`
	SourceType string `json:"source_type"`
}

// NewScrapeTask создаёт asynq.Task с заданной полезной нагрузкой.
func NewScrapeTask(p ScrapePayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return asynq.NewTask(TypeScrape, data), nil
}

// ParseScrapePayload десериализует полезную нагрузку из задачи.
func ParseScrapePayload(t *asynq.Task) (ScrapePayload, error) {
	var p ScrapePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return p, fmt.Errorf("unmarshal payload: %w", err)
	}
	return p, nil
}
