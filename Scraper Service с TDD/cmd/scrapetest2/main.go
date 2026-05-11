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
		chromedp.Navigate("https://www.banki.ru/services/responses/bank/sberbank/"),
		chromedp.Sleep(5*time.Second),
	); err != nil {
		log.Fatal(err)
	}

	var result string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				var item = document.querySelector('[data-test="responses__response"]');
				if (!item) return 'RESPONSE NOT FOUND';

				var out = '';

				// Grade элементы
				var grades = item.querySelectorAll('[class*="Grade__sc"]');
				out += 'Grade elements: ' + grades.length + '\n';
				grades.forEach(function(el) {
					out += 'TAG=' + el.tagName + ' CLASS=' + el.className + ' VALUE=' + el.getAttribute('value') + ' TEXT=' + el.innerText.trim() + '\n';
				});

				// StyledItemSmallText
				var texts = item.querySelectorAll('[class*="StyledItemSmallText"]');
				out += 'SmallText elements: ' + texts.length + '\n';
				texts.forEach(function(el) {
					out += 'TEXT=' + el.innerText.trim() + '\n';
				});

				out += '\nFIRST ITEM TEXT:\n' + item.innerText.substring(0, 500);

				return out;
			})()
		`, &result),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}
