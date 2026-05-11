// Package parser разбирает сырой текст, полученный скрапером,
// на структурированные отзывы.
//
// Поддерживаемые источники определяются по URL:
//   - irecommend.ru
//   - banki.ru
//   - otzovik.com
package parser

import (
	"regexp"
	"strings"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// Review — один отзыв.
type Review struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Rating string `json:"rating,omitempty"`
	Date   string `json:"date,omitempty"`
	Pros   string `json:"pros,omitempty"`
	Cons   string `json:"cons,omitempty"`
}

// Result — результат парсинга одного источника.
type Result struct {
	Brand     string    `json:"brand"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	ScrapedAt time.Time `json:"scraped_at"`
	Reviews   []Review  `json:"reviews"`
}

// ─── Регулярные выражения ─────────────────────────────────────────────────────

var (
	reMarker = regexp.MustCompile(`\[RATING:([^\]]*)\]\[DATE:([^\]]*)\]`)
	reNumber = regexp.MustCompile(`^\d+$`)
)

func isNumber(s string) bool { return reNumber.MatchString(s) }

// ─── Public API ───────────────────────────────────────────────────────────────

// ParseByURL выбирает нужный парсер по URL и возвращает список отзывов.
func ParseByURL(rawURL, text string) []Review {
	switch {
	case strings.Contains(rawURL, "banki.ru"):
		return ParseBankiru(text)
	case strings.Contains(rawURL, "otzovik.com"):
		return ParseOtzovik(text)
	default:
		return ParseIrecommend(text)
	}
}

// ─── Парсеры по источникам ────────────────────────────────────────────────────

// ParseIrecommend разбирает текст с irecommend.ru.
func ParseIrecommend(text string) []Review {
	reviews := make([]Review, 0)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		m := reMarker.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		rating, date := m[1], m[2]
		title, reviewText := "", ""

		for j := i + 1; j < len(lines) && j < i+8; j++ {
			next := strings.TrimSpace(lines[j])
			if reMarker.MatchString(next) {
				break
			}
			if len([]rune(next)) > 20 && !isNumber(next) {
				if title == "" {
					title = next
				} else if len([]rune(next)) > 60 {
					reviewText = next
					break
				}
			}
		}

		if title != "" {
			reviews = append(reviews, Review{
				Title: title, Text: reviewText,
				Rating: rating, Date: date,
			})
		}
	}
	return reviews
}

// ParseBankiru разбирает текст с banki.ru.
func ParseBankiru(text string) []Review {
	reviews := make([]Review, 0)
	lines := strings.Split(text, "\n")

	skip := map[string]bool{
		"Ответ банка": true, "Отзыв проверен": true,
		"Документы прикреплены": true, "Зачтено": true,
		"Проблема решена": true, "Оценка:": true,
		"Показать еще": true,
	}

	for i, line := range lines {
		m := reMarker.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		rating, date := m[1], m[2]
		title, reviewText := "", ""

		for j := i - 1; j >= 0 && j >= i-6; j-- {
			prev := strings.TrimSpace(lines[j])
			if len([]rune(prev)) > 5 && !reMarker.MatchString(prev) &&
				!isNumber(prev) && !skip[prev] {
				title = prev
				break
			}
		}

		for j := i + 1; j < len(lines) && j < i+8; j++ {
			next := strings.TrimSpace(lines[j])
			if reMarker.MatchString(next) {
				break
			}
			if len([]rune(next)) > 60 && !skip[next] {
				reviewText = next
				break
			}
		}

		if title != "" && !skip[title] {
			reviews = append(reviews, Review{
				Title: title, Text: reviewText,
				Rating: rating, Date: date,
			})
		}
	}
	return reviews
}

// ParseOtzovik разбирает текст с otzovik.com.
func ParseOtzovik(text string) []Review {
	reviews := make([]Review, 0)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		m := reMarker.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		rating, date := m[1], m[2]
		title, reviewText, pros, cons := "", "", "", ""

		for j := i + 1; j < len(lines) && j < i+5; j++ {
			next := strings.TrimSpace(lines[j])
			if len([]rune(next)) > 5 && !reMarker.MatchString(next) && !isNumber(next) {
				title = next
				break
			}
		}

		for j := i + 1; j < len(lines) && j < i+15; j++ {
			next := strings.TrimSpace(lines[j])
			if reMarker.MatchString(next) {
				break
			}
			switch {
			case strings.HasPrefix(next, "Достоинства:"):
				pros = strings.TrimSpace(strings.TrimPrefix(next, "Достоинства:"))
			case strings.HasPrefix(next, "Недостатки:"):
				cons = strings.TrimSpace(strings.TrimPrefix(next, "Недостатки:"))
			case len([]rune(next)) > 60 && reviewText == "" && next != title:
				reviewText = next
			}
		}

		if title != "" {
			reviews = append(reviews, Review{
				Title: title, Text: reviewText,
				Rating: rating, Date: date,
				Pros: pros, Cons: cons,
			})
		}
	}
	return reviews
}
