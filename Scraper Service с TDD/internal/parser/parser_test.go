package parser_test

import (
	"testing"

	"diplom/scraper-service/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ParseIrecommend ──────────────────────────────────────────────────────────

func TestParseIrecommend_Basic(t *testing.T) {
	text := `
[RATING:5][DATE:20.12.2025]
Отличный банк с хорошим обслуживанием
Долго пользуюсь и очень доволен качеством работы всех сервисов банка

[RATING:2][DATE:15.01.2026]
Ужасный опыт посещения отделения
`
	reviews := parser.ParseIrecommend(text)
	require.Len(t, reviews, 2)

	assert.Equal(t, "Отличный банк с хорошим обслуживанием", reviews[0].Title)
	assert.Equal(t, "5", reviews[0].Rating)
	assert.Equal(t, "20.12.2025", reviews[0].Date)
	assert.NotEmpty(t, reviews[0].Text)

	assert.Equal(t, "2", reviews[1].Rating)
	assert.Equal(t, "15.01.2026", reviews[1].Date)
}

func TestParseIrecommend_Empty(t *testing.T) {
	reviews := parser.ParseIrecommend("")
	assert.NotNil(t, reviews)
	assert.Empty(t, reviews)
}

func TestParseIrecommend_NoMarkers(t *testing.T) {
	reviews := parser.ParseIrecommend("просто текст без маркеров рейтинга")
	assert.NotNil(t, reviews)
	assert.Empty(t, reviews)
}

func TestParseIrecommend_TableDriven(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		wantCount   int
		wantTitle0  string
		wantText0   string
		wantRating0 string
		wantDate0   string
	}{
		{
			name: "marker with title and long text",
			text: "[RATING:5][DATE:20.12.2025]\nОтличный банк с хорошим обслуживанием\nДолго пользуюсь и очень доволен качеством работы всех сервисов банка\n",
			wantCount:   1,
			wantTitle0:  "Отличный банк с хорошим обслуживанием",
			wantText0:   "Долго пользуюсь и очень доволен качеством работы всех сервисов банка",
			wantRating0: "5",
			wantDate0:   "20.12.2025",
		},
		{
			name: "marker with title only no long text",
			text: "[RATING:3][DATE:01.01.2026]\nЗаголовок отзыва достаточно длинный\n",
			wantCount:   1,
			wantTitle0:  "Заголовок отзыва достаточно длинный",
			wantText0:   "",
			wantRating0: "3",
			wantDate0:   "01.01.2026",
		},
		{
			name:      "empty input returns empty slice",
			text:      "",
			wantCount: 0,
		},
		{
			name:      "no markers returns empty slice",
			text:      "Просто обычный текст без каких-либо маркеров рейтинга здесь",
			wantCount: 0,
		},
		{
			name:      "short text after marker skipped as title",
			text:      "[RATING:5][DATE:01.01.2026]\nКорот\n",
			wantCount: 0,
		},
		{
			name:      "numbers-only line not used as title",
			text:      "[RATING:5][DATE:01.01.2026]\n12345678901234567890123\n",
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reviews := parser.ParseIrecommend(tc.text)
			require.NotNil(t, reviews)
			require.Len(t, reviews, tc.wantCount)
			if tc.wantCount > 0 {
				assert.Equal(t, tc.wantTitle0, reviews[0].Title)
				assert.Equal(t, tc.wantText0, reviews[0].Text)
				assert.Equal(t, tc.wantRating0, reviews[0].Rating)
				assert.Equal(t, tc.wantDate0, reviews[0].Date)
			}
		})
	}
}

func TestParseIrecommend_MultipleReviews(t *testing.T) {
	text := `
[RATING:5][DATE:20.12.2025]
Отличный банк с хорошим обслуживанием
Долго пользуюсь и очень доволен качеством работы всех сервисов банка

[RATING:2][DATE:15.01.2026]
Ужасный опыт посещения отделения

[RATING:4][DATE:10.02.2026]
Хорошее отделение рядом с домом и вежливый персонал
`
	reviews := parser.ParseIrecommend(text)
	require.Len(t, reviews, 3)
	assert.Equal(t, "5", reviews[0].Rating)
	assert.Equal(t, "2", reviews[1].Rating)
	assert.Equal(t, "4", reviews[2].Rating)
}

// ─── ParseBankiru ─────────────────────────────────────────────────────────────

