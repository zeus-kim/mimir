package fetch

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebContentFetcher fetches and parses web content from sites without RSS feeds.
// Useful for government, legal, and other institutional sites.
type WebContentFetcher struct {
	Client  *http.Client
	Headers map[string]string
}

// WebPage represents extracted content from a web page.
type WebPage struct {
	URL         string
	Title       string
	Summary     string
	Author      string
	PublishedAt int64
	SourceName  string
	SourceURL   string
}

// NewWebContentFetcher creates a new fetcher with sensible defaults.
func NewWebContentFetcher() *WebContentFetcher {
	return &WebContentFetcher{
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		Headers: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			"Accept-Language": "ko-KR,ko;q=0.9,en;q=0.8",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		},
	}
}

// FetchPage fetches and extracts content from a single URL.
func (f *WebContentFetcher) FetchPage(pageURL string) (*WebPage, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range f.Headers {
		req.Header.Set(k, v)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	page := &WebPage{
		URL:         pageURL,
		Title:       extractTitle(doc),
		Summary:     extractContent(doc),
		Author:      extractAuthor(doc),
		PublishedAt: extractDate(doc, string(body)),
	}

	if page.Title == "" {
		return nil, nil
	}

	// Truncate summary
	if len(page.Summary) > 500 {
		page.Summary = page.Summary[:500]
	}

	return page, nil
}

// CrawlSite crawls a site looking for article links and fetches them.
func (f *WebContentFetcher) CrawlSite(baseURL string, keywords []string, maxPages int) ([]*WebPage, error) {
	var pages []*WebPage

	// Try common news/article paths
	newsPaths := []string{
		"/news", "/press", "/notice", "/board", "/bbs",
		"/portal/news", "/article", "/articles",
	}

	for _, path := range newsPaths {
		listURL, err := url.JoinPath(baseURL, path)
		if err != nil {
			continue
		}

		links, err := f.extractArticleLinks(listURL, baseURL)
		if err != nil || len(links) == 0 {
			continue
		}

		for _, link := range links {
			if len(pages) >= maxPages {
				break
			}
			page, err := f.FetchPage(link)
			if err == nil && page != nil {
				pages = append(pages, page)
			}
		}

		if len(pages) > 0 {
			break
		}
	}

	// Try sitemap if no results
	if len(pages) == 0 {
		sitemapPages, _ := f.crawlSitemap(baseURL, keywords, maxPages)
		pages = append(pages, sitemapPages...)
	}

	// Try main page links as fallback
	if len(pages) == 0 {
		mainPages, _ := f.crawlMainPage(baseURL, keywords, maxPages)
		pages = append(pages, mainPages...)
	}

	// Apply keyword filter if provided
	if len(keywords) > 0 {
		pages = filterRelevant(pages, keywords)
	}

	if len(pages) > maxPages {
		pages = pages[:maxPages]
	}

	return pages, nil
}

// extractArticleLinks extracts article-like links from a page.
func (f *WebContentFetcher) extractArticleLinks(listURL, baseURL string) ([]string, error) {
	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range f.Headers {
		req.Header.Set(k, v)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	baseURLParsed, _ := url.Parse(baseURL)
	baseDomain := baseURLParsed.Host

	var links []string
	seen := make(map[string]bool)

	articlePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)/(view|read|detail|article|news)[?/]`),
		regexp.MustCompile(`(?i)(seq|idx|id|no)=\d+`),
	}

	var extractLinks func(*html.Node)
	extractLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := attr.Val
					resolvedURL := resolveURL(baseURL, href)
					if resolvedURL == "" {
						continue
					}

					parsedURL, err := url.Parse(resolvedURL)
					if err != nil || parsedURL.Host != baseDomain {
						continue
					}

					// Check if it looks like an article
					for _, pattern := range articlePatterns {
						if pattern.MatchString(resolvedURL) {
							if !seen[resolvedURL] {
								seen[resolvedURL] = true
								links = append(links, resolvedURL)
							}
							break
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractLinks(c)
		}
	}
	extractLinks(doc)

	return links, nil
}

// crawlSitemap attempts to crawl sitemap.xml for URLs.
func (f *WebContentFetcher) crawlSitemap(baseURL string, keywords []string, maxPages int) ([]*WebPage, error) {
	var pages []*WebPage

	sitemapURLs := []string{
		baseURL + "/sitemap.xml",
		baseURL + "/sitemap_index.xml",
	}

	locPattern := regexp.MustCompile(`<loc>([^<]+)</loc>`)
	newsPattern := regexp.MustCompile(`(?i)/(news|article|press|view)/`)

	for _, sitemapURL := range sitemapURLs {
		req, err := http.NewRequest("GET", sitemapURL, nil)
		if err != nil {
			continue
		}

		for k, v := range f.Headers {
			req.Header.Set(k, v)
		}

		resp, err := f.Client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		matches := locPattern.FindAllStringSubmatch(string(body), -1)

		for _, match := range matches {
			if len(pages) >= maxPages {
				break
			}

			pageURL := match[1]

			// Filter by keywords or news patterns
			if len(keywords) > 0 {
				keywordMatch := false
				urlLower := strings.ToLower(pageURL)
				for _, kw := range keywords[:min(len(keywords), 3)] {
					if strings.Contains(urlLower, strings.ToLower(kw)) {
						keywordMatch = true
						break
					}
				}
				if !keywordMatch && !newsPattern.MatchString(pageURL) {
					continue
				}
			}

			page, err := f.FetchPage(pageURL)
			if err == nil && page != nil {
				pages = append(pages, page)
			}
		}

		if len(pages) > 0 {
			break
		}
	}

	return pages, nil
}

// crawlMainPage extracts links from the main page.
func (f *WebContentFetcher) crawlMainPage(baseURL string, keywords []string, maxPages int) ([]*WebPage, error) {
	links, err := f.extractArticleLinks(baseURL, baseURL)
	if err != nil {
		return nil, err
	}

	var pages []*WebPage
	for _, link := range links {
		if len(pages) >= maxPages {
			break
		}
		page, err := f.FetchPage(link)
		if err == nil && page != nil {
			pages = append(pages, page)
		}
	}

	return pages, nil
}

// extractTitle extracts the page title from HTML.
func extractTitle(doc *html.Node) string {
	// Try og:title meta tag first
	if ogTitle := findMetaContent(doc, "og:title", "property"); ogTitle != "" {
		return strings.TrimSpace(ogTitle)
	}

	// Try h1
	if h1 := findFirstElement(doc, "h1"); h1 != "" {
		return strings.TrimSpace(h1)
	}

	// Try title tag
	if title := findFirstElement(doc, "title"); title != "" {
		return strings.TrimSpace(title)
	}

	return ""
}

// extractContent extracts the main content from HTML.
func extractContent(doc *html.Node) string {
	// Try semantic content areas
	selectors := []string{"article", "main"}
	for _, sel := range selectors {
		if content := findElementText(doc, sel); content != "" {
			return truncateText(content, 2000)
		}
	}

	// Try common class/id patterns
	classPatterns := []string{"content", "post", "view", "article-body"}
	for _, pattern := range classPatterns {
		if content := findByClassOrID(doc, pattern); content != "" {
			return truncateText(content, 2000)
		}
	}

	// Fallback: collect all <p> text
	return truncateText(collectParagraphs(doc), 2000)
}

// extractAuthor extracts author information.
func extractAuthor(doc *html.Node) string {
	// Try meta tags
	authorMeta := findMetaContent(doc, "author", "name")
	if authorMeta != "" {
		return strings.TrimSpace(authorMeta)
	}

	// Try article:author
	if author := findMetaContent(doc, "article:author", "property"); author != "" {
		return strings.TrimSpace(author)
	}

	return ""
}

// extractDate extracts publication date from HTML.
func extractDate(doc *html.Node, rawHTML string) int64 {
	// Try article:published_time meta
	if dateStr := findMetaContent(doc, "article:published_time", "property"); dateStr != "" {
		if t := parseISODate(dateStr); t > 0 {
			return t
		}
	}

	// Try date patterns in text
	datePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d{4})[.-](\d{1,2})[.-](\d{1,2})`),
	}

	for _, pattern := range datePatterns {
		if match := pattern.FindStringSubmatch(rawHTML); match != nil {
			year, month, day := atoi(match[1]), atoi(match[2]), atoi(match[3])
			if year >= 2000 && year <= 2100 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
				t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
				return t.Unix()
			}
		}
	}

	return time.Now().Unix()
}

