.PHONY: test test-watch test-cover lint run build

## TDD: запуск тестов (RED → GREEN цикл)
test:
	go test ./... -v -count=1

## TDD: автоперезапуск тестов при изменении файлов (нужен: go install github.com/gotestsum/gotestsum@latest)
test-watch:
	gotestsum --watch ./...

## Покрытие кода тестами
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Только тесты normalizer
test-normalizer:
	go test ./internal/normalizer/... -v

## Только тесты handler
test-handler:
	go test ./internal/handler/... -v

## Только тесты middleware
test-middleware:
	go test ./internal/middleware/... -v

## Линтер
lint:
	golangci-lint run ./...

## Запуск сервера локально
run:
	PORT=8081 go run ./cmd/server

## Сборка бинаря
build:
	go build -o bin/normalizer-service ./cmd/server

## Docker
docker-build:
	docker build -t normalizer-service:latest .

docker-run:
	docker run -p 8081:8081 normalizer-service:latest
