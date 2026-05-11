package repository_test

// TDD: тесты репозитория работают с РЕАЛЬНЫМ Postgres.
// testcontainers-go поднимает контейнер один раз для всего пакета.
// Каждый тест получает изолированную схему — полная независимость.

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/brandmon/storage-service/internal/model"
	"github.com/brandmon/storage-service/internal/repository"
	"github.com/brandmon/storage-service/internal/repository/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain — поднимаем контейнер ОДИН РАЗ для всех тестов пакета.
func TestMain(m *testing.M) {
	testhelper.Setup()
	code := m.Run()
	testhelper.Teardown()
	os.Exit(code)
}

// ─── ScrapeJobRepository ──────────────────────────────────────────────────────

func TestScrapeJobRepo_Create(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	job := &model.ScrapeJob{
		Brand:      "сбер",
		URL:        "https://banki.ru/sber",
		SourceType: model.SourceTypeHTML,
	}

	created, err := repo.Create(context.Background(), job)

	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, model.JobStatusPending, created.Status)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Equal(t, "сбер", created.Brand)
}

func TestScrapeJobRepo_GetByID_Found(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	job := &model.ScrapeJob{
		Brand: "vtb", URL: "https://vtb.ru", SourceType: model.SourceTypeHTML,
	}
	created, err := repo.Create(context.Background(), job)
	require.NoError(t, err)

	found, err := repo.GetByID(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "vtb", found.Brand)
}

func TestScrapeJobRepo_GetByID_NotFound(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	_, err := repo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")

	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestScrapeJobRepo_UpdateStatus_ToRunning(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	job, _ := repo.Create(context.Background(), &model.ScrapeJob{
		Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
	})

	err := repo.UpdateStatus(context.Background(), job.ID, model.JobStatusRunning, "")

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), job.ID)
	assert.Equal(t, model.JobStatusRunning, updated.Status)
	assert.Nil(t, updated.FinishedAt)
}

func TestScrapeJobRepo_UpdateStatus_ToDone_SetsFinishedAt(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	job, _ := repo.Create(context.Background(), &model.ScrapeJob{
		Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
	})

	err := repo.UpdateStatus(context.Background(), job.ID, model.JobStatusDone, "")

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), job.ID)
	assert.Equal(t, model.JobStatusDone, updated.Status)
	assert.NotNil(t, updated.FinishedAt) // finished_at должен выставиться
}

func TestScrapeJobRepo_UpdateStatus_ToFailed_SavesError(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	job, _ := repo.Create(context.Background(), &model.ScrapeJob{
		Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
	})

	err := repo.UpdateStatus(context.Background(), job.ID, model.JobStatusFailed, "connection refused")

	require.NoError(t, err)

	updated, _ := repo.GetByID(context.Background(), job.ID)
	assert.Equal(t, model.JobStatusFailed, updated.Status)
	assert.Equal(t, "connection refused", updated.ErrorMsg)
}

