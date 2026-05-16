# Diplom — Система анализа отзывов брендов

Микросервисная система на Go для автоматического парсинга, нормализации и хранения отзывов с сайтов irecommend.ru, banki.ru и otzovik.com.

---

## Архитектура

```
[Клиент]
    │
    ▼
[Scheduler Service :8084]  ──► Redis Queue
                                    │
                            [Scheduler Worker]
                                    │
                    ┌───────────────▼───────────────┐
                    │       Scraper Service :8082    │
                    │  (html / api / js парсинг)    │
                    └───────────────────────────────┘
                                    │
                    ┌───────────────▼───────────────┐
                    │      Storage Service :8083     │
                    │         PostgreSQL             │
                    └───────────────────────────────┘

[Normalizer Service :8081]  — независимый сервис нормализации текста
```

### Как данные проходят через систему

```
1. POST /schedule → задача попадает в Redis очередь
2. Worker забирает задачу → вызывает POST /scrape
3. Scraper парсит страницу → возвращает список отзывов целиком
4. Worker сохраняет отзывы → POST /reviews (таблица reviews в БД)
5. Результат доступен → GET /reviews?brand=Сбербанк

Параллельно можно:
- Нормализовать текст → POST /normalize (частота слов)
- Сохранить слова     → POST /words     (таблица parsed_words)
- Получить топ слов   → GET /words?brand=Сбербанк
```

### Что где хранится

| Таблица | Что содержит | Как получить |
|---|---|---|
| `reviews` | Целые отзывы (заголовок, текст, рейтинг, дата) | `GET /reviews?brand=...` |
| `parsed_words` | Слова из отзывов с частотой встречаемости | `GET /words?brand=...` |
| `word_cooccurrence` | Какие слова стоят рядом с целевым словом | через репозиторий напрямую |
| `scrape_jobs` | История задач парсинга со статусами | `GET /jobs/:id` |

---

## Требования

- Go 1.21+
- Docker Desktop
- GoLand (рекомендуется)

---

## Быстрый старт

### 1. Запуск инфраструктуры (Docker)

```bash
# Redis (очередь задач)
docker run -d --name redis -p 6379:6379 redis:7-alpine

# PostgreSQL (хранилище данных)
# Порт 5433 — если на машине уже установлен локальный PostgreSQL на 5432
docker run -d --name postgres \
  -p 5433:5432 \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=brandmon \
  postgres:16-alpine
```

Проверка:
```bash
docker exec redis redis-cli ping        # → PONG
docker exec postgres psql -U postgres -d brandmon -c "SELECT 'ok';"
```

### 2. Запуск сервисов

Открой **5 терминалов** (или используй GoLand Run Configurations):

**Normalizer (порт 8081)**
```bash
cd "Normalizer Service с TDD"
go run ./cmd/server
```

**Scraper (порт 8082)**
```bash
cd "Scraper Service с TDD"
go run ./cmd/server
```

**Storage (порт 8083)**
```bash
cd "Storage Service с TDD"
DATABASE_URL="postgres://postgres:postgres@127.0.0.1:5433/brandmon?sslmode=disable" \
go run ./cmd/server
```

**Scheduler Server (порт 8084)**
```bash
cd "Scheduler Service с TDD"
go run ./cmd/server
```

**Scheduler Worker**
```bash
cd "Scheduler Service с TDD"
SCRAPER_URL=http://localhost:8082 \
STORAGE_URL=http://localhost:8083 \
go run ./cmd/worker
```

### 3. Проверка

```bash
curl http://localhost:8081/health  # {"status":"ok"}
curl http://localhost:8082/health  # {"status":"ok"}
curl http://localhost:8083/health  # {"status":"ok"}
curl http://localhost:8084/health  # {"status":"ok"}
```

---

## Запуск через GoLand

В проекте уже настроены Run Configurations (`.idea/runConfigurations/`):

