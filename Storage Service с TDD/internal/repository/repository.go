package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brandmon/storage-service/internal/model"
)

var ErrNotFound = errors.New("record not found")

// ─── ScrapeJobRepository ──────────────────────────────────────────────────────

type ScrapeJobRepo struct {
	db *pgxpool.Pool
}

func NewScrapeJobRepo(db *pgxpool.Pool) *ScrapeJobRepo {
	return &ScrapeJobRepo{db: db}
}

func (r *ScrapeJobRepo) Create(ctx context.Context, job *model.ScrapeJob) (*model.ScrapeJob, error) {
	const q = `
		INSERT INTO scrape_jobs (brand, url, source_type, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, brand, url, source_type, status, error_msg,
		          created_at, updated_at, finished_at`

	var result model.ScrapeJob
	err := r.db.QueryRow(ctx, q, job.Brand, job.URL, job.SourceType).
		Scan(&result.ID, &result.Brand, &result.URL, &result.SourceType,
			&result.Status, &result.ErrorMsg,
			&result.CreatedAt, &result.UpdatedAt, &result.FinishedAt)
	if err != nil {
		return nil, fmt.Errorf("create scrape job: %w", err)
	}
	return &result, nil
}

func (r *ScrapeJobRepo) GetByID(ctx context.Context, id string) (*model.ScrapeJob, error) {
	const q = `
		SELECT id, brand, url, source_type, status, error_msg,
		       created_at, updated_at, finished_at
		FROM scrape_jobs WHERE id = $1`

	var job model.ScrapeJob
	err := r.db.QueryRow(ctx, q, id).
		Scan(&job.ID, &job.Brand, &job.URL, &job.SourceType,
			&job.Status, &job.ErrorMsg,
			&job.CreatedAt, &job.UpdatedAt, &job.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get scrape job: %w", err)
	}
	return &job, nil
}

