package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/brandmon/storage-service/internal/model"
	"github.com/brandmon/storage-service/internal/repository"
)

// ─── Request / Response types ─────────────────────────────────────────────────

type CreateJobRequest struct {
	Brand      string `json:"brand"`
	URL        string `json:"url"`
	SourceType string `json:"source_type"`
}

type JobResponse struct {
	ID         string  `json:"id"`
	Brand      string  `json:"brand"`
	URL        string  `json:"url"`
	SourceType string  `json:"source_type"`
	Status     string  `json:"status"`
	ErrorMsg   string  `json:"error_msg,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type SaveWordsRequest struct {
	Brand     string      `json:"brand"`
	SourceURL string      `json:"source_url"`
	Words     []WordEntry `json:"words"`
}

type WordEntry struct {
	Word      string `json:"word"`
	Frequency int    `json:"frequency"`
}

type TopWordsResponse struct {
	Brand string                 `json:"brand"`
	Words []model.WordFreqResult `json:"words"`
}

type SaveReviewsRequest struct {
	Brand     string         `json:"brand"`
	SourceURL string         `json:"source_url"`
	JobID     string         `json:"job_id,omitempty"`
	Reviews   []ReviewEntry  `json:"reviews"`
}

type ReviewEntry struct {
	Title      string `json:"title"`
	Text       string `json:"text"`
	Rating     string `json:"rating,omitempty"`
	ReviewDate string `json:"review_date,omitempty"`
	Pros       string `json:"pros,omitempty"`
	Cons       string `json:"cons,omitempty"`
}

type ReviewsResponse struct {
	Brand   string         `json:"brand"`
	Total   int            `json:"total"`
	Reviews []model.Review `json:"reviews"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ─── Store interface — позволяет подменять реальный Postgres на in-memory ─────

type Store interface {
	CreateJob(ctx context.Context, job *model.ScrapeJob) (*model.ScrapeJob, error)
	GetJobByID(ctx context.Context, id string) (*model.ScrapeJob, error)
	SaveWords(ctx context.Context, words []model.ParsedWord) error
	GetTopWords(ctx context.Context, q model.TopWordsQuery) ([]model.WordFreqResult, error)
	SaveReviews(ctx context.Context, reviews []model.Review) error
	GetReviews(ctx context.Context, brand string, limit int) ([]model.Review, error)
	GetOrCreateBrand(ctx context.Context, name string) (*model.Brand, error)
	ListBrands(ctx context.Context) ([]*model.Brand, error)
}

// ─── pgStore — реальная реализация через репозитории ─────────────────────────

type pgStore struct {
	jobs    *repository.ScrapeJobRepo
	words   *repository.ParsedWordRepo
	reviews *repository.ReviewRepo
	brands  *repository.BrandRepo
}

func (s *pgStore) CreateJob(ctx context.Context, job *model.ScrapeJob) (*model.ScrapeJob, error) {
	return s.jobs.Create(ctx, job)
}
func (s *pgStore) GetJobByID(ctx context.Context, id string) (*model.ScrapeJob, error) {
	return s.jobs.GetByID(ctx, id)
}
func (s *pgStore) SaveWords(ctx context.Context, words []model.ParsedWord) error {
	return s.words.SaveBatch(ctx, words)
}
func (s *pgStore) GetTopWords(ctx context.Context, q model.TopWordsQuery) ([]model.WordFreqResult, error) {
	return s.words.GetTopWords(ctx, q)
}
func (s *pgStore) SaveReviews(ctx context.Context, reviews []model.Review) error {
	return s.reviews.SaveBatch(ctx, reviews)
}
func (s *pgStore) GetReviews(ctx context.Context, brand string, limit int) ([]model.Review, error) {
	return s.reviews.GetByBrand(ctx, brand, limit)
}
func (s *pgStore) GetOrCreateBrand(ctx context.Context, name string) (*model.Brand, error) {
	return s.brands.GetOrCreate(ctx, name)
}
func (s *pgStore) ListBrands(ctx context.Context) ([]*model.Brand, error) {
	return s.brands.List(ctx)
}

// ─── memStore — in-memory реализация для unit-тестов handler ─────────────────

type memStore struct {
	mu      sync.RWMutex
	jobs    map[string]*model.ScrapeJob
	words   []model.ParsedWord
	reviews []model.Review
	brands  map[string]*model.Brand
	nextID  int64
}

func newMemStore() *memStore {
	return &memStore{
		jobs:   make(map[string]*model.ScrapeJob),
		brands: make(map[string]*model.Brand),
	}
}

func (s *memStore) CreateJob(_ context.Context, job *model.ScrapeJob) (*model.ScrapeJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := *job
	j.ID = uuid.New().String()
	j.Status = model.JobStatusPending
	j.CreatedAt = time.Now()
	j.UpdatedAt = time.Now()
	s.jobs[j.ID] = &j
	return &j, nil
}

func (s *memStore) GetJobByID(_ context.Context, id string) (*model.ScrapeJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return j, nil
}

func (s *memStore) SaveWords(_ context.Context, words []model.ParsedWord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.words = append(s.words, words...)
	return nil
}

func (s *memStore) SaveReviews(_ context.Context, reviews []model.Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reviews = append(s.reviews, reviews...)
	return nil
}

func (s *memStore) GetReviews(_ context.Context, brand string, limit int) ([]model.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	var results []model.Review
	for _, r := range s.reviews {
		if r.Brand == brand {
			results = append(results, r)
		}
	}
	if limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

func (s *memStore) GetOrCreateBrand(_ context.Context, name string) (*model.Brand, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.brands[name]; ok {
		return b, nil
	}
	s.nextID++
	b := &model.Brand{ID: s.nextID, Name: name, CreatedAt: time.Now()}
	s.brands[name] = b
	return b, nil
}

func (s *memStore) ListBrands(_ context.Context) ([]*model.Brand, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*model.Brand, 0, len(s.brands))
	for _, b := range s.brands {
		list = append(list, b)
	}
	return list, nil
}

func (s *memStore) GetTopWords(_ context.Context, q model.TopWordsQuery) ([]model.WordFreqResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[string]int{}
	for _, w := range s.words {
		if w.Brand == q.Brand {
			counts[w.Word] += w.Frequency
		}
	}

	results := make([]model.WordFreqResult, 0, len(counts))
	for word, freq := range counts {
		results = append(results, model.WordFreqResult{Word: word, Frequency: freq})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Frequency > results[j].Frequency
	})

	limit := q.Limit
	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	return results[:limit], nil
}