| Конфигурация | Описание |
|---|---|
| `Normalizer` | Normalizer Service :8081 |
| `Scraper` | Scraper Service :8082 |
| `Storage` | Storage Service :8083 + PostgreSQL |
| `Scheduler Server` | Scheduler HTTP :8084 |
| `Scheduler Worker` | Worker (Redis → Scraper → Storage) |
| **`All Services`** | Запуск всех 5 сервисов одной кнопкой |

Выбери `All Services` в выпадающем меню и нажми ▶.

---

## Переменные окружения

| Сервис | Переменная | По умолчанию | Описание |
|---|---|---|---|
| Storage | `DATABASE_URL` | — (in-memory) | Строка подключения к PostgreSQL |
| Storage | `PORT` | `8083` | Порт HTTP сервера |
| Normalizer | `PORT` | `8081` | Порт HTTP сервера |
| Scraper | `PORT` | `8082` | Порт HTTP сервера |
| Scheduler Server | `PORT` | `8084` | Порт HTTP сервера |
| Scheduler Server | `REDIS_ADDR` | `localhost:6379` | Адрес Redis |
| Scheduler Worker | `REDIS_ADDR` | `localhost:6379` | Адрес Redis |
| Scheduler Worker | `SCRAPER_URL` | `http://localhost:8082` | URL Scraper Service |
| Scheduler Worker | `STORAGE_URL` | `http://localhost:8083` | URL Storage Service |

---

## Подключение к базе данных

### Строка подключения

```
Host:     127.0.0.1
Port:     5433
Database: brandmon
User:     postgres
Password: postgres
```

### Через psql (терминал)

```bash
docker exec -it postgres psql -U postgres -d brandmon
```

### Через pgAdmin / DBeaver / DataGrip

1. Создай новое подключение → PostgreSQL
2. Заполни поля:
   - **Host**: `127.0.0.1`
   - **Port**: `5433`
   - **Database**: `brandmon`
   - **Username**: `postgres`
   - **Password**: `postgres`
3. Нажми **Test Connection** → Connect

### Просмотр таблиц и данных через psql

```sql
-- Список всех таблиц
\dt

-- Структура таблицы
\d reviews
\d parsed_words
\d scrape_jobs

-- Посмотреть все отзывы
SELECT id, brand, source_url, title, rating, review_date, scraped_at
FROM reviews
ORDER BY scraped_at DESC
LIMIT 20;

-- Посмотреть отзывы конкретного бренда
SELECT title, text, rating, review_date
FROM reviews
WHERE brand = 'Сбербанк'
ORDER BY scraped_at DESC;

-- Топ слов по бренду
SELECT word, SUM(frequency) as total
FROM parsed_words
WHERE brand = 'Сбербанк'
GROUP BY word
ORDER BY total DESC
LIMIT 20;

-- История задач парсинга
SELECT brand, url, status, created_at, finished_at
FROM scrape_jobs
ORDER BY created_at DESC;

-- Количество отзывов по брендам
SELECT brand, COUNT(*) as reviews_count
FROM reviews
GROUP BY brand
ORDER BY reviews_count DESC;
```

---

## API — Routes

### Normalizer Service (порт 8081)

#### `POST /normalize`
Нормализует текст и возвращает частоту слов.

**Запрос:**
```json
{
  "text": "Сбербанк хороший банк. Банк работает хорошо."
}
```
**Ответ:**
```json
{
  "words": ["сбербанк", "хороший", "банк", "работать"],
  "frequencies": [
    { "word": "банк", "frequency": 2 },
    { "word": "хороший", "frequency": 1 }
  ]
}
```

---

#### `POST /cooccurrence`
Возвращает слова, встречающиеся рядом с целевым словом.

**Запрос:**
```json
{
  "text": "сбербанк хороший банк работает быстро",
  "target": "банк",
  "window": 2
}
```
**Ответ:**
```json
{
  "target": "банк",
  "neighbors": {
    "сбербанк": 1,
    "хороший": 1,
    "работает": 1
  }
}
```

---

#### `GET /health`
```json
{ "status": "ok" }
```

