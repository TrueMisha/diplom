package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brandmon/storage-service/internal/handler"
	"github.com/brandmon/storage-service/internal/repository"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// migrations — DDL, применяемый при старте если БД пуста.
const migrations = `
CREATE TABLE IF NOT EXISTS scrape_jobs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    brand       TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    source_type TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending',
    error_msg   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS parsed_words (
    id          BIGSERIAL   PRIMARY KEY,
    brand       TEXT        NOT NULL,
    source_url  TEXT        NOT NULL,
    word        TEXT        NOT NULL,
    frequency   INT         NOT NULL DEFAULT 1,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrape_jobs_brand      ON scrape_jobs(brand);
CREATE INDEX IF NOT EXISTS idx_scrape_jobs_status     ON scrape_jobs(status);
CREATE INDEX IF NOT EXISTS idx_parsed_words_brand     ON parsed_words(brand);
CREATE INDEX IF NOT EXISTS idx_parsed_words_brand_word ON parsed_words(brand, word);

CREATE TABLE IF NOT EXISTS reviews (
    id          BIGSERIAL   PRIMARY KEY,
    job_id      UUID,
    brand       TEXT        NOT NULL,
    source_url  TEXT        NOT NULL,
    title       TEXT        NOT NULL DEFAULT '',
    text        TEXT        NOT NULL DEFAULT '',
    rating      TEXT,
    review_date TEXT,
    pros        TEXT        NOT NULL DEFAULT '',
    cons        TEXT        NOT NULL DEFAULT '',
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reviews_brand      ON reviews(brand);
CREATE INDEX IF NOT EXISTS idx_reviews_scraped_at ON reviews(scraped_at DESC);
`

func main() {
	port := getEnv("PORT", "8083")
	databaseURL := os.Getenv("DATABASE_URL")

	var h *handler.Handler

	if databaseURL != "" {
		// ── Режим Postgres ────────────────────────────────────────────────
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Fatalf("connect to postgres: %v", err)
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("ping postgres: %v", err)
		}

		if _, err := pool.Exec(ctx, migrations); err != nil {
			log.Fatalf("run migrations: %v", err)
		}

		jobRepo    := repository.NewScrapeJobRepo(pool)
		wordRepo   := repository.NewParsedWordRepo(pool)
		reviewRepo := repository.NewReviewRepo(pool)

		h = handler.New(handler.WithPgStore(jobRepo, wordRepo, reviewRepo))
		log.Printf("storage-service: using PostgreSQL (%s)", databaseURL)
	} else {

		h = handler.New(handler.WithMemoryStore())
		log.Printf("storage-service: using in-memory store (DATABASE_URL not set)")
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("storage-service listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
