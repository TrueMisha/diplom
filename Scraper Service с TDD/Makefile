.PHONY: test test-cover test-fetcher test-scraper test-antiblock test-handler run build

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html"

test-fetcher:
	go test ./internal/fetcher/... -v

test-scraper:
	go test ./internal/scraper/... -v

test-antiblock:
	go test ./internal/antiblock/... -v

test-handler:
	go test ./internal/handler/... -v

## TDD watch mode (нужен gotestsum)
test-watch:
	gotestsum --watch ./...

run:
	PORT=8082 go run ./cmd/server

build:
	go build -o bin/scraper-service ./cmd/server

docker-build:
	docker build -t scraper-service:latest .
