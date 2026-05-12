package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hibiken/asynq"

	"diplom/scheduler-service/internal/handler"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port      := getEnv("PORT", "8084")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()

	h := handler.New(client)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("scheduler-service listening on %s (redis=%s)", addr, redisAddr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
