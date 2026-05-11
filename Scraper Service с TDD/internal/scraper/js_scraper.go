package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type JSScraper struct{}

func NewJSScraper() *JSScraper {
	return &JSScraper{}
}

func (s *JSScraper) Scrape(rawURL string, opts Options) (*ScrapeResult, error) {
	switch {
	case strings.Contains(rawURL, "banki.ru"):
		return s.scrapeBankiru(rawURL, opts)
	case strings.Contains(rawURL, "otzovik.com"):
		return s.scrapeOtzovik(rawURL, opts)
	default:
		return s.scrapeIrecommend(rawURL, opts)
	}
}

func (s *JSScraper) scrapeIrecommend(rawURL string, opts Options) (*ScrapeResult, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
			chromedp.Flag("no-sandbox", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		)...,
	)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 300*time.Second)
	defer cancelTimeout()

	var allText strings.Builder
	gdprDone := false

	for page := 0; page <= 7; page++ {
		pageURL := rawURL
		if page > 0 {
			pageURL = fmt.Sprintf("%s?page=%d", rawURL, page)
		}

		if err := chromedp.Run(ctx,
			chromedp.Navigate(pageURL),
			chromedp.Sleep(4*time.Second),
		); err != nil {
			return nil, fmt.Errorf("navigate page %d: %w", page, err)
		}

		if !gdprDone {
			_ = chromedp.Run(ctx,
				chromedp.WaitVisible(`button.fc-cta-consent`, chromedp.ByQuery),
				chromedp.Click(`button.fc-cta-consent`, chromedp.ByQuery),
				chromedp.Sleep(2*time.Second),
			)
			gdprDone = true
		}

		var pageText string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					var list = document.querySelector('ul.list-comments');
					if (!list) return '';

					list.querySelectorAll('li').forEach(function(item) {
						// Рейтинг из класса fivestarWidgetStatic-N
						var fivestar = item.querySelector('[class*="fivestarWidgetStatic-"]');
						var rating = '';
						if (fivestar) {
							var m = fivestar.className.match(/fivestarWidgetStatic-(\d+)/);
							if (m) rating = m[1];
						}

						// Дата из div.created
						var dateEl = item.querySelector('.created');
						var date = dateEl ? dateEl.innerText.trim() : '';

						if (fivestar) {
							var span = document.createElement('span');
							span.textContent = '\n[RATING:' + rating + '][DATE:' + date + ']\n';
							fivestar.replaceWith(span);
							if (dateEl) dateEl.remove();
						}
					});

					return list.innerText;
				})()
			`, &pageText),
		); err != nil {
			return nil, fmt.Errorf("get text page %d: %w", page, err)
		}

		allText.WriteString(pageText)
		allText.WriteString("\n")
	}

	var title string
	_ = chromedp.Run(ctx, chromedp.Title(&title))

	return &ScrapeResult{
		SourceURL: rawURL,
		Source:    extractPath(rawURL),
		Title:     title,
		Text:      allText.String(),
		ScrapedAt: timeNow(),
	}, nil
}

func (s *JSScraper) scrapeBankiru(rawURL string, opts Options) (*ScrapeResult, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
			chromedp.Flag("no-sandbox", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		)...,
	)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 120*time.Second)
	defer cancelTimeout()

	if err := chromedp.Run(ctx,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(5*time.Second),
	); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	for i := 0; i < 5; i++ {
		err := chromedp.Run(ctx,
			chromedp.Click(`[data-test="responses__more-btn"]`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			break
		}
	}

	var reviewsText string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				var container = document.querySelector('[data-test="responses"]');
				if (!container) return '';

				container.querySelectorAll('[data-test="responses__response"]').forEach(function(item) {
					// Рейтинг из атрибута value у Grade__sc
					var gradeEl = item.querySelector('[class*="Grade__sc"]');
					var rating = gradeEl ? (gradeEl.getAttribute('value') || gradeEl.innerText.trim()) : '';

					// Дата — первый StyledItemSmallText содержащий ДД.ММ.ГГГГ
					var date = '';
					var dateEl = null;
					item.querySelectorAll('[class*="StyledItemSmallText"]').forEach(function(el) {
						if (!dateEl && /\d{2}\.\d{2}\.\d{4}/.test(el.innerText)) {
							dateEl = el;
							date = el.innerText.trim();
						}
					});

					if (gradeEl || dateEl) {
						var span = document.createElement('span');
						span.textContent = '\n[RATING:' + rating + '][DATE:' + date + ']\n';
						if (gradeEl) {
							gradeEl.replaceWith(span);
						} else if (dateEl) {
							dateEl.replaceWith(span);
						}
						if (dateEl && gradeEl) dateEl.remove();
					}
				});

				return container.innerText;
			})()
		`, &reviewsText),
	); err != nil {
		return nil, fmt.Errorf("get data: %w", err)
	}

	var title string
	_ = chromedp.Run(ctx, chromedp.Title(&title))

	return &ScrapeResult{
		SourceURL: rawURL,
		Source:    extractPath(rawURL),
		Title:     title,
		Text:      reviewsText,
		ScrapedAt: timeNow(),
	}, nil
}

func (s *JSScraper) scrapeOtzovik(rawURL string, opts Options) (*ScrapeResult, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-gpu", false),
			chromedp.Flag("no-sandbox", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		)...,
	)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 300*time.Second)
	defer cancelTimeout()

	baseURL := strings.TrimRight(rawURL, "/")
	var allText strings.Builder

	for page := 1; page <= 5; page++ {
		pageURL := baseURL + "/"
		if page > 1 {
			pageURL = fmt.Sprintf("%s/%d/", baseURL, page)
		}

		if err := chromedp.Run(ctx,
			chromedp.Navigate(pageURL),
			chromedp.Sleep(3*time.Second),
		); err != nil {
			return nil, fmt.Errorf("navigate page %d: %w", page, err)
		}

		var pageText string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					var list = document.querySelector('.review-list-2');
					if (!list) return '';

					list.querySelectorAll('.rating-wrap').forEach(function(wrap) {
						var scoreEl = wrap.querySelector('.rating-score span');
						var rating = scoreEl ? scoreEl.innerText.trim() : '';

						var dateEl = wrap.querySelector('[itemprop="datePublished"]');
						var date = dateEl ? dateEl.getAttribute('content') : '';
						if (date) date = date.substring(0, 10);

						var span = document.createElement('span');
						span.textContent = '\n[RATING:' + rating + '][DATE:' + date + ']\n';
						wrap.replaceWith(span);
					});

					return list.innerText;
				})()
			`, &pageText),
		); err != nil {
			return nil, fmt.Errorf("get text page %d: %w", page, err)
		}

		allText.WriteString(pageText)
		allText.WriteString("\n")
	}

	var title string
	_ = chromedp.Run(ctx, chromedp.Title(&title))

	return &ScrapeResult{
		SourceURL: rawURL,
		Source:    extractPath(rawURL),
		Title:     title,
		Text:      allText.String(),
		ScrapedAt: timeNow(),
	}, nil
}

func timeNow() time.Time {
	return time.Now().UTC()
}