// ─── Option functions ─────────────────────────────────────────────────────────

type option func(*Handler)

func WithMemoryStore() option {
	return func(h *Handler) { h.store = newMemStore() }
}

func WithPgStore(jobs *repository.ScrapeJobRepo, words *repository.ParsedWordRepo, reviews *repository.ReviewRepo, brands *repository.BrandRepo) option {
	return func(h *Handler) {
		h.store = &pgStore{jobs: jobs, words: words, reviews: reviews, brands: brands}
	}
}

// ─── Handler ─────────────────────────────────────────────────────────────────

type Handler struct {
	mux   *http.ServeMux
	store Store
}

func New(opts ...option) *Handler {
	h := &Handler{mux: http.NewServeMux()}
	for _, o := range opts {
		o(h)
	}
	if h.store == nil {
		h.store = newMemStore() // дефолт
	}
	h.mux.HandleFunc("/jobs", h.handleJobs)
	h.mux.HandleFunc("/jobs/", h.handleJobByID)
	h.mux.HandleFunc("/words", h.handleWords)
	h.mux.HandleFunc("/reviews", h.handleReviews)
	h.mux.HandleFunc("/brands", h.handleBrands)
	h.mux.HandleFunc("/health", h.handleHealth)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ─── /jobs ────────────────────────────────────────────────────────────────────

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createJob(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	job := &model.ScrapeJob{
		Brand:      req.Brand,
		URL:        req.URL,
		SourceType: model.SourceType(req.SourceType),
	}
	if err := job.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.CreateJob(r.Context(), job)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toJobResponse(created))
}

