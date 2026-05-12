package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"diplom/scraper-service/internal/parser"
	"diplom/scraper-service/internal/scraper"
)

func main() {
	targets := []struct {
		brand string
		url   string
	}{
		{"Сбербанк", "https://irecommend.ru/content/sberbank"},
		{"Сбербанк", "https://www.banki.ru/services/responses/bank/sberbank/"},
		{"Сбербанк", "https://otzovik.com/reviews/sberbank_rossii/"},
	}

	results := make([]parser.Result, len(targets))
	var wg sync.WaitGroup

	for idx, target := range targets {
		wg.Add(1)
		go func(idx int, brand, url string) {
			defer wg.Done()

			fmt.Printf("[pipeline] старт: %s\n", url)

			s := scraper.NewJSScraper()
			raw, err := s.Scrape(url, scraper.Options{})
			if err != nil {
				log.Printf("[pipeline] ошибка %s: %v\n", url, err)
				return
			}

			reviews := parser.ParseByURL(url, raw.Text)
			fmt.Printf("[pipeline] готово %s — найдено отзывов: %d\n", url, len(reviews))

			results[idx] = parser.Result{
				Brand:     brand,
				Source:    raw.Source,
				URL:       raw.SourceURL,
				ScrapedAt: raw.ScrapedAt,
				Reviews:   reviews,
			}
		}(idx, target.brand, target.url)
	}

	wg.Wait()

	var final []parser.Result
	for _, r := range results {
		if r.Source != "" {
			final = append(final, r)
		}
	}

	out := getEnv("OUTPUT", "reviews.json")
	f, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(final); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Сохранено %d источников в %s\n", len(final), out)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
