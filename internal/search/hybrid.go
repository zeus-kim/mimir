package search

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Result represents a single search result
type Result struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Snippet  string  `json:"snippet"`
	URL      string  `json:"url"`
	Source   string  `json:"source"` // "fts", "semantic", or "hybrid"
	Score    float64 `json:"score"`
	FTSRank  int     `json:"fts_rank,omitempty"`
	SemRank  int     `json:"sem_rank,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// EmbeddingProvider is an interface for semantic/embedding search backends
// Implement this interface to plug in different embedding providers
// (e.g., local sentence-transformers, OpenAI, Cohere, etc.)
type EmbeddingProvider interface {
	// Embed generates embeddings for the given texts
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Search performs semantic search and returns document IDs with scores
	Search(ctx context.Context, query string, limit int) ([]SemanticResult, error)
}

// SemanticResult represents a result from semantic/embedding search
type SemanticResult struct {
	ID       string
	Score    float64
	Metadata map[string]interface{}
}

// HybridSearcher combines FTS and semantic search
type HybridSearcher struct {
	db        *sql.DB
	embedding EmbeddingProvider
	config    HybridConfig
}

// HybridConfig configures hybrid search behavior
type HybridConfig struct {
	// FTSWeight is the weight for FTS results in hybrid scoring (0.0 - 1.0)
	// Default: 0.5
	FTSWeight float64

	// SemanticWeight is the weight for semantic results (0.0 - 1.0)
	// Default: 0.5
	SemanticWeight float64

	// RRFConstant is the constant k in RRF formula: 1 / (k + rank)
	// Higher values reduce the impact of rank differences
	// Default: 60
	RRFConstant float64

	// MinFTSScore filters out FTS results below this BM25 score
	MinFTSScore float64

	// MinSemanticScore filters out semantic results below this cosine similarity
	MinSemanticScore float64

	// FTSBoostExact boosts exact phrase matches in FTS
	FTSBoostExact float64

	// FTSTable is the FTS5 table name to search
	FTSTable string

	// FTSColumns are the columns to return from FTS search
	FTSColumns []string

	// IDColumn is the primary key column name
	IDColumn string

	// TitleColumn is the column containing the title
	TitleColumn string

	// SnippetColumn is the column to use for snippet generation
	SnippetColumn string
}

// DefaultConfig returns sensible defaults for hybrid search
func DefaultConfig() HybridConfig {
	return HybridConfig{
		FTSWeight:        0.5,
		SemanticWeight:   0.5,
		RRFConstant:      60,
		MinFTSScore:      0.0,
		MinSemanticScore: 0.0,
		FTSBoostExact:    1.5,
		FTSTable:         "documents_fts",
		IDColumn:         "rowid",
		TitleColumn:      "title",
		SnippetColumn:    "summary",
	}
}

// NewHybridSearcher creates a new hybrid searcher
func NewHybridSearcher(db *sql.DB, embedding EmbeddingProvider, config HybridConfig) *HybridSearcher {
	if config.FTSWeight == 0 && config.SemanticWeight == 0 {
		config = DefaultConfig()
	}
	return &HybridSearcher{
		db:        db,
		embedding: embedding,
		config:    config,
	}
}

// Search performs hybrid search combining FTS and semantic search
func (h *HybridSearcher) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 10
	}

	// Fetch more results from each source for better fusion
	fetchLimit := limit * 3

	// Run FTS and semantic search
	ftsResults, err := h.searchFTS(ctx, query, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("FTS search failed: %w", err)
	}

	var semResults []Result
	if h.embedding != nil {
		semResults, err = h.searchSemantic(ctx, query, fetchLimit)
		if err != nil {
			// Log error but continue with FTS-only results
			semResults = nil
		}
	}

	// If no semantic search, return FTS results only
	if len(semResults) == 0 {
		for i := range ftsResults {
			ftsResults[i].Source = "fts"
		}
		if len(ftsResults) > limit {
			return ftsResults[:limit], nil
		}
		return ftsResults, nil
	}

	// Fuse results using Reciprocal Rank Fusion
	fused := h.fuseResults(ftsResults, semResults)

	// Return top results
	if len(fused) > limit {
		return fused[:limit], nil
	}
	return fused, nil
}

// SearchFTSOnly performs FTS-only search
func (h *HybridSearcher) SearchFTSOnly(ctx context.Context, query string, limit int) ([]Result, error) {
	results, err := h.searchFTS(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Source = "fts"
	}
	return results, nil
}

// SearchSemanticOnly performs semantic-only search
func (h *HybridSearcher) SearchSemanticOnly(ctx context.Context, query string, limit int) ([]Result, error) {
	if h.embedding == nil {
		return nil, fmt.Errorf("no embedding provider configured")
	}
	results, err := h.searchSemantic(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Source = "semantic"
	}
	return results, nil
}

// searchFTS performs full-text search using SQLite FTS5
func (h *HybridSearcher) searchFTS(ctx context.Context, query string, limit int) ([]Result, error) {
	// Escape special FTS5 characters and prepare query
	ftsQuery := escapeFTS5Query(query)

	// Build the SQL query with BM25 scoring
	sql := fmt.Sprintf(`
		SELECT
			%s as id,
			%s as title,
			snippet(%s, -1, '<mark>', '</mark>', '...', 64) as snippet,
			bm25(%s) as score
		FROM %s
		WHERE %s MATCH ?
		ORDER BY bm25(%s)
		LIMIT ?
	`,
		h.config.IDColumn,
		h.config.TitleColumn,
		h.config.FTSTable,
		h.config.FTSTable,
		h.config.FTSTable,
		h.config.FTSTable,
		h.config.FTSTable,
	)

	rows, err := h.db.QueryContext(ctx, sql, ftsQuery, limit)
	if err != nil {
		// If the query fails, try with simpler query format
		simpleQuery := strings.Join(strings.Fields(query), " OR ")
		rows, err = h.db.QueryContext(ctx, sql, simpleQuery, limit)
		if err != nil {
			return nil, fmt.Errorf("FTS query failed: %w", err)
		}
	}
	defer rows.Close()

	var results []Result
	rank := 1
	for rows.Next() {
		var r Result
		var score float64
		if err := rows.Scan(&r.ID, &r.Title, &r.Snippet, &score); err != nil {
			continue
		}

		// BM25 returns negative scores; lower (more negative) is better
		// Convert to positive score where higher is better
		r.Score = -score
		r.FTSRank = rank
		r.Source = "fts"

		// Boost exact phrase matches
		if h.config.FTSBoostExact > 1.0 && containsExact(r.Title+" "+r.Snippet, query) {
			r.Score *= h.config.FTSBoostExact
		}

		if r.Score >= h.config.MinFTSScore {
			results = append(results, r)
			rank++
		}
	}

	return results, nil
}

// searchSemantic performs semantic search using the embedding provider
func (h *HybridSearcher) searchSemantic(ctx context.Context, query string, limit int) ([]Result, error) {
	if h.embedding == nil {
		return nil, nil
	}

	semResults, err := h.embedding.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	var results []Result
	rank := 1
	for _, sr := range semResults {
		if sr.Score < h.config.MinSemanticScore {
			continue
		}

		r := Result{
			ID:       sr.ID,
			Score:    sr.Score,
			SemRank:  rank,
			Source:   "semantic",
			Metadata: sr.Metadata,
		}

		// Try to get title from metadata
		if title, ok := sr.Metadata["title"].(string); ok {
			r.Title = title
		}
		if snippet, ok := sr.Metadata["snippet"].(string); ok {
			r.Snippet = snippet
		}

		results = append(results, r)
		rank++
	}

	return results, nil
}

// fuseResults combines FTS and semantic results using Reciprocal Rank Fusion
func (h *HybridSearcher) fuseResults(ftsResults, semResults []Result) []Result {
	k := h.config.RRFConstant
	if k <= 0 {
		k = 60 // Default RRF constant
	}

	// Build ID -> result map and calculate RRF scores
	resultMap := make(map[string]*Result)
	scores := make(map[string]float64)

	// Process FTS results
	for i, r := range ftsResults {
		rank := float64(i + 1)
		rrfScore := h.config.FTSWeight * (1.0 / (k + rank))

		if existing, ok := resultMap[r.ID]; ok {
			existing.FTSRank = i + 1
			scores[r.ID] += rrfScore
		} else {
			rCopy := r
			rCopy.FTSRank = i + 1
			resultMap[r.ID] = &rCopy
			scores[r.ID] = rrfScore
		}
	}

	// Process semantic results
	for i, r := range semResults {
		rank := float64(i + 1)
		rrfScore := h.config.SemanticWeight * (1.0 / (k + rank))

		if existing, ok := resultMap[r.ID]; ok {
			existing.SemRank = i + 1
			// Merge metadata if semantic result has it
			if r.Metadata != nil && existing.Metadata == nil {
				existing.Metadata = r.Metadata
			}
			// Use semantic snippet/title if FTS didn't have it
			if existing.Title == "" && r.Title != "" {
				existing.Title = r.Title
			}
			if existing.Snippet == "" && r.Snippet != "" {
				existing.Snippet = r.Snippet
			}
			scores[r.ID] += rrfScore
		} else {
			rCopy := r
			rCopy.SemRank = i + 1
			resultMap[r.ID] = &rCopy
			scores[r.ID] = rrfScore
		}
	}

	// Build final result list with hybrid scores
	var results []Result
	for id, r := range resultMap {
		r.Score = scores[id]
		r.Source = "hybrid"

		// Mark as found in both if it has both ranks
		if r.FTSRank > 0 && r.SemRank > 0 {
			r.Source = "hybrid:both"
		} else if r.FTSRank > 0 {
			r.Source = "hybrid:fts"
		} else {
			r.Source = "hybrid:semantic"
		}

		results = append(results, *r)
	}

	// Sort by hybrid score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// escapeFTS5Query escapes special characters in FTS5 query
func escapeFTS5Query(query string) string {
	// FTS5 special characters that need escaping: " ( ) * :
	// We wrap each token in quotes to handle special chars
	tokens := strings.Fields(query)
	var escaped []string

	for _, t := range tokens {
		// Skip empty tokens
		if t == "" {
			continue
		}

		// Remove existing quotes and escape
		t = strings.ReplaceAll(t, `"`, `""`)

		// Wrap in quotes for exact token matching
		escaped = append(escaped, `"`+t+`"`)
	}

	if len(escaped) == 0 {
		return `""`
	}

	// Join with implicit AND
	return strings.Join(escaped, " ")
}