---

### Scraper Service (порт 8082)

#### `POST /scrape`
Парсит страницу и возвращает **целые отзывы** прямо в ответе (без сохранения в БД).
Для сохранения в БД используй `POST /schedule` — воркер сделает это автоматически.

**Запрос:**
```json
{
  "brand": "Сбербанк",
  "url": "https://irecommend.ru/content/sberbank",
  "source_type": "js"
}
```

Допустимые значения `source_type`:
| Значение | Описание |
|---|---|
| `html` | Статичные HTML-страницы |
| `api` | JSON API |
| `js` | Динамические страницы (Chrome headless) |

**Ответ:**
```json
{
  "job_id": "uuid",
  "brand": "Сбербанк",
  "source": "irecommend.ru/content/sberbank",
  "url": "https://irecommend.ru/content/sberbank",
  "scraped_at": "2024-01-01T00:00:00Z",
  "reviews": [
    {
      "title": "Отличный банк",
      "text": "Пользуюсь много лет, всё устраивает",
      "rating": "5",
      "date": "2024-01-01",
      "pros": "",
      "cons": ""
    }
  ]
}
```

---

#### `GET /health`
```json
{ "status": "ok" }
```

---

### Storage Service (порт 8083)

#### `POST /jobs`
Создаёт задачу парсинга.

**Запрос:**
```json
{
  "brand": "Сбербанк",
  "url": "https://irecommend.ru/content/sberbank",
  "source_type": "js"
}
```
**Ответ `201`:**
```json
{
  "id": "uuid",
  "brand": "Сбербанк",
  "url": "https://irecommend.ru/content/sberbank",
  "source_type": "js",
  "status": "pending",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

#### `GET /jobs/:id`
Возвращает задачу по ID.

```
GET /jobs/550e8400-e29b-41d4-a716-446655440000
```
**Ответ `200`:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "brand": "Сбербанк",
  "status": "done",
  "finished_at": "2024-01-01T00:01:00Z"
}
```

---

#### `POST /words`
Сохраняет слова с частотами для бренда.

**Запрос:**
```json
{
  "brand": "Сбербанк",
  "source_url": "https://irecommend.ru/content/sberbank",
  "words": [
    { "word": "банк",  "frequency": 42 },
    { "word": "карта", "frequency": 18 }
  ]
}
```
**Ответ `201`:**
```json
{ "saved": 2 }
```

---

#### `GET /words?brand=...&limit=...`
Топ слов по бренду из таблицы `parsed_words`.

```
GET /words?brand=Сбербанк&limit=10
```
**Ответ `200`:**
```json
{
  "brand": "Сбербанк",
  "words": [
    { "word": "банк",  "frequency": 42 },
    { "word": "карта", "frequency": 18 }
  ]
}
```

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `brand` | string | обязательный | Название бренда |
| `limit` | int | 20 | Количество слов |

---

#### `POST /reviews`
Сохраняет список целых отзывов в таблицу `reviews`.

**Запрос:**
```json
{
  "brand": "Сбербанк",
  "source_url": "https://irecommend.ru/content/sberbank",
  "job_id": "uuid (опционально)",
  "reviews": [
    {
      "title": "Отличный банк",
      "text": "Пользуюсь много лет, всё устраивает",
      "rating": "5",
      "review_date": "2024-01-01",
      "pros": "Быстро работает",
      "cons": "Очереди в отделениях"
    }
  ]
}
```
**Ответ `201`:**
```json
{ "saved": 1, "brand_id": 3 }
```

> `brand_id` — ID бренда, автоматически созданного (или найденного) в таблице `brands`.

---

#### `GET /brands`
Возвращает список всех брендов.

**Ответ `200`:**
```json
{
  "brands": [
    { "id": 1, "name": "Сбербанк", "created_at": "2024-01-01T00:00:00Z" },
    { "id": 2, "name": "Тинькофф", "created_at": "2024-01-02T00:00:00Z" }
  ]
}
```

