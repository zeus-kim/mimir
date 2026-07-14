// Package analysis provides trend analysis for news and blog content.
package analysis

import (
	"database/sql"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Stopwords to filter from keyword extraction
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"from": true, "are": true, "was": true, "were": true, "been": true, "have": true,
	"has": true, "had": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "must": true, "can": true, "your": true, "you": true,
	"they": true, "them": true, "their": true, "what": true, "which": true, "when": true,
	"where": true, "how": true, "who": true, "why": true, "all": true, "each": true,
	"every": true, "both": true, "few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "than": true, "too": true, "very": true, "just": true,
	"but": true, "not": true, "only": true, "own": true, "same": true, "into": true,
	"over": true, "after": true, "before": true, "between": true, "under": true,
	"again": true, "there": true, "here": true, "about": true,
	// Spanish/German common words
	"que": true, "con": true, "para": true, "una": true, "por": true, "del": true,
	"los": true, "las": true, "der": true, "und": true, "die": true,
}

var (
	wordRegex   = regexp.MustCompile(`[a-zA-Z\p{Hangul}\p{Hiragana}\p{Katakana}\p{Han}]+`)
	entityRegex = regexp.MustCompile(`[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*`)
	acronymRe   = regexp.MustCompile(`\b[A-Z]{2,5}\b`)
)

// KeywordCount represents a keyword with its count
type KeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

// BurstKeyword represents a keyword with burst metrics
type BurstKeyword struct {
	Keyword    string  `json:"keyword"`
	Current    int     `json:"current"`
	Previous   int     `json:"previous"`
	BurstScore float64 `json:"burst_score"`
}

// EntityCount represents an entity with its count
type EntityCount struct {
	Entity string `json:"entity"`
	Count  int    `json:"count"`
}

// TrendResult contains the full trend analysis results
type TrendResult struct {
	Source           string              `json:"source"`
	PeriodHours      int                 `json:"period_hours"`
	TotalDocs        int                 `json:"total_docs"`
	HotKeywords      []KeywordCount      `json:"hot_keywords"`
	BurstKeywords    []BurstKeyword      `json:"burst_keywords"`
	EmergingEntities []EntityCount       `json:"emerging_entities"`
	CategoryTopics   map[string][]string `json:"category_topics"`
	CountryTopics    map[string][]string `json:"country_topics"`
	LanguageTopics   map[string][]string `json:"language_topics"`
}

// CrossSourceTrend represents a keyword trending across sources
type CrossSourceTrend struct {
	Keyword string `json:"keyword"`
	News    int    `json:"news"`
	Blog    int    `json:"blog"`
	Total   int    `json:"total"`
}

// TrendAnalyzer provides trend analysis capabilities
type TrendAnalyzer struct {
	db *sql.DB
}

// NewTrendAnalyzer creates a new analyzer with the given database connection
func NewTrendAnalyzer(db *sql.DB) *TrendAnalyzer {
	return &TrendAnalyzer{db: db}
}