// containsExact checks if text contains the exact query phrase (case-insensitive)
func containsExact(text, query string) bool {
	return strings.Contains(
		strings.ToLower(text),
		strings.ToLower(query),
	)
}

// NormalizeScore normalizes a score to 0-1 range using sigmoid
func NormalizeScore(score, midpoint, scale float64) float64 {
	return 1.0 / (1.0 + math.Exp(-(score-midpoint)/scale))
}

// NoOpEmbeddingProvider is a placeholder implementation that returns no results
// Use this when semantic search is not available/configured
type NoOpEmbeddingProvider struct{}

func (n *NoOpEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("embedding not configured")
}

func (n *NoOpEmbeddingProvider) Search(ctx context.Context, query string, limit int) ([]SemanticResult, error) {
	return nil, nil
}

// LocalEmbeddingProvider is a template for local embedding search
// Implement this with your actual vector store (e.g., ChromaDB, Qdrant, etc.)
type LocalEmbeddingProvider struct {
	// VectorDB connection
	// Model for generating embeddings
	// Add your fields here
}

// NewLocalEmbeddingProvider creates a placeholder local embedding provider
// Replace with actual implementation when embedding infrastructure is ready
func NewLocalEmbeddingProvider() *LocalEmbeddingProvider {
	return &LocalEmbeddingProvider{}
}