func TestScrapeJobRepo_UpdateStatus_NonExistentID(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	err := repo.UpdateStatus(context.Background(), "00000000-0000-0000-0000-000000000000", model.JobStatusDone, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestScrapeJobRepo_ListPending(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	ctx := context.Background()

	// Создаём 3 pending и 1 done
	for i := 0; i < 3; i++ {
		repo.Create(ctx, &model.ScrapeJob{
			Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
		})
	}
	doneJob, _ := repo.Create(ctx, &model.ScrapeJob{
		Brand: "vtb", URL: "https://vtb.ru", SourceType: model.SourceTypeHTML,
	})
	repo.UpdateStatus(ctx, doneJob.ID, model.JobStatusDone, "")

	pending, err := repo.ListByStatus(ctx, model.JobStatusPending, 10)

	require.NoError(t, err)
	assert.Len(t, pending, 3)
	for _, j := range pending {
		assert.Equal(t, model.JobStatusPending, j.Status)
	}
}

func TestScrapeJobRepo_ListPending_RespectsLimit(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewScrapeJobRepo(db)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		repo.Create(ctx, &model.ScrapeJob{
			Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
		})
	}

	jobs, err := repo.ListByStatus(ctx, model.JobStatusPending, 3)

	require.NoError(t, err)
	assert.Len(t, jobs, 3)
}

// ─── RawPageRepository ────────────────────────────────────────────────────────

func TestRawPageRepo_Save(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewRawPageRepo(db)

	page := &model.RawPage{
		Brand:      "сбер",
		SourceURL:  "https://banki.ru/sber/reviews",
		SourceType: "html",
		RawHTML:    "<html><body>Отличный банк</body></html>",
	}

	saved, err := repo.Save(context.Background(), page)

	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.False(t, saved.ScrapedAt.IsZero())
}

func TestRawPageRepo_Save_WithJobID(t *testing.T) {
	db := testhelper.NewDB(t)
	jobRepo := repository.NewScrapeJobRepo(db)
	pageRepo := repository.NewRawPageRepo(db)

	ctx := context.Background()
	job, _ := jobRepo.Create(ctx, &model.ScrapeJob{
		Brand: "сбер", URL: "https://banki.ru", SourceType: model.SourceTypeHTML,
	})

	page := &model.RawPage{
		JobID: job.ID, Brand: "сбер",
		SourceURL: "https://banki.ru", SourceType: "html",
		RawHTML: "<p>текст</p>",
	}

	saved, err := pageRepo.Save(ctx, page)

	require.NoError(t, err)
	assert.Equal(t, job.ID, saved.JobID)
}

func TestRawPageRepo_GetByBrand(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewRawPageRepo(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		repo.Save(ctx, &model.RawPage{
			Brand: "сбер", SourceURL: "https://banki.ru",
			SourceType: "html", RawHTML: "<p>текст</p>",
		})
	}
	repo.Save(ctx, &model.RawPage{
		Brand: "втб", SourceURL: "https://vtb.ru",
		SourceType: "html", RawHTML: "<p>другой</p>",
	})

	pages, err := repo.GetByBrand(ctx, "сбер", 10)

	require.NoError(t, err)
	assert.Len(t, pages, 3)
	for _, p := range pages {
		assert.Equal(t, "сбер", p.Brand)
	}
}

// ─── ParsedWordRepository ─────────────────────────────────────────────────────

func TestParsedWordRepo_SaveBatch(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewParsedWordRepo(db)

	words := []model.ParsedWord{
		{Brand: "сбер", SourceURL: "https://banki.ru", Word: "банк", Frequency: 15},
		{Brand: "сбер", SourceURL: "https://banki.ru", Word: "надёжный", Frequency: 8},
		{Brand: "сбер", SourceURL: "https://banki.ru", Word: "удобный", Frequency: 5},
	}

	err := repo.SaveBatch(context.Background(), words)

	require.NoError(t, err)
}

func TestParsedWordRepo_SaveBatch_Empty(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewParsedWordRepo(db)

	// Пустой батч — не ошибка
	err := repo.SaveBatch(context.Background(), []model.ParsedWord{})

	assert.NoError(t, err)
}

func TestParsedWordRepo_GetTopWords(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewParsedWordRepo(db)
	ctx := context.Background()

	words := []model.ParsedWord{
		{Brand: "сбер", SourceURL: "https://a.ru", Word: "банк", Frequency: 50},
		{Brand: "сбер", SourceURL: "https://a.ru", Word: "надёжный", Frequency: 30},
		{Brand: "сбер", SourceURL: "https://a.ru", Word: "удобный", Frequency: 10},
		{Brand: "втб", SourceURL: "https://b.ru", Word: "кредит", Frequency: 100}, // другой бренд
	}
	require.NoError(t, repo.SaveBatch(ctx, words))

	top, err := repo.GetTopWords(ctx, model.TopWordsQuery{Brand: "сбер", Limit: 2})

	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, "банк", top[0].Word)     // наибольшая частота
	assert.Equal(t, 50, top[0].Frequency)
	assert.Equal(t, "надёжный", top[1].Word)
}

func TestParsedWordRepo_GetTopWords_AggregatesAcrossSources(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewParsedWordRepo(db)
	ctx := context.Background()

	// Слово "банк" встречается на двух разных источниках — должна суммироваться
	words := []model.ParsedWord{
		{Brand: "сбер", SourceURL: "https://site1.ru", Word: "банк", Frequency: 20},
		{Brand: "сбер", SourceURL: "https://site2.ru", Word: "банк", Frequency: 15},
		{Brand: "сбер", SourceURL: "https://site1.ru", Word: "приложение", Frequency: 10},
	}
	require.NoError(t, repo.SaveBatch(ctx, words))

	top, err := repo.GetTopWords(ctx, model.TopWordsQuery{Brand: "сбер", Limit: 10})

	require.NoError(t, err)
	freqMap := make(map[string]int)
	for _, r := range top {
		freqMap[r.Word] = r.Frequency
	}

	assert.Equal(t, 35, freqMap["банк"])        // 20 + 15
	assert.Equal(t, 10, freqMap["приложение"])
}

func TestParsedWordRepo_GetTopWords_EmptyBrand(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewParsedWordRepo(db)

	top, err := repo.GetTopWords(context.Background(), model.TopWordsQuery{Brand: "несуществующий", Limit: 10})

	require.NoError(t, err)
	assert.Empty(t, top)
}

// ─── WordCooccurrenceRepository ───────────────────────────────────────────────

func TestCooccurrenceRepo_SaveBatch(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewCooccurrenceRepo(db)

	entries := []model.WordCooccurrence{
		{Brand: "сбер", TargetWord: "банк", Neighbor: "надёжный", Weight: 5, SourceURL: "https://a.ru"},
		{Brand: "сбер", TargetWord: "банк", Neighbor: "удобный", Weight: 3, SourceURL: "https://a.ru"},
	}

	err := repo.SaveBatch(context.Background(), entries)

	require.NoError(t, err)
}

