package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"diplom/scheduler-service/internal/handler"
)

// ─── Stub enqueuer ────────────────────────────────────────────────────────────

type stubEnqueuer struct {
	enqueued []*asynq.Task
	err      error
}

func (e *stubEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.enqueued = append(e.enqueued, task)
	return &asynq.TaskInfo{ID: "stub-task-id", Type: task.Type()}, nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestScheduleHandler_ValidRequest(t *testing.T) {
	stub := &stubEnqueuer{}
	h := handler.New(stub)

	body, _ := json.Marshal(handler.ScheduleRequest{
		Brand:      "сбер",
		URL:        "https://example.com",
		SourceType: "html",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp handler.ScheduleResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "stub-task-id", resp.TaskID)
	assert.Equal(t, "enqueued", resp.Status)
	assert.Len(t, stub.enqueued, 1)
}

func TestScheduleHandler_MissingBrand(t *testing.T) {
	h := handler.New(&stubEnqueuer{})

	body, _ := json.Marshal(handler.ScheduleRequest{
		URL:        "https://example.com",
		SourceType: "html",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp handler.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error, "brand")
}

func TestScheduleHandler_InvalidSourceType(t *testing.T) {
	h := handler.New(&stubEnqueuer{})

	body, _ := json.Marshal(handler.ScheduleRequest{
		Brand:      "сбер",
		URL:        "https://example.com",
		SourceType: "ftp",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleHandler_InvalidURL(t *testing.T) {
	h := handler.New(&stubEnqueuer{})

	body, _ := json.Marshal(handler.ScheduleRequest{
		Brand:      "сбер",
		URL:        "not-a-url",
		SourceType: "html",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleHandler_WrongMethod(t *testing.T) {
	h := handler.New(&stubEnqueuer{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/schedule", nil)

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestScheduleHandler_EnqueueError(t *testing.T) {
	stub := &stubEnqueuer{err: assert.AnError}
	h := handler.New(stub)

	body, _ := json.Marshal(handler.ScheduleRequest{
		Brand:      "сбер",
		URL:        "https://example.com",
		SourceType: "html",
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/schedule", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestScheduleHandler_Health(t *testing.T) {
	h := handler.New(&stubEnqueuer{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
