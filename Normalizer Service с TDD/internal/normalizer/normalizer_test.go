package normalizer_test

// TDD: тесты написаны ДО реализации.
// Запусти: go test ./internal/normalizer/... -v
// Сначала всё RED — потом пишем код пока не GREEN.

import (
	"testing"

	"github.com/brandmon/normalizer-service/internal/normalizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Tokenize ───────────────────────────────────────────────────────────────

func TestTokenize_BasicRussian(t *testing.T) {
	words := normalizer.Tokenize("Сбербанк — лучший банк России!")
	assert.Equal(t, []string{"Сбербанк", "лучший", "банк", "России"}, words)
}

func TestTokenize_RemovesPunctuation(t *testing.T) {
	words := normalizer.Tokenize("привет, мир! как дела?")
	assert.Equal(t, []string{"привет", "мир", "как", "дела"}, words)
}

func TestTokenize_RemovesHTMLTags(t *testing.T) {
	words := normalizer.Tokenize("<p>Отличный <b>сервис</b></p>")
	assert.Equal(t, []string{"Отличный", "сервис"}, words)
}

func TestTokenize_EmptyString(t *testing.T) {
	words := normalizer.Tokenize("")
	assert.Empty(t, words)
}

func TestTokenize_OnlyPunctuation(t *testing.T) {
	words := normalizer.Tokenize("!!! ??? ...")
	assert.Empty(t, words)
}

func TestTokenize_MixedLanguages(t *testing.T) {
	words := normalizer.Tokenize("Сбербанк Sber банк bank")
	assert.Equal(t, []string{"Сбербанк", "Sber", "банк", "bank"}, words)
}

// ─── Lowercase ──────────────────────────────────────────────────────────────

func TestLowercase_Russian(t *testing.T) {
	result := normalizer.Lowercase([]string{"СБЕРБАНК", "Банк", "банк"})
	assert.Equal(t, []string{"сбербанк", "банк", "банк"}, result)
}

func TestLowercase_EmptySlice(t *testing.T) {
	result := normalizer.Lowercase([]string{})
	assert.Empty(t, result)
}

// ─── RemoveStopWords ────────────────────────────────────────────────────────

func TestRemoveStopWords_RemovesCommonWords(t *testing.T) {
	input := []string{"это", "хороший", "банк", "и", "он", "лучший"}
	result := normalizer.RemoveStopWords(input)
	assert.Equal(t, []string{"хороший", "банк", "лучший"}, result)
}

func TestRemoveStopWords_PreservesUnknownWords(t *testing.T) {
	input := []string{"сбербанк", "надёжный", "быстрый"}
	result := normalizer.RemoveStopWords(input)
	assert.Equal(t, input, result)
}

func TestRemoveStopWords_AllStopWords(t *testing.T) {
	input := []string{"и", "в", "на", "с", "что"}
	result := normalizer.RemoveStopWords(input)
	assert.Empty(t, result)
}

// ─── FilterShortWords ────────────────────────────────────────────────────────

func TestFilterShortWords_RemovesShort(t *testing.T) {
	input := []string{"я", "ок", "банк", "сбер"}
	result := normalizer.FilterShortWords(input, 3)
	assert.Equal(t, []string{"банк", "сбер"}, result)
}

func TestFilterShortWords_ZeroMinLength(t *testing.T) {
	input := []string{"я", "ок", "банк"}
	result := normalizer.FilterShortWords(input, 0)
	assert.Equal(t, input, result)
}

// ─── CountFrequency ──────────────────────────────────────────────────────────

func TestCountFrequency_BasicCount(t *testing.T) {
	input := []string{"банк", "сбер", "банк", "хорошо", "банк"}
	result := normalizer.CountFrequency(input)

	require.Len(t, result, 3)

	freq := freqMap(result)
	assert.Equal(t, 3, freq["банк"])
	assert.Equal(t, 1, freq["сбер"])
	assert.Equal(t, 1, freq["хорошо"])
}

func TestCountFrequency_EmptyInput(t *testing.T) {
	result := normalizer.CountFrequency([]string{})
	assert.Empty(t, result)
}

func TestCountFrequency_SingleWord(t *testing.T) {
	result := normalizer.CountFrequency([]string{"банк"})
	require.Len(t, result, 1)
	assert.Equal(t, "банк", result[0].Word)
	assert.Equal(t, 1, result[0].Frequency)
}

func TestCountFrequency_SortedByFrequencyDesc(t *testing.T) {
	input := []string{"а", "б", "б", "в", "в", "в"}
	result := normalizer.CountFrequency(input)

	require.Len(t, result, 3)
	assert.Equal(t, "в", result[0].Word)
	assert.Equal(t, 3, result[0].Frequency)
	assert.Equal(t, "б", result[1].Word)
	assert.Equal(t, 2, result[1].Frequency)
}

// ─── BuildCooccurrence ───────────────────────────────────────────────────────

func TestBuildCooccurrence_BasicWindow(t *testing.T) {
	// "сбер хороший банк надёжный" с окном 2
	// сбер соседствует с: хороший, банк
	words := []string{"сбер", "хороший", "банк", "надёжный"}
	result := normalizer.BuildCooccurrence(words, "сбер", 2)

	assert.Equal(t, 2, result["хороший"])
	assert.Equal(t, 1, result["банк"])
	assert.Zero(t, result["надёжный"]) // вне окна
}

func TestBuildCooccurrence_TargetNotFound(t *testing.T) {
	words := []string{"хороший", "банк", "надёжный"}
	result := normalizer.BuildCooccurrence(words, "сбер", 2)
	assert.Empty(t, result)
}

func TestBuildCooccurrence_MultipleOccurrences(t *testing.T) {
	// "сбер банк сбер надёжный" с окном 1
	// первый сбер: сосед банк (weight=1)
	// второй сбер: соседи банк(weight=1) и надёжный(weight=1)
	words := []string{"сбер", "банк", "сбер", "надёжный"}
	result := normalizer.BuildCooccurrence(words, "сбер", 1)

	assert.Equal(t, 2, result["банк"]) // встречается дважды рядом
	assert.Equal(t, 1, result["надёжный"])
}

func TestBuildCooccurrence_WindowLargerThanText(t *testing.T) {
	words := []string{"сбер", "банк"}
	result := normalizer.BuildCooccurrence(words, "сбер", 10)
	// window=10, distance=1, weight = 10-1+1 = 10
	assert.Equal(t, 10, result["банк"])
}

// ─── Normalize (full pipeline) ───────────────────────────────────────────────

func TestNormalize_FullPipeline(t *testing.T) {
	text := "<p>Сбербанк — это <b>хороший</b> и надёжный банк!</p>"
	result, err := normalizer.Normalize(text, normalizer.DefaultConfig())

	require.NoError(t, err)
	require.NotEmpty(t, result.Words)

	wordSet := toSet(result.Words)
	assert.Contains(t, wordSet, "сбербанк")
	assert.Contains(t, wordSet, "хороший")
	assert.Contains(t, wordSet, "надёжный")
	assert.Contains(t, wordSet, "банк")
	assert.NotContains(t, wordSet, "это") // стоп-слово
	assert.NotContains(t, wordSet, "и")   // стоп-слово
}

func TestNormalize_ReturnsFrequencies(t *testing.T) {
	text := "банк банк банк хороший сервис"
	result, err := normalizer.Normalize(text, normalizer.DefaultConfig())

	require.NoError(t, err)
	freq := freqMap(result.Frequencies)
	assert.Equal(t, 3, freq["банк"])
}

func TestNormalize_EmptyText(t *testing.T) {
	result, err := normalizer.Normalize("", normalizer.DefaultConfig())
	require.NoError(t, err)
	assert.Empty(t, result.Words)
	assert.Empty(t, result.Frequencies)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func freqMap(freqs []normalizer.WordFreq) map[string]int {
	m := make(map[string]int)
	for _, f := range freqs {
		m[f.Word] = f.Frequency
	}
	return m
}

func toSet(words []string) map[string]bool {
	s := make(map[string]bool)
	for _, w := range words {
		s[w] = true
	}
	return s
}