func TestCooccurrenceRepo_SaveBatch_UpsertOnConflict(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewCooccurrenceRepo(db)
	ctx := context.Background()

	entry := model.WordCooccurrence{
		Brand: "сбер", TargetWord: "банк",
		Neighbor: "надёжный", Weight: 3, SourceURL: "https://a.ru",
	}

	require.NoError(t, repo.SaveBatch(ctx, []model.WordCooccurrence{entry}))

	// Сохраняем снова — должен обновить weight, не создать дубль
	entry.Weight = 7
	require.NoError(t, repo.SaveBatch(ctx, []model.WordCooccurrence{entry}))

	results, err := repo.GetNeighbors(ctx, model.CooccurrenceQuery{
		Brand: "сбер", TargetWord: "банк", Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 7, results[0].Weight) // обновился, не задублировался
}

func TestCooccurrenceRepo_GetNeighbors_SortedByWeight(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewCooccurrenceRepo(db)
	ctx := context.Background()

	entries := []model.WordCooccurrence{
		{Brand: "сбер", TargetWord: "банк", Neighbor: "удобный", Weight: 2, SourceURL: "https://a.ru"},
		{Brand: "сбер", TargetWord: "банк", Neighbor: "надёжный", Weight: 10, SourceURL: "https://a.ru"},
		{Brand: "сбер", TargetWord: "банк", Neighbor: "быстрый", Weight: 5, SourceURL: "https://a.ru"},
	}
	require.NoError(t, repo.SaveBatch(ctx, entries))

	results, err := repo.GetNeighbors(ctx, model.CooccurrenceQuery{
		Brand: "сбер", TargetWord: "банк", Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "надёжный", results[0].Neighbor) // наибольший weight
	assert.Equal(t, "быстрый", results[1].Neighbor)
	assert.Equal(t, "удобный", results[2].Neighbor)
}

func TestCooccurrenceRepo_GetNeighbors_FiltersByBrand(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewCooccurrenceRepo(db)
	ctx := context.Background()

	repo.SaveBatch(ctx, []model.WordCooccurrence{
		{Brand: "сбер", TargetWord: "банк", Neighbor: "надёжный", Weight: 5, SourceURL: "https://a.ru"},
		{Brand: "втб", TargetWord: "банк", Neighbor: "дорогой", Weight: 3, SourceURL: "https://b.ru"},
	})

	results, err := repo.GetNeighbors(ctx, model.CooccurrenceQuery{
		Brand: "сбер", TargetWord: "банк", Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "надёжный", results[0].Neighbor)
}

func TestCooccurrenceRepo_GetNeighbors_RespectsLimit(t *testing.T) {
	db := testhelper.NewDB(t)
	repo := repository.NewCooccurrenceRepo(db)
	ctx := context.Background()

	entries := make([]model.WordCooccurrence, 10)
	for i := range entries {
		entries[i] = model.WordCooccurrence{
			Brand: "сбер", TargetWord: "банк",
			Neighbor:  fmt.Sprintf("слово%d", i),
			Weight:    i + 1,
			SourceURL: "https://a.ru",
		}
	}
	require.NoError(t, repo.SaveBatch(ctx, entries))

	results, err := repo.GetNeighbors(ctx, model.CooccurrenceQuery{
		Brand: "сбер", TargetWord: "банк", Limit: 3,
	})

	require.NoError(t, err)
	assert.Len(t, results, 3)
}

// ─── Transaction test ─────────────────────────────────────────────────────────

func TestStorage_SavePageAndWords_Transaction(t *testing.T) {
	db := testhelper.NewDB(t)
	storage := repository.NewStorage(db)
	ctx := context.Background()

	// Всё вместе — страница + слова — должно сохраняться атомарно
	err := storage.SaveScrapedData(ctx, repository.ScrapedData{
		Page: model.RawPage{
			Brand: "сбер", SourceURL: "https://banki.ru",
			SourceType: "html", RawHTML: "<p>банк надёжный удобный</p>",
		},
		Words: []model.ParsedWord{
			{Brand: "сбер", SourceURL: "https://banki.ru", Word: "банк", Frequency: 3},
			{Brand: "сбер", SourceURL: "https://banki.ru", Word: "надёжный", Frequency: 1},
		},
		Cooccurrences: []model.WordCooccurrence{
			{Brand: "сбер", TargetWord: "банк", Neighbor: "надёжный", Weight: 2, SourceURL: "https://banki.ru"},
		},
	})

	require.NoError(t, err)

	// Проверяем что всё сохранилось
	wordRepo := repository.NewParsedWordRepo(db)
	top, err := wordRepo.GetTopWords(ctx, model.TopWordsQuery{Brand: "сбер", Limit: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, top)
}