func (r *ScrapeJobRepo) UpdateStatus(ctx context.Context, id string, status model.JobStatus, errMsg string) error {
	const q = `
		UPDATE scrape_jobs
		SET status     = $2,
		    error_msg  = NULLIF($3, ''),
		    updated_at = now(),
		    finished_at = CASE WHEN $2 IN ('done', 'failed') THEN now() ELSE NULL END
		WHERE id = $1`

	tag, err := r.db.Exec(ctx, q, id, status, errMsg)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ScrapeJobRepo) ListByStatus(ctx context.Context, status model.JobStatus, limit int) ([]*model.ScrapeJob, error) {
	const q = `
		SELECT id, brand, url, source_type, status, error_msg,
		       created_at, updated_at, finished_at
		FROM scrape_jobs
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := r.db.Query(ctx, q, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list by status: %w", err)
	}
	defer rows.Close()

	var jobs []*model.ScrapeJob
	for rows.Next() {
		var job model.ScrapeJob
		if err := rows.Scan(&job.ID, &job.Brand, &job.URL, &job.SourceType,
			&job.Status, &job.ErrorMsg,
			&job.CreatedAt, &job.UpdatedAt, &job.FinishedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, rows.Err()
}

// ─── RawPageRepository ────────────────────────────────────────────────────────

type RawPageRepo struct {
	db *pgxpool.Pool
}

func NewRawPageRepo(db *pgxpool.Pool) *RawPageRepo {
	return &RawPageRepo{db: db}
}

func (r *RawPageRepo) Save(ctx context.Context, page *model.RawPage) (*model.RawPage, error) {
	const q = `
		INSERT INTO raw_pages (job_id, brand, source_url, source_type, raw_html)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5)
		RETURNING id, COALESCE(job_id::text, ''), brand, source_url, source_type,
		          raw_html, scraped_at`

	var result model.RawPage
	err := r.db.QueryRow(ctx, q,
		page.JobID, page.Brand, page.SourceURL, page.SourceType, page.RawHTML).
		Scan(&result.ID, &result.JobID, &result.Brand, &result.SourceURL,
			&result.SourceType, &result.RawHTML, &result.ScrapedAt)
	if err != nil {
		return nil, fmt.Errorf("save raw page: %w", err)
	}
	return &result, nil
}

func (r *RawPageRepo) GetByBrand(ctx context.Context, brand string, limit int) ([]*model.RawPage, error) {
	const q = `
		SELECT id, COALESCE(job_id::text, ''), brand, source_url, source_type,
		       raw_html, scraped_at
		FROM raw_pages WHERE brand = $1
		ORDER BY scraped_at DESC
		LIMIT $2`

	rows, err := r.db.Query(ctx, q, brand, limit)
	if err != nil {
		return nil, fmt.Errorf("get pages by brand: %w", err)
	}
	defer rows.Close()

	var pages []*model.RawPage
	for rows.Next() {
		var p model.RawPage
		if err := rows.Scan(&p.ID, &p.JobID, &p.Brand, &p.SourceURL,
			&p.SourceType, &p.RawHTML, &p.ScrapedAt); err != nil {
			return nil, err
		}
		pages = append(pages, &p)
	}
	return pages, rows.Err()
}

// ─── ParsedWordRepository ─────────────────────────────────────────────────────

type ParsedWordRepo struct {
	db *pgxpool.Pool
}

func NewParsedWordRepo(db *pgxpool.Pool) *ParsedWordRepo {
	return &ParsedWordRepo{db: db}
}

func (r *ParsedWordRepo) SaveBatch(ctx context.Context, words []model.ParsedWord) error {
	if len(words) == 0 {
		return nil
	}

	// Используем batch для эффективной вставки
	batch := &pgx.Batch{}
	const q = `
		INSERT INTO parsed_words (brand, source_url, word, frequency)
		VALUES ($1, $2, $3, $4)`

	for _, w := range words {
		batch.Queue(q, w.Brand, w.SourceURL, w.Word, w.Frequency)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range words {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("save word batch: %w", err)
		}
	}
	return nil
}

func (r *ParsedWordRepo) GetTopWords(ctx context.Context, q model.TopWordsQuery) ([]model.WordFreqResult, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}

	const query = `
		SELECT word, SUM(frequency) as total_freq
		FROM parsed_words
		WHERE brand = $1
		GROUP BY word
		ORDER BY total_freq DESC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, q.Brand, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("get top words: %w", err)
	}
	defer rows.Close()

	var results []model.WordFreqResult
	for rows.Next() {
		var r model.WordFreqResult
		if err := rows.Scan(&r.Word, &r.Frequency); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ─── CooccurrenceRepository ───────────────────────────────────────────────────

type CooccurrenceRepo struct {
	db *pgxpool.Pool
}

func NewCooccurrenceRepo(db *pgxpool.Pool) *CooccurrenceRepo {
	return &CooccurrenceRepo{db: db}
}

func (r *CooccurrenceRepo) SaveBatch(ctx context.Context, entries []model.WordCooccurrence) error {
	if len(entries) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	const q = `
		INSERT INTO word_cooccurrence (brand, target_word, neighbor, weight, source_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (brand, target_word, neighbor, source_url)
		DO UPDATE SET weight = EXCLUDED.weight, scraped_at = now()`

	for _, e := range entries {
		batch.Queue(q, e.Brand, e.TargetWord, e.Neighbor, e.Weight, e.SourceURL)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range entries {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("save cooccurrence batch: %w", err)
		}
	}
	return nil
}

func (r *CooccurrenceRepo) GetNeighbors(ctx context.Context, q model.CooccurrenceQuery) ([]model.CooccurrenceResult, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}

	const query = `
		SELECT neighbor, weight
		FROM word_cooccurrence
		WHERE brand = $1 AND target_word = $2
		ORDER BY weight DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, q.Brand, q.TargetWord, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("get neighbors: %w", err)
	}
	defer rows.Close()

	var results []model.CooccurrenceResult
	for rows.Next() {
		var r model.CooccurrenceResult
		if err := rows.Scan(&r.Neighbor, &r.Weight); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ─── Storage — фасад для транзакционного сохранения ──────────────────────────

type ScrapedData struct {
	Page          model.RawPage
	Words         []model.ParsedWord
	Cooccurrences []model.WordCooccurrence
}

type Storage struct {
	db            *pgxpool.Pool
	pageRepo      *RawPageRepo
	wordRepo      *ParsedWordRepo
	cooccRepo     *CooccurrenceRepo
}

func NewStorage(db *pgxpool.Pool) *Storage {
	return &Storage{
		db:        db,
		pageRepo:  NewRawPageRepo(db),
		wordRepo:  NewParsedWordRepo(db),
		cooccRepo: NewCooccurrenceRepo(db),
	}
}

// SaveScrapedData атомарно сохраняет страницу + слова + co-occurrence в одной транзакции.
func (s *Storage) SaveScrapedData(ctx context.Context, data ScrapedData) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Сохраняем сырую страницу
	data.Page.ScrapedAt = time.Now().UTC()
	const pageQ = `
		INSERT INTO raw_pages (job_id, brand, source_url, source_type, raw_html, scraped_at)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5, $6)`

	if _, err := tx.Exec(ctx, pageQ,
		data.Page.JobID, data.Page.Brand, data.Page.SourceURL,
		data.Page.SourceType, data.Page.RawHTML, data.Page.ScrapedAt); err != nil {
		return fmt.Errorf("insert raw page: %w", err)
	}

	// 2. Сохраняем слова
	if len(data.Words) > 0 {
		const wordQ = `INSERT INTO parsed_words (brand, source_url, word, frequency) VALUES ($1, $2, $3, $4)`
		for _, w := range data.Words {
			if _, err := tx.Exec(ctx, wordQ, w.Brand, w.SourceURL, w.Word, w.Frequency); err != nil {
				return fmt.Errorf("insert word %q: %w", w.Word, err)
			}
		}
	}

	// 3. Сохраняем co-occurrence
	if len(data.Cooccurrences) > 0 {
		const coQ = `
			INSERT INTO word_cooccurrence (brand, target_word, neighbor, weight, source_url)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (brand, target_word, neighbor, source_url)
			DO UPDATE SET weight = EXCLUDED.weight`
		for _, c := range data.Cooccurrences {
			if _, err := tx.Exec(ctx, coQ, c.Brand, c.TargetWord, c.Neighbor, c.Weight, c.SourceURL); err != nil {
				return fmt.Errorf("insert cooccurrence: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}
