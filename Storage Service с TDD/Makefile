.PHONY: test test-unit test-integration test-cover run build

## Все тесты (нужен Docker для testcontainers)
test:
	go test ./... -v -count=1

## Только unit-тесты (без Docker — быстро)
test-unit:
	go test ./internal/model/... ./internal/handler/... -v -count=1

## Только интеграционные тесты репозитория (нужен Docker)
test-integration:
	go test ./internal/repository/... -v -count=1

## TDD watch
test-watch:
	gotestsum --watch ./...

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

run:
	PORT=8083 DATABASE_URL="postgres://user:pass@localhost:5432/brandmon?sslmode=disable" \
	go run ./cmd/server

build:
	go build -o bin/storage-service ./cmd/server

## Применить миграции вручную
migrate-up:
	psql $$DATABASE_URL -f migrations/001_init.up.sql

migrate-down:
	psql $$DATABASE_URL -f migrations/001_init.down.sql