func TestParseBankiru_Basic(t *testing.T) {
	text := `
Хороший банк для повседневных задач
[RATING:4][DATE:10.02.2026]
Очень удобное мобильное приложение и поддержка работает круглосуточно

Плохой сервис в отделении
[RATING:1][DATE:05.03.2026]
Долго ждал своей очереди без каких-либо объяснений со стороны персонала
`
	reviews := parser.ParseBankiru(text)
	require.Len(t, reviews, 2)

	assert.Equal(t, "Хороший банк для повседневных задач", reviews[0].Title)
	assert.Equal(t, "4", reviews[0].Rating)
	assert.Equal(t, "10.02.2026", reviews[0].Date)

	assert.Equal(t, "Плохой сервис в отделении", reviews[1].Title)
	assert.Equal(t, "1", reviews[1].Rating)
}

func TestParseBankiru_Empty(t *testing.T) {
	reviews := parser.ParseBankiru("")
	assert.NotNil(t, reviews)
	assert.Empty(t, reviews)
}

func TestParseBankiru_TableDriven(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantCount  int
		wantTitle0 string
	}{
		{
			name:       "title before marker text after marker",
			text:       "Хороший банк для повседневных задач\n[RATING:4][DATE:10.02.2026]\nОчень удобное мобильное приложение и поддержка работает круглосуточно\n",
			wantCount:  1,
			wantTitle0: "Хороший банк для повседневных задач",
		},
		{
			name:      "skip word: Ответ банка",
			text:      "Ответ банка\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Отзыв проверен",
			text:      "Отзыв проверен\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Документы прикреплены",
			text:      "Документы прикреплены\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Зачтено",
			text:      "Зачтено\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Проблема решена",
			text:      "Проблема решена\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Оценка:",
			text:      "Оценка:\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "skip word: Показать еще",
			text:      "Показать еще\n[RATING:4][DATE:10.02.2026]\nТекст отзыва достаточно длинный чтобы пройти проверку длины строки\n",
			wantCount: 0,
		},
		{
			name:      "empty input",
			text:      "",
			wantCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reviews := parser.ParseBankiru(tc.text)
			require.NotNil(t, reviews)
			require.Len(t, reviews, tc.wantCount)
			if tc.wantCount > 0 {
				assert.Equal(t, tc.wantTitle0, reviews[0].Title)
			}
		})
	}
}

func TestParseBankiru_MultipleReviews(t *testing.T) {
	text := `
Хорошее обслуживание клиентов в отделении
[RATING:4][DATE:01.03.2026]
Быстро оформили карту и объяснили все условия обслуживания

Проблема с переводами за границу
[RATING:2][DATE:15.03.2026]
Несколько раз возникали задержки при международных переводах без объяснений
`
	reviews := parser.ParseBankiru(text)
	require.Len(t, reviews, 2)
	assert.Equal(t, "4", reviews[0].Rating)
	assert.Equal(t, "2", reviews[1].Rating)
}

// ─── ParseOtzovik ─────────────────────────────────────────────────────────────

func TestParseOtzovik_Basic(t *testing.T) {
	text := `
[RATING:5][DATE:2025-12-01]
Надёжный банк с хорошей репутацией
Достоинства: Быстрое обслуживание и вежливый персонал
Недостатки: Высокая комиссия за снятие наличных в банкоматах

[RATING:3][DATE:2026-01-20]
Обычный банк ничего особенного
`
	reviews := parser.ParseOtzovik(text)
	require.Len(t, reviews, 2)

	assert.Equal(t, "Надёжный банк с хорошей репутацией", reviews[0].Title)
	assert.Equal(t, "5", reviews[0].Rating)
	assert.Equal(t, "Быстрое обслуживание и вежливый персонал", reviews[0].Pros)
	assert.Equal(t, "Высокая комиссия за снятие наличных в банкоматах", reviews[0].Cons)
}

func TestParseOtzovik_Empty(t *testing.T) {
	reviews := parser.ParseOtzovik("")
	assert.NotNil(t, reviews)
	assert.Empty(t, reviews)
}