func (l *LocalEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// TODO: Implement with actual embedding model
	// Example with sentence-transformers or OpenAI:
	//
	// embeddings := make([][]float32, len(texts))
	// for i, text := range texts {
	//     embeddings[i] = l.model.Encode(text)
	// }
	// return embeddings, nil
	return nil, fmt.Errorf("local embedding not implemented")
}

func (l *LocalEmbeddingProvider) Search(ctx context.Context, query string, limit int) ([]SemanticResult, error) {
	// TODO: Implement with actual vector database
	// Example workflow:
	//
	// 1. Generate query embedding
	// queryEmb, err := l.Embed(ctx, []string{query})
	// if err != nil {
	//     return nil, err
	// }
	//
	// 2. Search vector database
	// results := l.vectorDB.Search(queryEmb[0], limit)
	//
	// 3. Convert to SemanticResult
	// return results, nil
	return nil, fmt.Errorf("local semantic search not implemented")
}

// SearchOptions provides optional parameters for search
type SearchOptions struct {
	// Filters to apply (e.g., date range, source type)
	Filters map[string]interface{}

	// Reranker to apply after fusion
	Reranker func([]Result, string) []Result

	// IncludeMetadata fetches additional metadata for results
	IncludeMetadata bool

	// HighlightQuery adds highlighting to snippets
	HighlightQuery bool

	// ExpandQuery uses query expansion
	ExpandQuery bool
}

// SearchWithOptions performs hybrid search with additional options
func (h *HybridSearcher) SearchWithOptions(ctx context.Context, query string, limit int, opts SearchOptions) ([]Result, error) {
	// Optionally expand query
	searchQuery := query
	if opts.ExpandQuery {
		searchQuery = expandQuery(query)
	}

	results, err := h.Search(ctx, searchQuery, limit*2) // Fetch extra for reranking
	if err != nil {
		return nil, err
	}

	// Apply reranker if provided
	if opts.Reranker != nil {
		results = opts.Reranker(results, query)
	}

	// Trim to limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// expandQuery is a simple query expansion (add synonyms, related terms)
// Replace with actual query expansion logic
func expandQuery(query string) string {
	// Simple implementation: just return original
	// In production, use LLM or thesaurus for expansion
	return query
}

// CrossEncoderReranker provides reranking using cross-encoder models
// This is a placeholder - implement with actual cross-encoder model
func CrossEncoderReranker(results []Result, query string) []Result {
	// TODO: Implement cross-encoder reranking
	// Example:
	// pairs := make([][]string, len(results))
	// for i, r := range results {
	//     pairs[i] = []string{query, r.Title + " " + r.Snippet}
	// }
	// scores := crossEncoder.Score(pairs)
	// ... sort by scores ...
	return results
}