// ExtractKeywords extracts keywords from text, filtering stopwords
func ExtractKeywords(text string, minLen int) []string {
	if minLen <= 0 {
		minLen = 3
	}

	lower := strings.ToLower(text)
	matches := wordRegex.FindAllString(lower, -1)

	var keywords []string
	for _, w := range matches {
		if len(w) >= minLen && !stopwords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// ExtractEntities extracts named entities (capitalized phrases) and acronyms
func ExtractEntities(text string) []string {
	var entities []string

	// Named entities (capitalized phrases)
	entityMatches := entityRegex.FindAllString(text, -1)
	entities = append(entities, entityMatches...)

	// Acronyms (2-5 uppercase letters)
	acronyms := acronymRe.FindAllString(text, -1)
	entities = append(entities, acronyms...)

	return entities
}

// AnalyzeTrendsOptions configures the trend analysis
type AnalyzeTrendsOptions struct {
	Source   string // "news", "blog", or "all"
	Hours    int
	Category string
	Language string
	Limit    int
}

// AnalyzeTrends performs trend analysis on documents
func (t *TrendAnalyzer) AnalyzeTrends(opts AnalyzeTrendsOptions) (*TrendResult, error) {
	if opts.Source == "" {
		opts.Source = "all"
	}
	if opts.Hours <= 0 {
		opts.Hours = 24
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	now := time.Now().Unix()
	since := now - int64(opts.Hours*3600)
	prevSince := since - int64(opts.Hours*3600)

	// Build conditions
	var conditions, prevConditions []string

	if opts.Source != "all" {
		conditions = append(conditions, "source = ?")
		prevConditions = append(prevConditions, "source = ?")
	}
	conditions = append(conditions, "published_at > ?")
	prevConditions = append(prevConditions, "published_at > ?", "published_at <= ?")

	if opts.Category != "" {
		conditions = append(conditions, "normalized_category = ?")
		prevConditions = append(prevConditions, "normalized_category = ?")
	}
	if opts.Language != "" {
		conditions = append(conditions, "language = ?")
		prevConditions = append(prevConditions, "language = ?")
	}

	// Build args
	var args, prevArgs []interface{}
	if opts.Source != "all" {
		args = append(args, opts.Source)
		prevArgs = append(prevArgs, opts.Source)
	}
	args = append(args, since)
	prevArgs = append(prevArgs, prevSince, since)

	if opts.Category != "" {
		args = append(args, opts.Category)
		prevArgs = append(prevArgs, opts.Category)
	}
	if opts.Language != "" {
		args = append(args, opts.Language)
		prevArgs = append(prevArgs, opts.Language)
	}

	whereClause := "1=1"
	if len(conditions) > 0 {
		whereClause = strings.Join(conditions, " AND ")
	}
	prevWhereClause := "1=1"
	if len(prevConditions) > 0 {
		prevWhereClause = strings.Join(prevConditions, " AND ")
	}

	// Query current period
	query := "SELECT title, summary, normalized_category, country, language FROM documents WHERE " + whereClause
	rows, err := t.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	currentKeywords := make(map[string]int)
	currentEntities := make(map[string]int)
	categoryKeywords := make(map[string]map[string]int)
	countryKeywords := make(map[string]map[string]int)
	languageKeywords := make(map[string]map[string]int)

	var totalDocs int
	for rows.Next() {
		var title, summary, cat, country, lang sql.NullString
		if err := rows.Scan(&title, &summary, &cat, &country, &lang); err != nil {
			return nil, err
		}
		totalDocs++

		text := nullStr(title) + " " + nullStr(summary)
		keywords := ExtractKeywords(text, 3)
		entities := ExtractEntities(nullStr(title))

		for _, kw := range keywords {
			currentKeywords[kw]++
		}
		for _, e := range entities {
			currentEntities[e]++
		}

		if cat.Valid && cat.String != "" {
			if categoryKeywords[cat.String] == nil {
				categoryKeywords[cat.String] = make(map[string]int)
			}
			for _, kw := range keywords {
				categoryKeywords[cat.String][kw]++
			}
		}
		if country.Valid && country.String != "" {
			if countryKeywords[country.String] == nil {
				countryKeywords[country.String] = make(map[string]int)
			}
			for _, kw := range keywords {
				countryKeywords[country.String][kw]++
			}
		}
		if lang.Valid && lang.String != "" {
			if languageKeywords[lang.String] == nil {
				languageKeywords[lang.String] = make(map[string]int)
			}
			for _, kw := range keywords {
				languageKeywords[lang.String][kw]++
			}
		}
	}

	// Query previous period
	prevQuery := "SELECT title, summary FROM documents WHERE " + prevWhereClause
	prevRows, err := t.db.Query(prevQuery, prevArgs...)
	if err != nil {
		return nil, err
	}
	defer prevRows.Close()

	prevKeywords := make(map[string]int)
	prevEntitiesSet := make(map[string]bool)
	for prevRows.Next() {
		var title, summary sql.NullString
		if err := prevRows.Scan(&title, &summary); err != nil {
			return nil, err
		}
		text := nullStr(title) + " " + nullStr(summary)
		for _, kw := range ExtractKeywords(text, 3) {
			prevKeywords[kw]++
		}
		for _, e := range ExtractEntities(nullStr(title)) {
			prevEntitiesSet[e] = true
		}
	}

	// Hot keywords (top by frequency)
	hotKeywords := topKeywords(currentKeywords, opts.Limit)

	// Burst keywords (significant increase from previous period)
	minCount := 5
	if opts.Source == "blog" {
		minCount = 3
	}
	burstKeywords := calculateBurst(currentKeywords, prevKeywords, minCount, opts.Limit)

	// Emerging entities (new in current period)
	emergingEntities := findEmergingEntities(currentEntities, prevEntitiesSet, opts.Limit)

	// Build topic matrices
	categoryTopics := buildTopicMatrix(categoryKeywords, 5, 10)
	countryTopics := buildCountryTopicMatrix(countryKeywords, 3, 15)
	languageTopics := buildTopicMatrix(languageKeywords, 3, 10)

	return &TrendResult{
		Source:           opts.Source,
		PeriodHours:      opts.Hours,
		TotalDocs:        totalDocs,
		HotKeywords:      hotKeywords,
		BurstKeywords:    burstKeywords,
		EmergingEntities: emergingEntities,
		CategoryTopics:   categoryTopics,
		CountryTopics:    countryTopics,
		LanguageTopics:   languageTopics,
	}, nil
}

// CrossSourceTrends finds keywords trending across both news and blog sources
func (t *TrendAnalyzer) CrossSourceTrends(hours int) ([]CrossSourceTrend, error) {
	if hours <= 0 {
		hours = 24
	}

	since := time.Now().Unix() - int64(hours*3600)

	newsKw := make(map[string]int)
	rows, err := t.db.Query("SELECT title FROM documents WHERE source='news' AND published_at > ?", since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var title sql.NullString
		if err := rows.Scan(&title); err != nil {
			rows.Close()
			return nil, err
		}
		for _, kw := range ExtractKeywords(nullStr(title), 3) {
			newsKw[kw]++
		}
	}
	rows.Close()

	blogKw := make(map[string]int)
	rows, err = t.db.Query("SELECT title FROM documents WHERE source='blog' AND published_at > ?", since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var title sql.NullString
		if err := rows.Scan(&title); err != nil {
			rows.Close()
			return nil, err
		}
		for _, kw := range ExtractKeywords(nullStr(title), 3) {
			blogKw[kw]++
		}
	}
	rows.Close()

	// Find keywords appearing in both sources
	var cross []CrossSourceTrend
	for kw, newsCount := range newsKw {
		blogCount, ok := blogKw[kw]
		if ok && newsCount >= 3 && blogCount >= 2 {
			cross = append(cross, CrossSourceTrend{
				Keyword: kw,
				News:    newsCount,
				Blog:    blogCount,
				Total:   newsCount + blogCount,
			})
		}
	}

	// Sort by total count descending
	sort.Slice(cross, func(i, j int) bool {
		return cross[i].Total > cross[j].Total
	})

	if len(cross) > 20 {
		cross = cross[:20]
	}
	return cross, nil
}

// Helper functions

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func topKeywords(counts map[string]int, limit int) []KeywordCount {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})

	var result []KeywordCount
	for i, item := range sorted {
		if i >= limit {
			break
		}
		result = append(result, KeywordCount{Keyword: item.k, Count: item.v})
	}
	return result
}

func calculateBurst(current, prev map[string]int, minCount, limit int) []BurstKeyword {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range current {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})

	var bursts []BurstKeyword
	maxCheck := 100
	if len(sorted) < maxCheck {
		maxCheck = len(sorted)
	}

	for i := 0; i < maxCheck; i++ {
		word := sorted[i].k
		count := sorted[i].v
		prevCount := prev[word]

		if count >= minCount {
			var burstScore float64
			if prevCount == 0 {
				burstScore = float64(count) * 10
			} else {
				burstScore = float64(count) / float64(max(prevCount, 1))
			}

			if burstScore >= 1.5 {
				bursts = append(bursts, BurstKeyword{
					Keyword:    word,
					Current:    count,
					Previous:   prevCount,
					BurstScore: roundFloat(burstScore, 2),
				})
			}
		}
	}

	sort.Slice(bursts, func(i, j int) bool {
		return bursts[i].BurstScore > bursts[j].BurstScore
	})

	if len(bursts) > limit {
		bursts = bursts[:limit]
	}
	return bursts
}

