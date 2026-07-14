package search

import (
	"regexp"
	"sort"
	"strings"
)

// RelevanceModel - ACHE 스타일 용어별 점수 추적 및 쿼리 확장
type RelevanceModel struct {
	TermScores  map[string]float64
	TermDocFreq map[string]int
	UsedTerms   map[string]bool
}

// NewRelevanceModel creates a new relevance model for term scoring
func NewRelevanceModel() *RelevanceModel {
	return &RelevanceModel{
		TermScores:  make(map[string]float64),
		TermDocFreq: make(map[string]int),
		UsedTerms:   make(map[string]bool),
	}
}

// AddPage updates term scores from a page
func (rm *RelevanceModel) AddPage(text string, isRelevant bool) {
	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return
	}

	docLen := float64(len(tokens))
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}

	for term, count := range tf {
		weight := float64(count) / docLen
		rm.TermDocFreq[term]++

		if isRelevant {
			rm.TermScores[term] += weight
		} else {
			rm.TermScores[term] -= weight * 0.5
		}
	}
}

// GetBestTerms returns the top N terms by score
func (rm *RelevanceModel) GetBestTerms(n int, excludeUsed bool) []string {
	type termScore struct {
		term  string
		score float64
	}

	var candidates []termScore
	for term, score := range rm.TermScores {
		if score <= 0 {
			continue
		}
		if excludeUsed && rm.UsedTerms[term] {
			continue
		}
		candidates = append(candidates, termScore{term, score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	result := make([]string, 0, n)
	for i := 0; i < len(candidates) && i < n; i++ {
		result = append(result, candidates[i].term)
	}

	return result
}

// GetBestUnusedTerm returns the highest scoring unused term
func (rm *RelevanceModel) GetBestUnusedTerm() string {
	var bestTerm string
	var bestScore float64 = -1

	for term, score := range rm.TermScores {
		if score > 0 && !rm.UsedTerms[term] && score > bestScore {
			bestTerm = term
			bestScore = score
		}
	}

	return bestTerm
}

// MarkUsed marks a term as used
func (rm *RelevanceModel) MarkUsed(term string) {
	rm.UsedTerms[term] = true
}

// Reweight adjusts term weight based on search precision
func (rm *RelevanceModel) Reweight(term string, precision float64) {
	if score, ok := rm.TermScores[term]; ok {
		rm.TermScores[term] = score * precision
	}
}

// QueryHistory tracks query performance
type QueryHistory struct {
	Query     []string
	Precision float64
}

// FeedbackResult contains the result of query feedback
type FeedbackResult struct {
	Precision float64
	NewTerms  []string
	NextQuery []string
	Round     int
}

// QueryExpander handles automatic query expansion
type QueryExpander struct {
	Topic           string
	InitialKeywords []string
	RelevanceModel  *RelevanceModel
	CurrentQuery    []string
	QueryHistory    []QueryHistory
	MinPrecision    float64
	MaxRounds       int
	Round           int
}

// NewQueryExpander creates a new query expander
func NewQueryExpander(initialKeywords []string, topic string) *QueryExpander {
	qe := &QueryExpander{
		Topic:           topic,
		InitialKeywords: initialKeywords,
		RelevanceModel:  NewRelevanceModel(),
		MinPrecision:    0.5,
		MaxRounds:       10,
		Round:           0,
	}

	// Start with top 3 keywords
	if len(initialKeywords) >= 3 {
		qe.CurrentQuery = make([]string, 3)
		copy(qe.CurrentQuery, initialKeywords[:3])
	} else {
		qe.CurrentQuery = make([]string, len(initialKeywords))
		copy(qe.CurrentQuery, initialKeywords)
	}

	// Set initial keyword scores
	for i, kw := range initialKeywords {
		qe.RelevanceModel.TermScores[strings.ToLower(kw)] = 1.0 - float64(i)*0.05
	}

	return qe
}

// GetCurrentQuery returns a copy of the current query
func (qe *QueryExpander) GetCurrentQuery() []string {
	result := make([]string, len(qe.CurrentQuery))
	copy(result, qe.CurrentQuery)
	return result
}

// GetSearchStrings generates query strings for searching
func (qe *QueryExpander) GetSearchStrings() []string {
	queries := make([]string, 0, 4)

	// Base query
	queries = append(queries, strings.Join(qe.CurrentQuery, " "))

	// RSS/feed discovery
	if len(qe.CurrentQuery) > 0 {
		queries = append(queries, qe.CurrentQuery[0]+" RSS feed")
		queries = append(queries, qe.CurrentQuery[0]+" blog")
	}

	// News discovery
	if len(qe.CurrentQuery) >= 2 {
		queries = append(queries, qe.CurrentQuery[0]+" "+qe.CurrentQuery[1]+" news")
	}

	return queries
}

// Feedback processes search results and adjusts the query
func (qe *QueryExpander) Feedback(results []string, isRelevant []bool) FeedbackResult {
	if len(results) == 0 {
		return FeedbackResult{
			Precision: 0,
			NewTerms:  nil,
			NextQuery: qe.CurrentQuery,
			Round:     qe.Round,
		}
	}

	// Calculate precision
	positive := 0
	for _, rel := range isRelevant {
		if rel {
			positive++
		}
	}
	precision := float64(positive) / float64(len(results))

	// Learn terms from results
	for i, text := range results {
		if i < len(isRelevant) {
			qe.RelevanceModel.AddPage(text, isRelevant[i])
		}
	}

	// Adjust query
	qe.Round++
	qe.adjustQuery(precision)

	// Get newly discovered good terms
	newTerms := qe.RelevanceModel.GetBestTerms(5, true)

	return FeedbackResult{
		Precision: precision,
		NewTerms:  newTerms,
		NextQuery: qe.CurrentQuery,
		Round:     qe.Round,
	}
}

func (qe *QueryExpander) adjustQuery(precision float64) {
	// Record history
	qe.QueryHistory = append(qe.QueryHistory, QueryHistory{
		Query:     qe.GetCurrentQuery(),
		Precision: precision,
	})

	// Adjust current term weights
	for _, term := range qe.CurrentQuery {
		termLower := strings.ToLower(term)
		qe.RelevanceModel.Reweight(termLower, precision)
		qe.RelevanceModel.MarkUsed(termLower)
	}

	querySize := len(qe.CurrentQuery)

	if precision < qe.MinPrecision {
		// Low precision -> be more specific (expand query)
		if querySize < 5 {
			querySize++
		}
	} else if precision > 0.8 {
		// High precision -> broaden scope (reduce query)
		if querySize > 2 {
			querySize--
		}
	}

	// Build new query
	bestTerms := qe.RelevanceModel.GetBestTerms(querySize-1, false)
	unusedTerm := qe.RelevanceModel.GetBestUnusedTerm()

	newQuery := make([]string, 0, querySize)
	if querySize > 1 && len(bestTerms) > 0 {
		count := querySize - 1
		if count > len(bestTerms) {
			count = len(bestTerms)
		}
		newQuery = append(newQuery, bestTerms[:count]...)
	}

	if unusedTerm != "" && !contains(newQuery, unusedTerm) {
		newQuery = append(newQuery, unusedTerm)
	}

	// Keep at least one initial keyword
	hasInitial := false
	for _, kw := range qe.InitialKeywords[:min(2, len(qe.InitialKeywords))] {
		for _, q := range newQuery {
			if strings.EqualFold(kw, q) {
				hasInitial = true
				break
			}
		}
	}

	if !hasInitial && len(newQuery) > 0 && len(qe.InitialKeywords) > 0 {
		newQuery[0] = qe.InitialKeywords[0]
	}

	if len(newQuery) > querySize {
		newQuery = newQuery[:querySize]
	}

	qe.CurrentQuery = newQuery
}

// ShouldContinue returns true if expansion should continue
func (qe *QueryExpander) ShouldContinue() bool {
	if qe.Round >= qe.MaxRounds {
		return false
	}

	// Check recent precision
	if len(qe.QueryHistory) >= 3 {
		var sumPrecision float64
		for i := len(qe.QueryHistory) - 3; i < len(qe.QueryHistory); i++ {
			sumPrecision += qe.QueryHistory[i].Precision
		}
		avgPrecision := sumPrecision / 3.0
		if avgPrecision < 0.2 {
			return false
		}
	}

	return true
}

// GetAllDiscoveredTerms returns all discovered high-scoring terms
func (qe *QueryExpander) GetAllDiscoveredTerms() []string {
	return qe.RelevanceModel.GetBestTerms(30, false)
}

// SeedResult represents a discovered seed URL
type SeedResult struct {
	URL   string
	Title string
	Score float64
	Query string
	Round int
}

// SeedFinder discovers seed URLs using query expansion
type SeedFinder struct {
	Topic          string
	Expander       *QueryExpander
	Classifier     Scorer
	DiscoveredURLs map[string]bool
	GoodURLs       []SeedResult
}

// Scorer interface for relevance scoring
type Scorer interface {
	Score(text string) float64
}

// NewSeedFinder creates a new seed finder
func NewSeedFinder(topic string, keywords []string, classifier Scorer) *SeedFinder {
	return &SeedFinder{
		Topic:          topic,
		Expander:       NewQueryExpander(keywords, topic),
		Classifier:     classifier,
		DiscoveredURLs: make(map[string]bool),
	}
}

// SearchResult represents a search engine result
type SearchResult struct {
	URL     string
	Title   string
	Snippet string
}

// SearchFunc is the signature for a search function
type SearchFunc func(query string) []SearchResult

// FindSeeds discovers seed URLs using the provided search function
func (sf *SeedFinder) FindSeeds(searchFunc SearchFunc, maxRounds int) []SeedResult {
	for roundNum := 0; roundNum < maxRounds; roundNum++ {
		queries := sf.Expander.GetSearchStrings()

		var roundResults []string
		var roundRelevance []bool

		for _, query := range queries {
			results := searchFunc(query)

			for _, r := range results {
				if r.URL == "" || sf.DiscoveredURLs[r.URL] {
					continue
				}

				sf.DiscoveredURLs[r.URL] = true

				text := r.Title + " " + r.Snippet
				score := sf.Classifier.Score(text)
				isRelevant := score >= 0.3

				roundResults = append(roundResults, text)
				roundRelevance = append(roundRelevance, isRelevant)

				if isRelevant {
					sf.GoodURLs = append(sf.GoodURLs, SeedResult{
						URL:   r.URL,
						Title: r.Title,
						Score: score,
						Query: query,
						Round: roundNum,
					})
				}
			}
		}

		// Feedback
		if len(roundResults) > 0 {
			sf.Expander.Feedback(roundResults, roundRelevance)
		}

		if !sf.Expander.ShouldContinue() {
			break
		}
	}

	// Sort by score descending
	sort.Slice(sf.GoodURLs, func(i, j int) bool {
		return sf.GoodURLs[i].Score > sf.GoodURLs[j].Score
	})

	return sf.GoodURLs
}

// Helper functions

var expanderTokenRegex = regexp.MustCompile(`[\p{L}\p{N}]+`)

func tokenizeText(text string) []string {
	textLower := strings.ToLower(text)
	matches := expanderTokenRegex.FindAllString(textLower, -1)
	var tokens []string
	for _, m := range matches {
		if len(m) >= 2 {
			tokens = append(tokens, m)
		}
	}
	return tokens
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