---

#### `GET /reviews?brand=...&limit=...`
Возвращает **целые отзывы** из таблицы `reviews`.

```
GET /reviews?brand=Сбербанк&limit=50
```
**Ответ `200`:**
```json
{
  "brand": "Сбербанк",
  "total": 2,
  "reviews": [
    {
      "id": 1,
      "brand_id": 3,
      "brand": "Сбербанк",
      "source_url": "https://irecommend.ru/content/sberbank",
      "title": "Отличный банк",
      "text": "Пользуюсь много лет, всё устраивает",
      "rating": "5",
      "review_date": "2024-01-01",
      "pros": "Быстро работает",
      "cons": "Очереди в отделениях",
      "scraped_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

| Параметр | Тип | По умолчанию | Описание |
|---|---|---|---|
| `brand` | string | обязательный | Название бренда |
| `limit` | int | 50 | Количество отзывов |

---

#### `GET /health`
```json
{ "status": "ok" }
```

---

### Scheduler Service (порт 8084)

#### `POST /schedule`
Ставит задачу парсинга в очередь Redis. Воркер автоматически заберёт задачу,
вызовет Scraper и сохранит результат в Storage.

**Запрос:**
```json
{
  "brand": "Сбербанк",
  "url": "https://irecommend.ru/content/sberbank",
  "source_type": "js"
}
```
**Ответ `202`:**
```json
{
  "status": "queued",
  "brand": "Сбербанк",
  "url": "https://irecommend.ru/content/sberbank"
}
```

---

#### `GET /health`
```json
{ "status": "ok" }
```

---

## Пример полного сценария использования

```bash
# 1. Поставить задачу парсинга в очередь
curl -X POST http://localhost:8084/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "brand": "Сбербанк",
    "url": "https://irecommend.ru/content/sberbank",
    "source_type": "js"
  }'
# Ответ: {"status":"queued",...}
# Worker автоматически: заберёт → спарсит → сохранит в БД

# 2. Подождать завершения парсинга (js-источники занимают 3-5 минут)

# 3. Получить сохранённые целые отзывы
curl "http://localhost:8083/reviews?brand=Сбербанк&limit=20"

# 4. Получить топ слов по бренду
curl "http://localhost:8083/words?brand=Сбербанк&limit=10"

# 5. Нормализовать произвольный текст
curl -X POST http://localhost:8081/normalize \
  -H "Content-Type: application/json" \
  -d '{"text": "Сбербанк отличный банк, работает быстро и удобно"}'

# 6. Или спарсить напрямую без сохранения в БД
curl -X POST http://localhost:8082/scrape \
  -H "Content-Type: application/json" \
  -d '{
    "brand": "Сбербанк",
    "url": "https://www.banki.ru/services/responses/bank/sberbank/",
    "source_type": "js"
  }'
