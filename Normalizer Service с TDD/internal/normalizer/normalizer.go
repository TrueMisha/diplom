package normalizer

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ─── Types ───────────────────────────────────────────────────────────────────

type WordFreq struct {
	Word      string `json:"word"`
	Frequency int    `json:"frequency"`
}

type NormalizeResult struct {
	Words        []string       `json:"words"`
	Frequencies  []WordFreq     `json:"frequencies"`
	Cooccurrence map[string]int `json:"cooccurrence,omitempty"`
}

type Config struct {
	MinWordLength int
	WindowSize    int // для co-occurrence
	StopWords     map[string]bool
}

func DefaultConfig() Config {
	return Config{
		MinWordLength: 3,
		WindowSize:    3,
		StopWords:     defaultStopWords,
	}
}

// ─── Stop words ──────────────────────────────────────────────────────────────

var defaultStopWords = map[string]bool{
	"и": true, "в": true, "на": true, "с": true, "что": true,
	"это": true, "как": true, "не": true, "но": true, "а": true,
	"по": true, "за": true, "то": true, "все": true, "от": true,
	"так": true, "или": true, "бы": true, "уже": true, "из": true,
	"он": true, "она": true, "они": true, "мы": true, "вы": true,
	"я": true, "ты": true, "его": true, "её": true, "их": true,
	"при": true, "был": true, "была": true, "были": true, "быть": true,
	"до": true, "об": true, "же": true, "ещё": true, "если": true,
}

// ─── Regex ───────────────────────────────────────────────────────────────────

var (
	reScriptBlock = regexp.MustCompile(`(?is)<(script|nav|footer|header|style)[^>]*>.*?</(script|nav|footer|header|style)>`)
	reHTMLTags    = regexp.MustCompile(`<[^>]+>`)
	reNonWord     = regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	reMultiSpace  = regexp.MustCompile(`\s+`)
)

// ─── Tokenize ────────────────────────────────────────────────────────────────

// Tokenize удаляет HTML-теги, знаки препинания и разбивает на слова.
// Регистр сохраняется — Lowercase вызывай отдельно.
func Tokenize(text string) []string {
	// 1. Убираем HTML
	text = reScriptBlock.ReplaceAllString(text, " ")
	text = reHTMLTags.ReplaceAllString(text, " ")
	// 2. Убираем всё кроме букв, цифр, пробелов
	text = reNonWord.ReplaceAllString(text, " ")
	// 3. Нормализуем пробелы
	text = strings.TrimSpace(reMultiSpace.ReplaceAllString(text, " "))

	if text == "" {
		return []string{}
	}

	// 4. Разбиваем и фильтруем пустые токены
	var tokens []string
	for _, t := range strings.Fields(text) {
		if hasLetter(t) {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// ─── Lowercase ───────────────────────────────────────────────────────────────

// Lowercase приводит все слова к нижнему регистру (Unicode-safe).
func Lowercase(words []string) []string {
	result := make([]string, len(words))
	for i, w := range words {
		result[i] = strings.ToLower(w)
	}
	return result
}

// ─── RemoveStopWords ─────────────────────────────────────────────────────────

// RemoveStopWords убирает слова из стоп-листа.
func RemoveStopWords(words []string) []string {
	return removeStopWordsWithSet(words, defaultStopWords)
}

func removeStopWordsWithSet(words []string, set map[string]bool) []string {
	result := make([]string, 0, len(words))
	for _, w := range words {
		if !set[strings.ToLower(w)] {
			result = append(result, w)
		}
	}
	return result
}

// ─── FilterShortWords ────────────────────────────────────────────────────────

// FilterShortWords удаляет слова короче minLen рун.
func FilterShortWords(words []string, minLen int) []string {
	if minLen <= 0 {
		return words
	}
	result := make([]string, 0, len(words))
	for _, w := range words {
		if len([]rune(w)) >= minLen {
			result = append(result, w)
		}
	}
	return result
}

// ─── CountFrequency ──────────────────────────────────────────────────────────

// CountFrequency считает частоту каждого слова и возвращает
// срез, отсортированный по убыванию частоты.
func CountFrequency(words []string) []WordFreq {
	if len(words) == 0 {
		return []WordFreq{}
	}

	counts := make(map[string]int, len(words))
	for _, w := range words {
		counts[w]++
	}

	result := make([]WordFreq, 0, len(counts))
	for word, freq := range counts {
		result = append(result, WordFreq{Word: word, Frequency: freq})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Frequency != result[j].Frequency {
			return result[i].Frequency > result[j].Frequency
		}
		return result[i].Word < result[j].Word // стабильный порядок
	})

	return result
}

// ─── BuildCooccurrence ───────────────────────────────────────────────────────

// BuildCooccurrence строит карту слов-соседей для target в скользящем окне.
// window — количество слов в каждую сторону от target.
// Пример: window=2, "a b TARGET c d" → соседи: a(1) b(2) c(2) d(1)
func BuildCooccurrence(words []string, target string, window int) map[string]int {
	result := make(map[string]int)

	for i, w := range words {
		if strings.ToLower(w) != strings.ToLower(target) {
			continue
		}

		start := max(0, i-window)
		end := min(len(words)-1, i+window)

		for j := start; j <= end; j++ {
			if j == i {
				continue
			}
			neighbor := strings.ToLower(words[j])
			distance := abs(i - j)          // чем ближе, тем больше вес
			weight := window - distance + 1 // window=2: dist1→2, dist2→1
			result[neighbor] += weight
		}
	}

	return result
}

// ─── Normalize (full pipeline) ───────────────────────────────────────────────

// Normalize запускает полный пайплайн: tokenize → lowercase → stopwords → filter.
func Normalize(text string, cfg Config) (NormalizeResult, error) {
	words := Tokenize(text)
	words = Lowercase(words)
	words = removeStopWordsWithSet(words, cfg.StopWords)
	words = FilterShortWords(words, cfg.MinWordLength)

	freqs := CountFrequency(words)

	return NormalizeResult{
		Words:       words,
		Frequencies: freqs,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
