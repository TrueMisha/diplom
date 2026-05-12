package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/brandmon/normalizer-service/internal/handler"
	"github.com/brandmon/normalizer-service/internal/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	h := handler.New()

	srv := middleware.Chain(
		h,
		middleware.Recovery,
		middleware.RequestID,
		middleware.JSONContentType,
	)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("normalizer-service listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
