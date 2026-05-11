package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandmon/normalizer-service/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestJSONContentType_SetsHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	middleware.JSONContentType(next).ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestRequestID_AddsHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// request-id должен быть в контексте/заголовке
		assert.NotEmpty(t, r.Header.Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	middleware.RequestID(next).ServeHTTP(w, r)
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	existingID := "test-request-123"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, existingID, r.Header.Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", existingID)

	middleware.RequestID(next).ServeHTTP(w, r)
}

func TestRecovery_HandlesPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	// Не должен паниковать
	assert.NotPanics(t, func() {
		middleware.Recovery(next).ServeHTTP(w, r)
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
