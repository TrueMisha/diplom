package scraper

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"diplom/scraper-service/internal/fetcher"
)

var (
	ErrUnknownSourceType = errors.New("unknown source type")
	ErrEmptyResult       = errors.New("scrape result is empty")
)

// Interface — контракт, которому удовлетворяет *Dispatcher.
// Используется handler-ом для инъекции зависимости.
type Interface interface {
	Scrape(req ScrapeRequest) (*ScrapeResult, error)
}

type SourceType string

const (
	SourceTypeHTML SourceType = "html"
	SourceTypeAPI  SourceType = "api"
	SourceTypeJS   SourceType = "js"
)

type Options struct {
	Selectors []string
}

type ScrapeResult struct {
	Brand      string
	SourceURL  string
	Source     string
	Title      string
	Text       string
	SourceType SourceType
	ScrapedAt  time.Time
}

type ScrapeRequest struct {
	URL        string
	SourceType SourceType
	Brand      string
	Options    Options
}

func (r ScrapeResult) Validate() error {
	if r.SourceURL == "" {
		return errors.New("source URL is required")
	}
	if strings.TrimSpace(r.Text) == "" {
		return errors.New("scraped text is empty")
	}
	return nil
}

var defaultSelectors = []string{
	"p", "h1", "h2", "h3",
	".review", ".comment", ".review-text",
	"article", "section",
}

var excludeSelectors = []string{
	"nav", "footer", "header", "script", "style",
	".ad", ".advertisement", ".menu", ".sidebar",
}

type HTMLScraper struct {
	f fetcher.Fetcher
}

func NewHTMLScraper(f fetcher.Fetcher) *HTMLScraper {
	return &HTMLScraper{f: f}
}

func (s *HTMLScraper) Scrape(rawURL string, opts Options) (*ScrapeResult, error) {
	result, err := s.f.Fetch(rawURL)
	if err != nil {
		return nil, err
	}
	text, err := s.extractText(result.Body, opts)
	if err != nil {
		return nil, err
	}
	title := extractTitle(result.Body)
	return &ScrapeResult{
		SourceURL: rawURL,
		Source:    extractPath(rawURL),
		Title:     title,
		Text:      text,
		ScrapedAt: time.Now().UTC(),
	}, nil
}

func (s *HTMLScraper) extractText(html string, opts Options) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}
	for _, sel := range excludeSelectors {
		doc.Find(sel).Remove()
	}
	selectors := opts.Selectors
	if len(selectors) == 0 {
		selectors = defaultSelectors
	}
	var parts []string
	for _, sel := range selectors {
		doc.Find(sel).Each(func(_ int, node *goquery.Selection) {
			text := strings.TrimSpace(node.Text())
			if text != "" {
				parts = append(parts, text)
			}
		})
	}
	return strings.Join(parts, "\n"), nil
}

type DispatcherConfig struct {
	HTMLFetcher fetcher.Fetcher
	APIFetcher  fetcher.Fetcher
}

type Dispatcher struct {
	cfg DispatcherConfig
}

func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{cfg: cfg}
}

func (d *Dispatcher) Scrape(req ScrapeRequest) (*ScrapeResult, error) {
	var result *ScrapeResult
	var err error

	switch req.SourceType {
	case SourceTypeHTML:
		if d.cfg.HTMLFetcher == nil {
			return nil, errors.New("HTML fetcher not configured")
		}
		s := NewHTMLScraper(d.cfg.HTMLFetcher)
		result, err = s.Scrape(req.URL, req.Options)

	case SourceTypeAPI:
		if d.cfg.APIFetcher == nil {
			return nil, errors.New("API fetcher not configured")
		}
		s := NewHTMLScraper(d.cfg.APIFetcher)
		result, err = s.Scrape(req.URL, req.Options)

	case SourceTypeJS:
		s := NewJSScraper()
		result, err = s.Scrape(req.URL, req.Options)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownSourceType, req.SourceType)
	}

	if err != nil {
		return nil, err
	}

	result.Brand = req.Brand
	result.SourceType = req.SourceType
	return result, nil
}

func extractTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

func extractPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	if u.Path != "" && u.Path != "/" {
		return host + u.Path
	}
	return host
}