func findEmergingEntities(current map[string]int, prevSet map[string]bool, limit int) []EntityCount {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range current {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})

	var emerging []EntityCount
	maxCheck := 50
	if len(sorted) < maxCheck {
		maxCheck = len(sorted)
	}

	for i := 0; i < maxCheck && len(emerging) < limit; i++ {
		entity := sorted[i].k
		count := sorted[i].v

		// Check if new, has minimum count, and minimum length
		if !prevSet[entity] && count >= 3 && entityLength(entity) > 2 {
			emerging = append(emerging, EntityCount{Entity: entity, Count: count})
		}
	}
	return emerging
}

func entityLength(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return count
}

func buildTopicMatrix(kwMaps map[string]map[string]int, topN, maxEntries int) map[string][]string {
	result := make(map[string][]string)

	type entry struct {
		key   string
		words []string
	}
	var entries []entry

	for key, counts := range kwMaps {
		if key == "" {
			continue
		}
		top := topKeywords(counts, topN)
		var words []string
		for _, kw := range top {
			words = append(words, kw.Keyword)
		}
		entries = append(entries, entry{key, words})
	}

	// Sort by number of words descending
	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].words) > len(entries[j].words)
	})

	for i, e := range entries {
		if i >= maxEntries {
			break
		}
		result[e.key] = e.words
	}
	return result
}

func buildCountryTopicMatrix(kwMaps map[string]map[string]int, topN, maxEntries int) map[string][]string {
	result := make(map[string][]string)

	type entry struct {
		key   string
		words []string
	}
	var entries []entry

	for key, counts := range kwMaps {
		// Only include 2-letter country codes
		if key == "" || len(key) != 2 {
			continue
		}
		top := topKeywords(counts, topN)
		var words []string
		for _, kw := range top {
			words = append(words, kw.Keyword)
		}
		entries = append(entries, entry{key, words})
	}

	sort.Slice(entries, func(i, j int) bool {
		return len(entries[i].words) > len(entries[j].words)
	})

	for i, e := range entries {
		if i >= maxEntries {
			break
		}
		result[e.key] = e.words
	}
	return result
}

func roundFloat(val float64, precision int) float64 {
	mult := 1.0
	for i := 0; i < precision; i++ {
		mult *= 10
	}
	return float64(int(val*mult+0.5)) / mult
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