func TestParseOtzovik_TableDriven(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantCount  int
		wantTitle0 string
		wantPros0  string
		wantCons0  string
		wantText0  string
	}{
		{
			name:       "marker then title then pros and cons",
			text:       "[RATING:5][DATE:2025-12-01]\nНадёжный банк\nДостоинства: Быстрое обслуживание и вежливый персонал\nНедостатки: Высокая комиссия\n",
			wantCount:  1,
			wantTitle0: "Надёжный банк",
			wantPros0:  "Быстрое обслуживание и вежливый персонал",
			wantCons0:  "Высокая комиссия",
		},
		{
			name:      "long line becomes text field",
			text:      "[RATING:4][DATE:2026-01-01]\nЗаголовок\nДостоинства: Плюсы\nЭтот текст достаточно длинный чтобы стать полем текста отзыва в системе\n",
			wantCount: 1,
			wantTitle0: "Заголовок",
			wantText0:  "Этот текст достаточно длинный чтобы стать полем текста отзыва в системе",
			wantPros0:  "Плюсы",
		},
		{
			name:      "empty input",
			text:      "",
			wantCount: 0,
		},
		{
			name:      "no markers",
			text:      "Просто текст без маркеров в этой строке совсем",
			wantCount: 0,
		},
		{
			name:       "multiple reviews",
			text:       "[RATING:5][DATE:2025-12-01]\nНадёжный банк\n\n[RATING:3][DATE:2026-01-20]\nОбычный банк ничего\n",
			wantCount:  2,
			wantTitle0: "Надёжный банк",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reviews := parser.ParseOtzovik(tc.text)
			require.NotNil(t, reviews)
			require.Len(t, reviews, tc.wantCount)
			if tc.wantCount > 0 {
				assert.Equal(t, tc.wantTitle0, reviews[0].Title)
				assert.Equal(t, tc.wantPros0, reviews[0].Pros)
				assert.Equal(t, tc.wantCons0, reviews[0].Cons)
				if tc.wantText0 != "" {
					assert.Equal(t, tc.wantText0, reviews[0].Text)
				}
			}
		})
	}
}

// ─── ParseByURL ───────────────────────────────────────────────────────────────

func TestParseByURL_RoutesCorrectly(t *testing.T) {
	marker := "[RATING:5][DATE:01.01.2026]\nЗаголовок отзыва достаточно длинный чтобы пройти проверку\n"

	cases := []struct {
		url      string
		wantFunc string
	}{
		{"https://banki.ru/services/banks/sberbank", "bankiru"},
		{"https://otzovik.com/reviews/sberbank/", "otzovik"},
		{"https://irecommend.ru/content/sberbank", "irecommend"},
		{"https://example.com/reviews", "irecommend"},
	}

	for _, tc := range cases {
		t.Run(tc.wantFunc, func(t *testing.T) {
			reviews := parser.ParseByURL(tc.url, marker)
			assert.NotNil(t, reviews)
		})
	}
}

func TestParseByURL_BankiruURL(t *testing.T) {
	text := "Хорошее обслуживание клиентов\n[RATING:4][DATE:01.03.2026]\nБыстро оформили карту и объяснили все условия обслуживания\n"
	reviews := parser.ParseByURL("https://banki.ru/services/banks/sberbank", text)
	require.NotNil(t, reviews)
	require.Len(t, reviews, 1)
	assert.Equal(t, "Хорошее обслуживание клиентов", reviews[0].Title)
	assert.Equal(t, "4", reviews[0].Rating)
}

func TestParseByURL_OtzovikURL(t *testing.T) {
	text := "[RATING:5][DATE:2025-12-01]\nНадёжный банк\nДостоинства: Быстрое обслуживание и вежливый персонал\n"
	reviews := parser.ParseByURL("https://otzovik.com/reviews/sberbank/", text)
	require.NotNil(t, reviews)
	require.Len(t, reviews, 1)
	assert.Equal(t, "Надёжный банк", reviews[0].Title)
	assert.Equal(t, "Быстрое обслуживание и вежливый персонал", reviews[0].Pros)
}

func TestParseByURL_IrecommendURL(t *testing.T) {
	text := "[RATING:5][DATE:20.12.2025]\nОтличный банк с хорошим обслуживанием\nДолго пользуюсь и очень доволен качеством работы всех сервисов банка\n"
	reviews := parser.ParseByURL("https://irecommend.ru/content/sberbank", text)
	require.NotNil(t, reviews)
	require.Len(t, reviews, 1)
	assert.Equal(t, "Отличный банк с хорошим обслуживанием", reviews[0].Title)
	assert.Equal(t, "5", reviews[0].Rating)
}

func TestParseByURL_UnknownURLUsesIrecommend(t *testing.T) {
	text := "[RATING:3][DATE:01.06.2026]\nОбычный отзыв на неизвестном сайте про сервис\n"
	reviews := parser.ParseByURL("https://example.com/reviews", text)
	require.NotNil(t, reviews)
	require.Len(t, reviews, 1)
	assert.Equal(t, "3", reviews[0].Rating)
}

// ─── Return type: always slice, never nil ─────────────────────────────────────

func TestParsers_NeverReturnNil(t *testing.T) {
	assert.NotNil(t, parser.ParseIrecommend(""))
	assert.NotNil(t, parser.ParseBankiru(""))
	assert.NotNil(t, parser.ParseOtzovik(""))
	assert.NotNil(t, parser.ParseByURL("https://example.com", ""))
}
