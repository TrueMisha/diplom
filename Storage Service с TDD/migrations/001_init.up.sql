-- 001_init.up.sql
-- Запускается автоматически при старте сервиса и в тестах

-- Задания на скрейпинг (создаются Scheduler Service)
CREATE TABLE IF NOT EXISTS scrape_jobs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    brand       TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    source_type TEXT        NOT NULL CHECK (source_type IN ('html', 'api', 'js')),
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'running', 'done', 'failed')),
    error_msg   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_scrape_jobs_brand  ON scrape_jobs(brand);
CREATE INDEX IF NOT EXISTS idx_scrape_jobs_status ON scrape_jobs(status);

-- Сырые спарсенные страницы (кладёт Scraper Service)
CREATE TABLE IF NOT EXISTS raw_pages (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id      UUID        REFERENCES scrape_jobs(id) ON DELETE SET NULL,
    brand       TEXT        NOT NULL,
    source_url  TEXT        NOT NULL,
    source_type TEXT        NOT NULL,
    raw_html    TEXT,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_raw_pages_brand ON raw_pages(brand);

-- Нормализованные слова (кладёт Normalizer Service)
CREATE TABLE IF NOT EXISTS parsed_words (
    id          BIGSERIAL   PRIMARY KEY,
    brand       TEXT        NOT NULL,
    source_url  TEXT        NOT NULL,
    word        TEXT        NOT NULL,
    frequency   INT         NOT NULL DEFAULT 1 CHECK (frequency > 0),
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_parsed_words_brand      ON parsed_words(brand);
CREATE INDEX IF NOT EXISTS idx_parsed_words_word       ON parsed_words(word);
CREATE INDEX IF NOT EXISTS idx_parsed_words_brand_word ON parsed_words(brand, word);

-- Co-occurrence матрица (близость слов)
CREATE TABLE IF NOT EXISTS word_cooccurrence (
    id          BIGSERIAL   PRIMARY KEY,
    brand       TEXT        NOT NULL,
    target_word TEXT        NOT NULL,
    neighbor    TEXT        NOT NULL,
    weight      INT         NOT NULL DEFAULT 1 CHECK (weight > 0),
    source_url  TEXT        NOT NULL,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (brand, target_word, neighbor, source_url)
);

CREATE INDEX IF NOT EXISTS idx_cooccurrence_brand_target ON word_cooccurrence(brand, target_word);
