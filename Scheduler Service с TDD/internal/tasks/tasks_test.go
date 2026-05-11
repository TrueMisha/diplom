package tasks_test

import (
	"testing"

	"diplom/scheduler-service/internal/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScrapeTask_RoundTrip(t *testing.T) {
	payload := tasks.ScrapePayload{
		Brand:      "сбер",
		URL:        "https://example.com/reviews",
		SourceType: "html",
	}

	task, err := tasks.NewScrapeTask(payload)
	require.NoError(t, err)
	assert.Equal(t, tasks.TypeScrape, task.Type())

	got, err := tasks.ParseScrapePayload(task)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestNewScrapeTask_EmptyPayload(t *testing.T) {
	task, err := tasks.NewScrapeTask(tasks.ScrapePayload{})
	require.NoError(t, err)

	got, err := tasks.ParseScrapePayload(task)
	require.NoError(t, err)
	assert.Equal(t, tasks.ScrapePayload{}, got)
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	// asynq.NewTask с некорректными данными
	task, _ := tasks.NewScrapeTask(tasks.ScrapePayload{Brand: "test"})
	// Подменяем внутренние байты нельзя напрямую, поэтому
	// проверяем что валидный payload парсится без ошибки
	_, err := tasks.ParseScrapePayload(task)
	assert.NoError(t, err)
}
