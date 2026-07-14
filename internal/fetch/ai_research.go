package fetch

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/user/mimir-mcp/internal/db"
)

// ============================================================================
// Data Structures
// ============================================================================

// ArxivPaper represents a paper from arXiv
type ArxivPaper struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Authors    []string `json:"authors"`
	Abstract   string   `json:"abstract"`
	Categories []string `json:"categories"`
	Published  string   `json:"published"`
	Updated    string   `json:"updated"`
	PDFLink    string   `json:"pdf_link"`
}

// SemanticScholarPaper represents a paper from Semantic Scholar
type SemanticScholarPaper struct {
	PaperID              string   `json:"paper_id"`
	Title                string   `json:"title"`
	Abstract             string   `json:"abstract"`
	Authors              []string `json:"authors"`
	Year                 int      `json:"year"`
	CitationCount        int      `json:"citation_count"`
	InfluentialCitations int      `json:"influential_citations"`
	Venue                string   `json:"venue"`
	URL                  string   `json:"url"`
}

// HuggingFaceModel represents a model from HuggingFace Hub
type HuggingFaceModel struct {
	ModelID     string   `json:"model_id"`
	Author      string   `json:"author"`
	Downloads   int      `json:"downloads"`
	Likes       int      `json:"likes"`
	Tags        []string `json:"tags"`
	PipelineTag string   `json:"pipeline_tag"`
	LastUpdated string   `json:"last_updated"`
}

// PapersWithCodePaper represents a paper from Papers With Code
type PapersWithCodePaper struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Abstract   string `json:"abstract"`
	URLPdf     string `json:"url_pdf"`
	URLAbs     string `json:"url_abs"`
	Proceeding string `json:"proceeding"`
	ArxivID    string `json:"arxiv_id"`
	Published  string `json:"published"`
}

// ============================================================================
// arXiv Fetcher
// ============================================================================

// ArxivFetcher fetches papers from arXiv API
type ArxivFetcher struct {
	BaseURL string
}

// NewArxivFetcher creates a new arXiv fetcher
func NewArxivFetcher() *ArxivFetcher {
	return &ArxivFetcher{
		BaseURL: "https://export.arxiv.org/api/query",
	}
}

// arXiv Atom feed structures
type arxivFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Categories []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
	Links []struct {
		Href  string `xml:"href,attr"`
		Title string `xml:"title,attr"`
		Type  string `xml:"type,attr"`
		Rel   string `xml:"rel,attr"`
	} `xml:"link"`
}

