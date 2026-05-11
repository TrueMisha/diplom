package main

import (
	"fmt"
	"log"
	"strings"

	"diplom/scraper-service/internal/scraper"
)

func main() {
	s := scraper.NewJSScraper()
	result, err := s.Scrape("https://otzovik.com/reviews/sberbank_rossii/", scraper.Options{})
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(result.Text, "\n")
	fmt.Printf("Строк: %d\n", len(lines))
	for i, l := range lines {
		if i >= 15 {
			break
		}
		l = strings.TrimSpace(l)
		if l != "" {
			fmt.Printf("[%d] %s\n", i+1, l)
		}
	}
}
