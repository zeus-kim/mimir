// Package text provides keyword cleaning and normalization utilities.
package text

import (
	"math"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Language codes for stopword sets.
const (
	LangEnglish  = "en"
	LangKorean   = "ko"
	LangJapanese = "ja"
	LangChinese  = "zh"
)

// stopwords contains language-specific stopwords.
var stopwords = map[string]map[string]struct{}{
	LangEnglish: toSet([]string{
		"the", "and", "for", "with", "that", "this", "from", "are", "was", "were",
		"have", "has", "been", "will", "would", "could", "should", "its", "you",
		"your", "they", "their", "more", "new", "first", "after", "about", "into",
		"over", "just", "also", "now", "how", "than", "most", "some", "can", "what",
		"when", "where", "which", "who", "why", "here", "there", "these", "those",
		"been", "being", "other", "very", "only", "even", "such", "back", "down",
		"then", "them", "made", "make", "many", "much", "must", "says", "said",
		"like", "want", "year", "years", "time", "week", "today", "news", "live",
		"home", "free", "best", "good", "last", "next", "show", "shows", "video",
		"read", "reading", "continue", "post", "appeared", "full", "watch",
		"june", "july", "monday", "tuesday", "wednesday", "thursday", "friday",
		"saturday", "sunday", "2024", "2025", "2026",
	}),
	LangKorean: toSet([]string{
		"있는", "하는", "되는", "위한", "통해", "대한", "있다", "했다", "한다",
		"것으로", "것이", "것을", "에서", "으로", "에게", "까지", "부터", "만에",
		"오늘", "내일", "어제", "올해", "작년", "사진", "영상", "뉴스", "속보",
		"무료", "공개", "발표", "종합", "단독",
	}),
	LangJapanese: toSet([]string{
		"する", "した", "している", "される", "された", "について", "として",
		"ために", "という", "これ", "それ", "あれ", "この", "その", "あの",
		"ニュース", "速報", "発表", "公開",
	}),
	LangChinese: toSet([]string{
		"的", "了", "和", "是", "在", "有", "与", "为", "这", "那", "之",
		"新闻", "发布", "公开",
	}),
}

// universalStopwords are filtered regardless of language.
var universalStopwords = toSet([]string{
	"http", "https", "www", "com", "org", "net", "html", "php", "asp",
})

// toSet converts a slice to a map for O(1) lookup.
func toSet(words []string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}

// IsStopword returns true if the word is a stopword for the given language.
// English stopwords are always checked regardless of language.
func IsStopword(word, lang string) bool {
	lower := strings.ToLower(word)

	// Universal stopwords
	if _, ok := universalStopwords[lower]; ok {
		return true
	}

	// Language-specific stopwords
	if langSet, ok := stopwords[lang]; ok {
		if _, found := langSet[lower]; found {
			return true
		}
	}

	// Always check English stopwords
	if lang != LangEnglish {
		if engSet, ok := stopwords[LangEnglish]; ok {
			if _, found := engSet[lower]; found {
				return true
			}
		}
	}

	// Too short
	if len([]rune(word)) < 2 {
		return true
	}

	// Digits only
	if isDigitsOnly(word) {
		return true
	}

	return false
}

// isDigitsOnly returns true if the string contains only digits.
func isDigitsOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

// NormalizeKeyword normalizes a keyword by:
// - Trimming whitespace
// - Unicode NFC normalization
// - Lowercasing
func NormalizeKeyword(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	keyword = norm.NFC.String(keyword)
	return strings.ToLower(keyword)
}

// CleanKeywords removes stopwords and normalizes a list of keywords.
// Returns deduplicated keywords in order of first occurrence.
func CleanKeywords(keywords []string, lang string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(keywords))

	for _, kw := range keywords {
		normalized := NormalizeKeyword(kw)
		if normalized == "" {
			continue
		}
		if IsStopword(kw, lang) {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, kw) // Keep original casing
	}

	return result
}