```

---

## База данных

Миграции применяются **автоматически** при старте Storage Service.

### Таблица `brands` — бренды

Справочник брендов. Заполняется автоматически при первом сохранении отзывов.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGSERIAL (PK) | Уникальный идентификатор |
| `name` | TEXT (UNIQUE) | Название бренда |
| `created_at` | TIMESTAMPTZ | Дата первой записи |

```sql
CREATE TABLE brands (
    id         BIGSERIAL   PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

### Таблица `reviews` — целые отзывы

Основная таблица с полными текстами отзывов.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGSERIAL (PK) | Уникальный идентификатор |
| `brand_id` | BIGINT (FK → brands) | ID бренда |
| `job_id` | UUID | Ссылка на задачу парсинга |
| `brand` | TEXT | Название бренда (денормализованное) |
| `source_url` | TEXT | URL источника |
| `title` | TEXT | Заголовок отзыва |
| `text` | TEXT | Полный текст отзыва |
| `rating` | TEXT | Оценка (1–5) |
| `review_date` | TEXT | Дата отзыва на сайте |
| `pros` | TEXT | Достоинства (otzovik.com) |
| `cons` | TEXT | Недостатки (otzovik.com) |
| `scraped_at` | TIMESTAMPTZ | Дата и время парсинга |

```sql
CREATE TABLE reviews (
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
```

---

### Таблица `parsed_words` — частота слов

Нормализованные слова из отзывов для частотного анализа.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGSERIAL (PK) | Уникальный идентификатор |
| `brand` | TEXT | Название бренда |
| `source_url` | TEXT | URL источника |
| `word` | TEXT | Нормализованное слово |
| `frequency` | INT | Частота встречаемости |
| `scraped_at` | TIMESTAMPTZ | Дата парсинга |

```sql
CREATE TABLE parsed_words (
    id          BIGSERIAL   PRIMARY KEY,
    brand       TEXT        NOT NULL,
    source_url  TEXT        NOT NULL,
    word        TEXT        NOT NULL,
    frequency   INT         NOT NULL DEFAULT 1,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

### Таблица `word_cooccurrence` — совместная встречаемость слов

Для семантического анализа — какие слова стоят рядом с целевым.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGSERIAL (PK) | Уникальный идентификатор |
| `brand` | TEXT | Название бренда |
| `target_word` | TEXT | Целевое слово |
| `neighbor` | TEXT | Соседнее слово |
| `weight` | INT | Вес совместной встречаемости |
| `source_url` | TEXT | URL источника |
| `scraped_at` | TIMESTAMPTZ | Дата парсинга |

```sql
CREATE TABLE word_cooccurrence (
    id          BIGSERIAL   PRIMARY KEY,
    brand       TEXT        NOT NULL,
    target_word TEXT        NOT NULL,
    neighbor    TEXT        NOT NULL,
    weight      INT         NOT NULL DEFAULT 1,
    source_url  TEXT        NOT NULL,
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (brand, target_word, neighbor, source_url)
);
```

---

### Таблица `scrape_jobs` — задачи парсинга

История всех задач с их статусами.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | UUID (PK) | Уникальный идентификатор |
| `brand` | TEXT | Название бренда |
| `url` | TEXT | URL для парсинга |
| `source_type` | TEXT | Тип источника: html / api / js |
| `status` | TEXT | pending / running / done / failed |
| `error_msg` | TEXT | Текст ошибки (если failed) |
| `created_at` | TIMESTAMPTZ | Дата создания |
| `updated_at` | TIMESTAMPTZ | Дата обновления |
| `finished_at` | TIMESTAMPTZ | Дата завершения |

```sql
CREATE TABLE scrape_jobs (
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
```

---

### Индексы

```sql
CREATE INDEX idx_scrape_jobs_brand       ON scrape_jobs(brand);
CREATE INDEX idx_scrape_jobs_status      ON scrape_jobs(status);
CREATE INDEX idx_parsed_words_brand      ON parsed_words(brand);
CREATE INDEX idx_parsed_words_brand_word ON parsed_words(brand, word);
CREATE INDEX idx_reviews_brand           ON reviews(brand);
CREATE INDEX idx_reviews_scraped_at      ON reviews(scraped_at DESC);
```

---

## Запуск тестов

```bash
# Normalizer
cd "Normalizer Service с TDD" && go test ./...

# Scraper
cd "Scraper Service с TDD" && go test ./...

# Scheduler
cd "Scheduler Service с TDD" && go test ./...

# Storage (без repository — не требует Docker)
cd "Storage Service с TDD" && go test ./internal/handler/... ./internal/model/...

# Storage полностью (нужен запущенный Docker)
cd "Storage Service с TDD" && go test ./...
```

---

## Поддерживаемые источники

| Сайт | source_type | Описание |
|---|---|---|
| irecommend.ru | `js` | Динамическая загрузка через Chrome |
| banki.ru | `js` | Динамическая загрузка через Chrome |
| otzovik.com | `js` | Динамическая загрузка через Chrome |
| Любой сайт | `html` | Статичный HTML |
| JSON API | `api` | REST API |
