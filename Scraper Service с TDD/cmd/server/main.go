package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"diplom/scraper-service/internal/antiblock"
	"diplom/scraper-service/internal/fetcher"
	"diplom/scraper-service/internal/handler"
	"diplom/scraper-service/internal/scraper"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := getEnv("PORT", "8082")

	uaRotator := antiblock.NewUserAgentRotator(nil)
	baseFetcher := fetcher.NewHTTPFetcher(fetcher.Config{
		Timeout:   15 * time.Second,
		UserAgent: uaRotator.Next(),
	})
	retryFetcher := fetcher.NewRetryFetcher(baseFetcher, fetcher.RetryConfig{
		MaxAttempts: 3,
		Delay:       2 * time.Second,
	})

	dispatcher := scraper.NewDispatcher(scraper.DispatcherConfig{
		HTMLFetcher: retryFetcher,
		APIFetcher:  retryFetcher,
	})

	h := handler.New(handler.Config{
		Mode:       handler.ModeProduction,
		Dispatcher: dispatcher,
		Timeout:    10 * time.Minute,
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("scraper-service listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