// CategoryKeywords represents keywords for a category.
type CategoryKeywords struct {
	Category string
	Keywords []string
}

// Specificity scores for keywords based on IDF.
type Specificity map[string]float64

// ComputeSpecificity calculates IDF-based specificity scores for keywords.
// Keywords appearing in fewer categories have higher specificity (closer to 1.0).
// Keywords appearing in all categories have low specificity (closer to 0.0).
func ComputeSpecificity(categories []CategoryKeywords) Specificity {
	// Count how many categories each keyword appears in
	keywordCategoryCount := make(map[string]int)
	totalCategories := len(categories)

	for _, cat := range categories {
		for _, kw := range cat.Keywords {
			lower := strings.ToLower(kw)
			keywordCategoryCount[lower]++
		}
	}

	// Calculate IDF: log(total_categories / count) / log(total_categories)
	specificity := make(Specificity)
	logTotal := math.Log(float64(totalCategories))

	for kw, count := range keywordCategoryCount {
		if count == 1 {
			specificity[kw] = 1.0 // Only in one category = max specificity
		} else if totalCategories > 1 {
			idf := math.Log(float64(totalCategories)/float64(count)) / logTotal
			specificity[kw] = math.Max(0, idf)
		} else {
			specificity[kw] = 1.0
		}
	}

	return specificity
}

// FilterBySpecificity removes keywords below the specificity threshold.
func FilterBySpecificity(keywords []string, specificity Specificity, threshold float64) []string {
	result := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		if score, ok := specificity[lower]; ok && score >= threshold {
			result = append(result, kw)
		} else if !ok {
			// Keywords not in specificity map are kept (likely unique)
			result = append(result, kw)
		}
	}
	return result
}

// CleanAndFilterKeywords performs full keyword cleaning:
// 1. Removes stopwords
// 2. Deduplicates
// 3. Filters by specificity threshold (if categories provided)
// 4. Limits to maxCount
func CleanAndFilterKeywords(keywords []string, lang string, specificity Specificity, threshold float64, maxCount int) []string {
	// Clean and deduplicate
	cleaned := CleanKeywords(keywords, lang)

	// Filter by specificity if provided
	if specificity != nil && threshold > 0 {
		cleaned = FilterBySpecificity(cleaned, specificity, threshold)
	}

	// Limit count
	if maxCount > 0 && len(cleaned) > maxCount {
		cleaned = cleaned[:maxCount]
	}

	return cleaned
}

// MergeKeywords merges seed keywords with existing keywords.
// Seeds are prepended and the result is deduplicated.
func MergeKeywords(existing, seeds []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(seeds)+len(existing))

	// Add seeds first
	for _, kw := range seeds {
		lower := strings.ToLower(kw)
		if _, exists := seen[lower]; !exists {
			seen[lower] = struct{}{}
			result = append(result, kw)
		}
	}

	// Add existing keywords
	for _, kw := range existing {
		lower := strings.ToLower(kw)
		if _, exists := seen[lower]; !exists {
			seen[lower] = struct{}{}
			result = append(result, kw)
		}
	}

	return result
}

// Deduplicate removes duplicate keywords (case-insensitive).
// Preserves order and original casing of first occurrence.
func Deduplicate(keywords []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(keywords))

	for _, kw := range keywords {
		lower := strings.ToLower(kw)
		if _, exists := seen[lower]; !exists {
			seen[lower] = struct{}{}
			result = append(result, kw)
		}
	}

	return result
}

// AddStopwords adds custom stopwords to a language's stopword set.
func AddStopwords(lang string, words []string) {
	if _, ok := stopwords[lang]; !ok {
		stopwords[lang] = make(map[string]struct{})
	}
	for _, w := range words {
		stopwords[lang][strings.ToLower(w)] = struct{}{}
	}
}

// AddUniversalStopwords adds words to the universal stopword set.
func AddUniversalStopwords(words []string) {
	for _, w := range words {
		universalStopwords[strings.ToLower(w)] = struct{}{}
	}
}
