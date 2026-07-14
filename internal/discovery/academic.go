package discovery

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// AcademicDiscoverer discovers academic sources from OpenAlex, arXiv, Semantic Scholar
type AcademicDiscoverer struct {
	httpClient *http.Client
	userAgent  string
}

// NewAcademicDiscoverer creates a new academic source discoverer
func NewAcademicDiscoverer() *AcademicDiscoverer {
	return &AcademicDiscoverer{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		userAgent:  "Mimir/1.0 (mailto:contact@mimir.local)",
	}
}

func (d *AcademicDiscoverer) Name() string { return "academic" }

// Discover finds academic sources for the given topic and keywords
func (d *AcademicDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	var sources []Source

	// Use topic + keywords, limit to first 3 keywords
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 3 {
		searchTerms = searchTerms[:3]
	}

	for _, kw := range searchTerms {
		encoded := url.QueryEscape(kw)

		// OpenAlex API
		sources = append(sources, Source{
			URL:         fmt.Sprintf("https://api.openalex.org/works?search=%s&per-page=20", encoded),
			Title:       fmt.Sprintf("OpenAlex: %s", kw),
			Description: "OpenAlex academic database search",
			Type:        "api",
			Language:    "en",
		})

		// arXiv RSS/API
		sources = append(sources, Source{
			URL:         fmt.Sprintf("http://export.arxiv.org/api/query?search_query=all:%s&max_results=20", encoded),
			Title:       fmt.Sprintf("arXiv: %s", kw),
			Description: "arXiv preprint server",
			Type:        "api",
			Language:    "en",
		})
	}

	// Semantic Scholar (first 2 keywords)
	semanticTerms := searchTerms
	if len(semanticTerms) > 2 {
		semanticTerms = semanticTerms[:2]
	}

	for _, kw := range semanticTerms {
		encoded := url.QueryEscape(kw)
		sources = append(sources, Source{
			URL:         fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/search?query=%s&limit=20", encoded),
			Title:       fmt.Sprintf("Semantic Scholar: %s", kw),
			Description: "Semantic Scholar academic search",
			Type:        "api",
			Language:    "en",
		})
	}

	// PubMed RSS for biomedical topics
	for _, kw := range searchTerms[:min(2, len(searchTerms))] {
		encoded := url.QueryEscape(kw)
		sources = append(sources, Source{
			URL:         fmt.Sprintf("https://pubmed.ncbi.nlm.nih.gov/rss/search/1?term=%s&limit=20", encoded),
			Title:       fmt.Sprintf("PubMed: %s", kw),
			Description: "PubMed biomedical literature",
			Type:        "rss",
			Language:    "en",
		})
	}

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// AcademicSource represents an academic paper/work
type AcademicSource struct {
	DOI         string   `json:"doi,omitempty"`
	Title       string   `json:"title"`
	Authors     []string `json:"authors,omitempty"`
	Abstract    string   `json:"abstract,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	Journal     string   `json:"journal,omitempty"`
	URL         string   `json:"url"`
	Source      string   `json:"source"` // openalex, arxiv, semantic_scholar, pubmed
	Citations   int      `json:"citations,omitempty"`
}