// filterRelevant filters pages by keyword relevance.
func filterRelevant(pages []*WebPage, keywords []string) []*WebPage {
	if len(keywords) == 0 {
		return pages
	}

	var filtered []*WebPage
	keywordsLower := make([]string, len(keywords))
	for i, kw := range keywords {
		keywordsLower[i] = strings.ToLower(kw)
	}

	for _, page := range pages {
		text := strings.ToLower(page.Title + " " + page.Summary)
		for _, kw := range keywordsLower[:min(len(keywordsLower), 10)] {
			if strings.Contains(text, kw) {
				filtered = append(filtered, page)
				break
			}
		}
	}

	return filtered
}

// Helper functions

func findMetaContent(doc *html.Node, name, attr string) string {
	var result string
	var find func(*html.Node)
	find = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var nameVal, contentVal string
			for _, a := range n.Attr {
				if a.Key == attr && a.Val == name {
					nameVal = a.Val
				}
				if a.Key == "name" && a.Val == name {
					nameVal = a.Val
				}
				if a.Key == "content" {
					contentVal = a.Val
				}
			}
			if nameVal != "" && contentVal != "" {
				result = contentVal
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	return result
}

func findFirstElement(doc *html.Node, tag string) string {
	var result string
	var find func(*html.Node)
	find = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			result = collectText(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	return result
}

func findElementText(doc *html.Node, tag string) string {
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == tag {
			// Remove script, style, nav, header, footer
			removeNoise(n)
			return collectText(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := find(c); result != "" {
				return result
			}
		}
		return ""
	}
	return find(doc)
}

func findByClassOrID(doc *html.Node, pattern string) string {
	var find func(*html.Node) string
	find = func(n *html.Node) string {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if (attr.Key == "class" || attr.Key == "id") && strings.Contains(strings.ToLower(attr.Val), pattern) {
					removeNoise(n)
					return collectText(n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := find(c); result != "" {
				return result
			}
		}
		return ""
	}
	return find(doc)
}

func collectParagraphs(doc *html.Node) string {
	var paragraphs []string
	count := 0
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if count >= 10 {
			return
		}
		if n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(collectText(n))
			if text != "" {
				paragraphs = append(paragraphs, text)
				count++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(doc)
	return strings.Join(paragraphs, " ")
}

func collectText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return sb.String()
}

func removeNoise(n *html.Node) {
	noiseElements := map[string]bool{
		"script": true, "style": true, "nav": true,
		"header": true, "footer": true, "aside": true,
	}

	var toRemove []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && noiseElements[c.Data] {
			toRemove = append(toRemove, c)
		} else {
			removeNoise(c)
		}
	}

	for _, child := range toRemove {
		n.RemoveChild(child)
	}
}

func resolveURL(base, href string) string {
	if href == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	refURL, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(refURL).String()
}

func truncateText(s string, maxLen int) string {
	// Normalize whitespace
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func parseISODate(s string) int64 {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	s = strings.ReplaceAll(s, "Z", "+00:00")
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func atoi(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
