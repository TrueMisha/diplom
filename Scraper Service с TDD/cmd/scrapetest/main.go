package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("no-sandbox", true),
			chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		)...,
	)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://irecommend.ru/content/sberbank"),
		chromedp.Sleep(5*time.Second),
	); err != nil {
		log.Fatal(err)
	}

	// Закрываем GDPR
	_ = chromedp.Run(ctx,
		chromedp.WaitVisible(`button.fc-cta-consent`, chromedp.ByQuery),
		chromedp.Click(`button.fc-cta-consent`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	var result string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				var list = document.querySelector('ul.list-comments');
				if (!list) return 'LIST NOT FOUND';

				var li = list.querySelector('li');
				if (!li) return 'LI NOT FOUND';

				// Все элементы с атрибутом value
				var withValue = li.querySelectorAll('[value]');
				var out = 'Elements with [value]: ' + withValue.length + '\n';
				withValue.forEach(function(el) {
					out += 'TAG=' + el.tagName + ' CLASS=' + el.className + ' VALUE=' + el.getAttribute('value') + '\n';
				});

				// Ищем fivestar
				var fivestar = li.querySelectorAll('[class*="fivestar"]');
				out += 'Fivestar elements: ' + fivestar.length + '\n';
				fivestar.forEach(function(el) {
					out += 'CLASS=' + el.className + '\n';
				});

				// .created
				var created = li.querySelector('.created');
				out += 'CREATED: ' + (created ? created.innerText.trim() : 'NOT FOUND') + '\n';

				// Первые 500 символов первого li
				out += '\nFIRST LI TEXT:\n' + li.innerText.substring(0, 500);

				return out;
			})()
		`, &result),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
