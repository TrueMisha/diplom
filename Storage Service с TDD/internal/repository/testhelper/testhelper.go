package testhelper

// Этот пакет поднимает реальный PostgreSQL в Docker через testcontainers-go.
// Каждый тест получает чистую БД — полная изоляция без моков.
//
// Использование в тестах:
//
//   func TestMain(m *testing.M) {
//       testhelper.Setup()        // поднять контейнер
//       code := m.Run()
//       testhelper.Teardown()     // убить контейнер
//       os.Exit(code)
//   }
//
//   func TestSomething(t *testing.T) {
//       db := testhelper.NewDB(t)  // чистая БД на каждый тест
//       defer testhelper.CleanDB(t, db)
//   }

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	container *postgres.PostgresContainer
	dsn       string
)

// Setup поднимает Postgres-контейнер. Вызывается один раз для всех тестов пакета.
func Setup() {
	ctx := context.Background()

	c, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("brandmon_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testcontainers: start postgres: %v\n", err)
		os.Exit(1)
	}

	container = c

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testcontainers: get connection string: %v\n", err)
		os.Exit(1)
	}
	dsn = connStr
}

// Teardown останавливает контейнер.
func Teardown() {
	if container != nil {
		_ = container.Terminate(context.Background())
	}
}

// NewDB создаёт пул соединений и прогоняет миграции.
// Каждый тест получает свою схему (через search_path).
func NewDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}

	// Создаём изолированную схему для теста
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf("SET search_path TO %s", schema)); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	// Прогоняем миграции в изолированной схеме
	RunMigrations(t, pool)

	t.Cleanup(func() {
		pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
		pool.Close()
	})

	return pool
}

// RunMigrations применяет SQL-миграции к пулу.
func RunMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS scrape_jobs (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			brand       TEXT        NOT NULL,
			url         TEXT        NOT NULL,
			source_type TEXT        NOT NULL,
			status      TEXT        NOT NULL DEFAULT 'pending',
			error_msg   TEXT,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			finished_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS raw_pages (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			job_id      UUID,
			brand       TEXT        NOT NULL,
			source_url  TEXT        NOT NULL,
			source_type TEXT        NOT NULL DEFAULT 'html',
			raw_html    TEXT,
			scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS parsed_words (
			id          BIGSERIAL   PRIMARY KEY,
			brand       TEXT        NOT NULL,
			source_url  TEXT        NOT NULL,
			word        TEXT        NOT NULL,
			frequency   INT         NOT NULL DEFAULT 1,
			scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS word_cooccurrence (
			id          BIGSERIAL   PRIMARY KEY,
			brand       TEXT        NOT NULL,
			target_word TEXT        NOT NULL,
			neighbor    TEXT        NOT NULL,
			weight      INT         NOT NULL DEFAULT 1,
			source_url  TEXT        NOT NULL,
			scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE (brand, target_word, neighbor, source_url)
		)`,
	}

	ctx := context.Background()
	for _, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("migration failed: %v\nSQL: %s", err, sql)
		}
	}
}