// ─── /jobs/:id ────────────────────────────────────────────────────────────────

func (h *Handler) handleJobByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, err := h.store.GetJobByID(r.Context(), id)
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toJobResponse(job))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ─── /words ───────────────────────────────────────────────────────────────────

func (h *Handler) handleWords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.saveWords(w, r)
	case http.MethodGet:
		h.getTopWords(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) saveWords(w http.ResponseWriter, r *http.Request) {
	var req SaveWordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Brand) == "" {
		writeError(w, http.StatusBadRequest, "brand is required")
		return
	}

	words := make([]model.ParsedWord, len(req.Words))
	for i, e := range req.Words {
		words[i] = model.ParsedWord{
			Brand: req.Brand, SourceURL: req.SourceURL,
			Word: e.Word, Frequency: e.Frequency,
		}
	}

	if err := h.store.SaveWords(r.Context(), words); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int{"saved": len(words)})
}

func (h *Handler) getTopWords(w http.ResponseWriter, r *http.Request) {
	brand := r.URL.Query().Get("brand")
	if brand == "" {
		writeError(w, http.StatusBadRequest, "brand query parameter is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}

	results, err := h.store.GetTopWords(r.Context(), model.TopWordsQuery{Brand: brand, Limit: limit})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, TopWordsResponse{Brand: brand, Words: results})
}

// ─── /reviews ────────────────────────────────────────────────────────────────

func (h *Handler) handleReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.saveReviews(w, r)
	case http.MethodGet:
		h.getReviews(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) saveReviews(w http.ResponseWriter, r *http.Request) {
	var req SaveReviewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Brand) == "" {
		writeError(w, http.StatusBadRequest, "brand is required")
		return
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		writeError(w, http.StatusBadRequest, "source_url is required")
		return
	}

	brand, err := h.store.GetOrCreateBrand(r.Context(), req.Brand)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	reviews := make([]model.Review, len(req.Reviews))
	for i, e := range req.Reviews {
		reviews[i] = model.Review{
			BrandID:    brand.ID,
			JobID:      req.JobID,
			Brand:      req.Brand,
			SourceURL:  req.SourceURL,
			Title:      e.Title,
			Text:       e.Text,
			Rating:     e.Rating,
			ReviewDate: e.ReviewDate,
			Pros:       e.Pros,
			Cons:       e.Cons,
		}
	}

	if err := h.store.SaveReviews(r.Context(), reviews); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"saved": len(reviews), "brand_id": brand.ID})
}

func (h *Handler) getReviews(w http.ResponseWriter, r *http.Request) {
	brand := r.URL.Query().Get("brand")
	if brand == "" {
		writeError(w, http.StatusBadRequest, "brand query parameter is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}

	reviews, err := h.store.GetReviews(r.Context(), brand, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ReviewsResponse{
		Brand:   brand,
		Total:   len(reviews),
		Reviews: reviews,
	})
}

// ─── /brands ─────────────────────────────────────────────────────────────────

func (h *Handler) handleBrands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	brands, err := h.store.ListBrands(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if brands == nil {
		brands = []*model.Brand{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"brands": brands})
}

// ─── /health ─────────────────────────────────────────────────────────────────

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func toJobResponse(j *model.ScrapeJob) JobResponse {
	return JobResponse{
		ID:         j.ID,
		Brand:      j.Brand,
		URL:        j.URL,
		SourceType: string(j.SourceType),
		Status:     string(j.Status),
		ErrorMsg:   j.ErrorMsg,
		CreatedAt:  j.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