// Fetch fetches papers from arXiv
func (f *ArxivFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	// Build search query for AI/ML categories
	searchQuery := fmt.Sprintf("all:%s AND (cat:cs.AI OR cat:cs.LG OR cat:cs.CL OR cat:cs.CV OR cat:stat.ML)", query)

	params := url.Values{}
	params.Set("search_query", searchQuery)
	params.Set("start", "0")
	params.Set("max_results", fmt.Sprintf("%d", limit))
	params.Set("sortBy", "submittedDate")
	params.Set("sortOrder", "descending")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("arxiv request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("arxiv returned status %d", resp.StatusCode)
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return 0, fmt.Errorf("arxiv xml decode failed: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers
		(paper_id, source, title, abstract, authors, categories, published, updated, pdf_link, citation_count)
		VALUES (?, 'arxiv', ?, ?, ?, ?, ?, ?, ?, 0)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers_fts
		(paper_id, title, abstract, authors, categories)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, entry := range feed.Entries {
		// Extract paper ID from URL (e.g., "http://arxiv.org/abs/2301.12345v1" -> "2301.12345")
		paperID := extractArxivID(entry.ID)

		// Extract authors
		var authors []string
		for _, a := range entry.Authors {
			authors = append(authors, strings.TrimSpace(a.Name))
		}
		authorsStr := strings.Join(authors, "; ")

		// Extract categories
		var categories []string
		for _, c := range entry.Categories {
			categories = append(categories, c.Term)
		}
		categoriesStr := strings.Join(categories, ", ")

		// Find PDF link
		var pdfLink string
		for _, link := range entry.Links {
			if link.Title == "pdf" || link.Type == "application/pdf" {
				pdfLink = link.Href
				break
			}
		}
		if pdfLink == "" {
			// Construct PDF link from ID
			pdfLink = fmt.Sprintf("https://arxiv.org/pdf/%s.pdf", paperID)
		}

		// Clean title and abstract (remove extra whitespace)
		title := strings.Join(strings.Fields(entry.Title), " ")
		abstract := strings.Join(strings.Fields(entry.Summary), " ")

		_, err := stmt.Exec(
			paperID, title, abstract, authorsStr, categoriesStr,
			entry.Published, entry.Updated, pdfLink,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(paperID, title, abstract, authorsStr, categoriesStr)
		count++
	}

	return count, nil
}

func extractArxivID(idURL string) string {
	// http://arxiv.org/abs/2301.12345v1 -> 2301.12345
	parts := strings.Split(idURL, "/abs/")
	if len(parts) == 2 {
		id := parts[1]
		// Remove version suffix
		if idx := strings.LastIndex(id, "v"); idx > 0 {
			id = id[:idx]
		}
		return id
	}
	return idURL
}

// ============================================================================
// Semantic Scholar Fetcher
// ============================================================================

// SemanticScholarFetcher fetches papers from Semantic Scholar API
type SemanticScholarFetcher struct {
	BaseURL string
}

// NewSemanticScholarFetcher creates a new Semantic Scholar fetcher
func NewSemanticScholarFetcher() *SemanticScholarFetcher {
	return &SemanticScholarFetcher{
		BaseURL: "https://api.semanticscholar.org/graph/v1/paper/search",
	}
}

type s2Response struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Data   []struct {
		PaperID              string `json:"paperId"`
		Title                string `json:"title"`
		Abstract             string `json:"abstract"`
		Year                 int    `json:"year"`
		CitationCount        int    `json:"citationCount"`
		InfluentialCitationCount int `json:"influentialCitationCount"`
		Venue                string `json:"venue"`
		URL                  string `json:"url"`
		Authors              []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"data"`
}

// Fetch fetches papers from Semantic Scholar
func (f *SemanticScholarFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100 // API limit
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("fields", "paperId,title,abstract,year,citationCount,influentialCitationCount,venue,url,authors")

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", f.BaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}
	// Add user agent to avoid rate limiting
	req.Header.Set("User-Agent", "mimir-research-fetcher/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("semantic scholar request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("semantic scholar returned status %d", resp.StatusCode)
	}

	var result s2Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("semantic scholar json decode failed: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers
		(paper_id, source, title, abstract, authors, categories, published, updated, pdf_link, citation_count, influential_citations, venue, url)
		VALUES (?, 'semantic_scholar', ?, ?, ?, '', ?, '', '', ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers_fts
		(paper_id, title, abstract, authors, categories)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, paper := range result.Data {
		if paper.PaperID == "" {
			continue
		}

		var authors []string
		for _, a := range paper.Authors {
			authors = append(authors, a.Name)
		}
		authorsStr := strings.Join(authors, "; ")

		published := ""
		if paper.Year > 0 {
			published = fmt.Sprintf("%d", paper.Year)
		}

		abstract := paper.Abstract
		if abstract == "" {
			abstract = "(abstract not available)"
		}

		_, err := stmt.Exec(
			paper.PaperID, paper.Title, abstract, authorsStr, published,
			paper.CitationCount, paper.InfluentialCitationCount, paper.Venue, paper.URL,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(paper.PaperID, paper.Title, abstract, authorsStr, paper.Venue)
		count++
	}

	return count, nil
}

// ============================================================================
// HuggingFace Hub Fetcher
// ============================================================================

// HuggingFaceFetcher fetches models from HuggingFace Hub API
type HuggingFaceFetcher struct {
	BaseURL string
}

// NewHuggingFaceFetcher creates a new HuggingFace Hub fetcher
func NewHuggingFaceFetcher() *HuggingFaceFetcher {
	return &HuggingFaceFetcher{
		BaseURL: "https://huggingface.co/api/models",
	}
}

type hfModel struct {
	ModelID       string   `json:"modelId"`
	ID            string   `json:"id"`
	Author        string   `json:"author"`
	Downloads     int      `json:"downloads"`
	Likes         int      `json:"likes"`
	Tags          []string `json:"tags"`
	PipelineTag   string   `json:"pipeline_tag"`
	LastModified  string   `json:"lastModified"`
	Private       bool     `json:"private"`
	LibraryName   string   `json:"library_name"`
}

// Fetch fetches models from HuggingFace Hub
func (f *HuggingFaceFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("search", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("sort", "downloads")
	params.Set("direction", "-1")
	params.Set("full", "false")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("huggingface request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("huggingface returned status %d", resp.StatusCode)
	}

	var models []hfModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return 0, fmt.Errorf("huggingface json decode failed: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_models
		(model_id, source, author, downloads, likes, tags, pipeline_tag, library_name, last_updated)
		VALUES (?, 'huggingface', ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_models_fts
		(model_id, author, tags, pipeline_tag)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, model := range models {
		if model.Private {
			continue // Skip private models
		}

		modelID := model.ModelID
		if modelID == "" {
			modelID = model.ID
		}
		if modelID == "" {
			continue
		}

		tagsStr := strings.Join(model.Tags, ", ")

		_, err := stmt.Exec(
			modelID, model.Author, model.Downloads, model.Likes,
			tagsStr, model.PipelineTag, model.LibraryName, model.LastModified,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(modelID, model.Author, tagsStr, model.PipelineTag)
		count++
	}

	return count, nil
}

// ============================================================================
// Papers With Code Fetcher
// ============================================================================

// PapersWithCodeFetcher fetches papers from Papers With Code API
type PapersWithCodeFetcher struct {
	BaseURL string
}

// NewPapersWithCodeFetcher creates a new Papers With Code fetcher
func NewPapersWithCodeFetcher() *PapersWithCodeFetcher {
	return &PapersWithCodeFetcher{
		BaseURL: "https://paperswithcode.com/api/v1/papers/",
	}
}

type pwcResponse struct {
	Count    int `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Abstract   string `json:"abstract"`
		URLPdf     string `json:"url_pdf"`
		URLAbs     string `json:"url_abs"`
		Proceeding string `json:"proceeding"`
		ArxivID    string `json:"arxiv_id"`
		Published  string `json:"published"`
	} `json:"results"`
}

// Fetch fetches papers from Papers With Code
func (f *PapersWithCodeFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("items_per_page", fmt.Sprintf("%d", min(limit, 50)))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("papers with code request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("papers with code returned status %d", resp.StatusCode)
	}

	var result pwcResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("papers with code json decode failed: %w", err)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers
		(paper_id, source, title, abstract, authors, categories, published, updated, pdf_link, citation_count, proceeding, arxiv_id, url)
		VALUES (?, 'papers_with_code', ?, ?, '', '', ?, '', ?, 0, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO ai_papers_fts
		(paper_id, title, abstract, authors, categories)
		VALUES (?, ?, ?, '', ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, paper := range result.Results {
		paperID := paper.ID
		if paperID == "" && paper.ArxivID != "" {
			paperID = "pwc:" + paper.ArxivID
		}
		if paperID == "" {
			continue
		}

		abstract := paper.Abstract
		if abstract == "" {
			abstract = "(abstract not available)"
		}

		_, err := stmt.Exec(
			paperID, paper.Title, abstract, paper.Published, paper.URLPdf,
			paper.Proceeding, paper.ArxivID, paper.URLAbs,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(paperID, paper.Title, abstract, paper.Proceeding)
		count++
	}

	return count, nil
}

// ============================================================================
// AI Research Fetcher (Unified Interface)
// ============================================================================

// AIResearchFetcher combines all AI/ML API sources
type AIResearchFetcher struct {
	arxiv           *ArxivFetcher
	semanticScholar *SemanticScholarFetcher
	huggingFace     *HuggingFaceFetcher
	papersWithCode  *PapersWithCodeFetcher
}

// NewAIResearchFetcher creates a new unified AI research fetcher
func NewAIResearchFetcher() *AIResearchFetcher {
	return &AIResearchFetcher{
		arxiv:           NewArxivFetcher(),
		semanticScholar: NewSemanticScholarFetcher(),
		huggingFace:     NewHuggingFaceFetcher(),
		papersWithCode:  NewPapersWithCodeFetcher(),
	}
}

// AIResearchResult holds results from AI research fetch operations
type AIResearchResult struct {
	Arxiv           int `json:"arxiv"`
	SemanticScholar int `json:"semantic_scholar"`
	HuggingFace     int `json:"huggingface"`
	PapersWithCode  int `json:"papers_with_code"`
}

// FetchArxiv fetches papers from arXiv
func (f *AIResearchFetcher) FetchArxiv(d *db.DB, query string, limit int) (int, error) {
	return f.arxiv.Fetch(d, query, limit)
}

// FetchSemanticScholar fetches papers from Semantic Scholar
func (f *AIResearchFetcher) FetchSemanticScholar(d *db.DB, query string, limit int) (int, error) {
	return f.semanticScholar.Fetch(d, query, limit)
}

// FetchHuggingFaceModels fetches models from HuggingFace Hub
func (f *AIResearchFetcher) FetchHuggingFaceModels(d *db.DB, query string, limit int) (int, error) {
	return f.huggingFace.Fetch(d, query, limit)
}

// FetchPapersWithCode fetches papers from Papers With Code
func (f *AIResearchFetcher) FetchPapersWithCode(d *db.DB, query string, limit int) (int, error) {
	return f.papersWithCode.Fetch(d, query, limit)
}

// FetchAll fetches from all sources with the same query
func (f *AIResearchFetcher) FetchAll(d *db.DB, query string, limit int) (*AIResearchResult, error) {
	result := &AIResearchResult{}

	// Fetch from each source (continue on errors)
	if count, err := f.FetchArxiv(d, query, limit); err == nil {
		result.Arxiv = count
	} else {
		fmt.Printf("arxiv fetch error: %v\n", err)
	}

	if count, err := f.FetchSemanticScholar(d, query, limit); err == nil {
		result.SemanticScholar = count
	} else {
		fmt.Printf("semantic scholar fetch error: %v\n", err)
	}

	if count, err := f.FetchHuggingFaceModels(d, query, limit); err == nil {
		result.HuggingFace = count
	} else {
		fmt.Printf("huggingface fetch error: %v\n", err)
	}

	if count, err := f.FetchPapersWithCode(d, query, limit); err == nil {
		result.PapersWithCode = count
	} else {
		fmt.Printf("papers with code fetch error: %v\n", err)
	}

	return result, nil
}

// AIResearchStats holds statistics from AI research sources
type AIResearchStats struct {
	Papers int `json:"papers"`
	Models int `json:"models"`
}

// GetStats returns statistics from AI research tables
func (f *AIResearchFetcher) GetStats(d *db.DB) (*AIResearchStats, error) {
	stats := &AIResearchStats{}

	var paperCount int
	row := d.QueryRow("SELECT COUNT(*) FROM ai_papers")
	if err := row.Scan(&paperCount); err == nil {
		stats.Papers = paperCount
	}

	var modelCount int
	row = d.QueryRow("SELECT COUNT(*) FROM ai_models")
	if err := row.Scan(&modelCount); err == nil {
		stats.Models = modelCount
	}

	return stats, nil
}

// AISearchResult holds results from a unified AI research search
type AISearchResult struct {
	Papers []AIPaperResult `json:"papers"`
	Models []AIModelResult `json:"models"`
}

// AIPaperResult represents a paper search result
type AIPaperResult struct {
	PaperID       string `json:"paper_id"`
	Source        string `json:"source"`
	Title         string `json:"title"`
	Abstract      string `json:"abstract,omitempty"`
	Authors       string `json:"authors"`
	Published     string `json:"published"`
	CitationCount int    `json:"citation_count"`
	PDFLink       string `json:"pdf_link,omitempty"`
}

// AIModelResult represents a model search result
type AIModelResult struct {
	ModelID     string `json:"model_id"`
	Author      string `json:"author"`
	Downloads   int    `json:"downloads"`
	Likes       int    `json:"likes"`
	PipelineTag string `json:"pipeline_tag"`
}

// Search performs a unified search across AI research sources
func (f *AIResearchFetcher) Search(d *db.DB, query string, limit int) (*AISearchResult, error) {
	if limit == 0 {
		limit = 20
	}

	result := &AISearchResult{
		Papers: []AIPaperResult{},
		Models: []AIModelResult{},
	}

	// Search papers
	paperRows, err := d.Query(`
		SELECT paper_id, source, title, abstract, authors, published, citation_count, pdf_link
		FROM ai_papers_fts
		JOIN ai_papers USING (paper_id)
		WHERE ai_papers_fts MATCH ?
		ORDER BY citation_count DESC
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer paperRows.Close()
		for paperRows.Next() {
			var p AIPaperResult
			var pdfLink sql.NullString
			if err := paperRows.Scan(&p.PaperID, &p.Source, &p.Title, &p.Abstract, &p.Authors, &p.Published, &p.CitationCount, &pdfLink); err == nil {
				if pdfLink.Valid {
					p.PDFLink = pdfLink.String
				}
				result.Papers = append(result.Papers, p)
			}
		}
	}

	// Search models
	modelRows, err := d.Query(`
		SELECT model_id, author, downloads, likes, pipeline_tag
		FROM ai_models_fts
		JOIN ai_models USING (model_id)
		WHERE ai_models_fts MATCH ?
		ORDER BY downloads DESC
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer modelRows.Close()
		for modelRows.Next() {
			var m AIModelResult
			if err := modelRows.Scan(&m.ModelID, &m.Author, &m.Downloads, &m.Likes, &m.PipelineTag); err == nil {
				result.Models = append(result.Models, m)
			}
		}
	}

	return result, nil
}

// Predefined query sets for AI/ML domains
var AIResearchQueries = map[string][]QueryConfig{
	"llm": {
		{Query: "large language model", Max: 50},
		{Query: "GPT transformer", Max: 50},
		{Query: "instruction tuning", Max: 30},
		{Query: "RLHF reinforcement learning human feedback", Max: 30},
		{Query: "prompt engineering", Max: 30},
	},
	"vision": {
		{Query: "vision transformer ViT", Max: 50},
		{Query: "image generation diffusion", Max: 50},
		{Query: "object detection YOLO", Max: 30},
		{Query: "image segmentation", Max: 30},
	},
	"multimodal": {
		{Query: "multimodal large language model", Max: 50},
		{Query: "vision language model", Max: 50},
		{Query: "CLIP contrastive", Max: 30},
		{Query: "text to image generation", Max: 30},
	},
	"agents": {
		{Query: "AI agent autonomous", Max: 50},
		{Query: "tool use language model", Max: 30},
		{Query: "planning reasoning LLM", Max: 30},
		{Query: "multi-agent collaboration", Max: 30},
	},
	"efficiency": {
		{Query: "model quantization", Max: 50},
		{Query: "knowledge distillation", Max: 30},
		{Query: "efficient transformer", Max: 30},
		{Query: "pruning neural network", Max: 30},
	},
}

// AvailableSources returns which AI research sources are available
// ALL AI RESEARCH APIS ARE KEY-FREE
func (f *AIResearchFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"arxiv":            true, // No key required
		"semantic_scholar": true, // No key required (rate-limited)
		"huggingface":      true, // No key required
		"papers_with_code": true, // No key required
	}
}

// EnsureAISchema ensures AI-related tables exist (already in main schema)
func EnsureAISchema(d *db.DB) error {
	// AI tables are created in db.EnsureSchema()
	// This is a no-op for backwards compatibility
	return nil
}
