package fetch

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

// WebSnippetResult represents a single web search result with snippet
type WebSnippetResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
	Error   string `json:"error,omitempty"`
}

// SnippetFetcher handles web search and snippet extraction
type SnippetFetcher struct {
	client    *http.Client
	userAgent string
}

// NewSnippetFetcher creates a new snippet fetcher
func NewSnippetFetcher() *SnippetFetcher {
	return &SnippetFetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		userAgent: defaultUserAgent,
	}
}

// doRequest performs an HTTP GET with proper headers
func (f *SnippetFetcher) doRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	return f.client.Do(req)
}

// SearchGoogle performs a Google search and returns results with snippets
func (f *SnippetFetcher) SearchGoogle(query string, numResults int) []WebSnippetResult {
	if numResults == 0 {
		numResults = 5
	}

	searchURL := "https://www.google.com/search?q=" + url.QueryEscape(query) +
		"&hl=ko&num=" + url.QueryEscape(string(rune('0'+numResults)))

	resp, err := f.doRequest(searchURL)
	if err != nil {
		return []WebSnippetResult{{Error: err.Error()}}
	}
	defer resp.Body.Close()

	return f.parseGoogleResults(resp.Body, numResults)
}

// parseGoogleResults extracts search results from Google HTML
func (f *SnippetFetcher) parseGoogleResults(body io.Reader, limit int) []WebSnippetResult {
	doc, err := html.Parse(body)
	if err != nil {
		return []WebSnippetResult{{Error: err.Error()}}
	}

	var results []WebSnippetResult

	// Find div.g elements (search result containers)
	var findResults func(*html.Node)
	findResults = func(n *html.Node) {
		if len(results) >= limit {
			return
		}

		if n.Type == html.ElementNode && n.Data == "div" {
			if hasClass(n, "g") {
				result := extractGoogleResult(n)
				if result.Title != "" && result.URL != "" {
					results = append(results, result)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findResults(c)
		}
	}
	findResults(doc)

	return results
}

// extractGoogleResult extracts title, snippet, and URL from a Google result div
func extractGoogleResult(n *html.Node) WebSnippetResult {
	var result WebSnippetResult

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.ElementNode {
			// Extract title from h3
			if node.Data == "h3" && result.Title == "" {
				result.Title = getTextContent(node)
			}
			// Extract link from first anchor
			if node.Data == "a" && result.URL == "" {
				for _, attr := range node.Attr {
					if attr.Key == "href" && strings.HasPrefix(attr.Val, "http") {
						result.URL = attr.Val
						break
					}
				}
			}
			// Extract snippet from VwiC3b class or data-sncf attribute
			if node.Data == "div" {
				if hasClass(node, "VwiC3b") || hasAttr(node, "data-sncf") {
					if result.Snippet == "" {
						result.Snippet = getTextContent(node)
					}
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)

	return result
}

// SearchNaver performs a Naver search and returns results with snippets
func (f *SnippetFetcher) SearchNaver(query string, numResults int) []WebSnippetResult {
	if numResults == 0 {
		numResults = 5
	}

	searchURL := "https://search.naver.com/search.naver?query=" + url.QueryEscape(query)

	resp, err := f.doRequest(searchURL)
	if err != nil {
		return []WebSnippetResult{{Error: err.Error()}}
	}
	defer resp.Body.Close()

	return f.parseNaverResults(resp.Body, numResults)
}

// parseNaverResults extracts search results from Naver HTML
func (f *SnippetFetcher) parseNaverResults(body io.Reader, limit int) []WebSnippetResult {
	doc, err := html.Parse(body)
	if err != nil {
		return []WebSnippetResult{{Error: err.Error()}}
	}

	var results []WebSnippetResult

	var findResults func(*html.Node)
	findResults = func(n *html.Node) {
		if len(results) >= limit {
			return
		}

		if n.Type == html.ElementNode && n.Data == "div" {
			if hasClass(n, "total_wrap") {
				result := extractNaverResult(n)
				if result.Title != "" {
					results = append(results, result)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findResults(c)
		}
	}
	findResults(doc)

	return results
}

// extractNaverResult extracts title, snippet, and URL from a Naver result div
func extractNaverResult(n *html.Node) WebSnippetResult {
	var result WebSnippetResult

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.ElementNode {
			// Extract title from total_tit class
			if node.Data == "a" && hasClass(node, "total_tit") {
				result.Title = strings.TrimSpace(getTextContent(node))
				for _, attr := range node.Attr {
					if attr.Key == "href" {
						result.URL = attr.Val
						break
					}
				}
			}
			// Extract snippet from total_dsc class
			if hasClass(node, "total_dsc") && result.Snippet == "" {
				result.Snippet = strings.TrimSpace(getTextContent(node))
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)

	return result
}

// FetchAndExtract fetches a URL and extracts relevant sentences based on query
func (f *SnippetFetcher) FetchAndExtract(pageURL string, query string, maxSentences int) []string {
	if maxSentences == 0 {
		maxSentences = 5
	}

	resp, err := f.doRequest(pageURL)
	if err != nil {
		return []string{"Error: " + err.Error()}
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return []string{"Error: " + err.Error()}
	}

	// Extract text content, skipping script/style/nav/footer/header
	text := extractSnippetText(doc)

	// Split into sentences
	sentenceRe := regexp.MustCompile(`[.!?。]\s+`)
	sentences := sentenceRe.Split(text, -1)

	// Get query keywords
	keywords := make(map[string]bool)
	for _, kw := range strings.Fields(strings.ToLower(query)) {
		keywords[kw] = true
	}

	// Score sentences by keyword matches
	type scored struct {
		score    int
		sentence string
	}
	var scoredSentences []scored

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) < 20 || len(sent) > 500 {
			continue
		}

		sentLower := strings.ToLower(sent)
		score := 0
		for kw := range keywords {
			if strings.Contains(sentLower, kw) {
				score++
			}
		}

		if score > 0 {
			scoredSentences = append(scoredSentences, scored{score, sent})
		}
	}

	// Sort by score descending
	sort.Slice(scoredSentences, func(i, j int) bool {
		return scoredSentences[i].score > scoredSentences[j].score
	})

	// Return top sentences
	var result []string
	for i := 0; i < len(scoredSentences) && i < maxSentences; i++ {
		result = append(result, scoredSentences[i].sentence)
	}

	return result
}

// extractSnippetText extracts text from HTML, skipping non-content elements
func extractSnippetText(n *html.Node) string {
	var sb strings.Builder

	// Tags to skip entirely
	skipTags := map[string]bool{
		"script": true, "style": true, "nav": true,
		"footer": true, "header": true, "noscript": true,
	}

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.ElementNode && skipTags[node.Data] {
			return
		}

		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)

	return sb.String()
}

// PerplexitySearch performs a Perplexity-style search with snippets
func (f *SnippetFetcher) PerplexitySearch(query string, engine string) string {
	var results []WebSnippetResult

	switch engine {
	case "naver":
		results = f.SearchNaver(query, 5)
	default:
		results = f.SearchGoogle(query, 5)
	}

	if len(results) == 0 {
		return "No results found"
	}

	var sb strings.Builder
	sb.WriteString("Search: ")
	sb.WriteString(query)
	sb.WriteString("\n\n")

	// Format top 3 results
	for i, r := range results {
		if i >= 3 {
			break
		}
		if r.Error != "" {
			continue
		}

		sb.WriteString("[")
		sb.WriteString(string(rune('1' + i)))
		sb.WriteString("] ")
		sb.WriteString(r.Title)
		sb.WriteString("\n")

		if r.Snippet != "" {
			snippet := r.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			sb.WriteString("    ")
			sb.WriteString(snippet)
			sb.WriteString("\n")
		}

		displayURL := r.URL
		if len(displayURL) > 60 {
			displayURL = displayURL[:60] + "..."
		}
		sb.WriteString("    ")
		sb.WriteString(displayURL)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// HighlightTerms highlights matching terms in text
func HighlightTerms(text string, query string, before string, after string) string {
	if before == "" {
		before = "**"
	}
	if after == "" {
		after = "**"
	}

	keywords := strings.Fields(strings.ToLower(query))
	result := text

	for _, kw := range keywords {
		// Case-insensitive replacement while preserving original case
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(kw))
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			return before + match + after
		})
	}

	return result
}

// Helper functions for HTML parsing

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func hasAttr(n *html.Node, name string) bool {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}

func getTextContent(n *html.Node) string {
	var sb strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(sb.String())
}
