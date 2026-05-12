package main

import (
	"log"
	"os"

	"github.com/hibiken/asynq"

	"diplom/scheduler-service/internal/tasks"
	"diplom/scheduler-service/internal/worker"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	redisAddr   := getEnv("REDIS_ADDR",   "localhost:6379")
	scraperURL  := getEnv("SCRAPER_URL",  "http://localhost:8082")
	storageURL  := getEnv("STORAGE_URL",  "http://localhost:8083")

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 5,
			Queues:      map[string]int{"default": 10},
		},
	)

	mux := asynq.NewServeMux()

	w := worker.New(scraperURL, storageURL, nil)
	mux.HandleFunc(tasks.TypeScrape, w.ProcessTask)

	log.Printf("scheduler-worker started (redis=%s scraper=%s storage=%s)",
		redisAddr, scraperURL, storageURL)
	if err := srv.Run(mux); err != nil {
		log.Fatal(err)
	}
}
