package discovery

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// WebCrawler crawls websites to discover content, useful for sites without RSS
type WebCrawler struct {
	httpClient *http.Client
	userAgent  string
	timeout    time.Duration
}

// CrawledPage represents a discovered page
type CrawledPage struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Published string `json:"published,omitempty"`
	Source    string `json:"source"` // sitemap, link_crawl
	LastMod   string `json:"lastmod,omitempty"`
}

// NewWebCrawler creates a new web crawler
func NewWebCrawler() *WebCrawler {
	return &WebCrawler{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		timeout:    15 * time.Second,
	}
}

func (c *WebCrawler) Name() string { return "crawler" }

// Discover implements the Discoverer interface by crawling authority sites
func (c *WebCrawler) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	// Crawler doesn't directly discover, but can be used to crawl authority sites
	return []Source{}, nil
}

// CrawlSites crawls multiple sites for relevant content
func (c *WebCrawler) CrawlSites(sites []string, keywords []string, maxPages int) []CrawledPage {
	var allPages []CrawledPage
	var mu sync.Mutex
	var wg sync.WaitGroup

	pagesPerSite := maxPages / len(sites)
	if pagesPerSite < 1 {
		pagesPerSite = 1
	}

	for _, site := range sites {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			pages := c.CrawlSite(s, keywords, pagesPerSite)
			mu.Lock()
			allPages = append(allPages, pages...)
			mu.Unlock()
		}(site)
	}

	wg.Wait()

	if len(allPages) > maxPages {
		allPages = allPages[:maxPages]
	}

	return allPages
}

// CrawlSite crawls a single site for content
func (c *WebCrawler) CrawlSite(site string, keywords []string, maxPages int) []CrawledPage {
	// 1. Try sitemap first
	pages := c.crawlSitemap(site, keywords, maxPages)
	if len(pages) > 0 {
		return pages
	}

	// 2. Fallback to link crawling
	return c.crawlLinks(site, keywords, maxPages)
}

// crawlSitemap extracts URLs from sitemap.xml
func (c *WebCrawler) crawlSitemap(site string, keywords []string, maxPages int) []CrawledPage {
	var pages []CrawledPage

	sitemapURLs := []string{
		site + "/sitemap.xml",
		site + "/sitemap_index.xml",
		site + "/sitemap-news.xml",
		site + "/news-sitemap.xml",
	}

	for _, sitemapURL := range sitemapURLs {
		found := c.parseSitemap(sitemapURL, keywords, maxPages-len(pages))
		pages = append(pages, found...)
		if len(pages) >= maxPages {
			break
		}
	}

	return pages[:min(maxPages, len(pages))]
}

// Sitemap represents a sitemap.xml structure
type Sitemap struct {
	XMLName xml.Name      `xml:"urlset"`
	URLs    []SitemapURL  `xml:"url"`
	Sitemaps []SitemapRef `xml:"sitemap"`
}

type SitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Sitemaps []SitemapRef `xml:"sitemap"`
}

type SitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type SitemapRef struct {
	Loc string `xml:"loc"`
}

// parseSitemap parses a sitemap XML and extracts URLs
func (c *WebCrawler) parseSitemap(sitemapURL string, keywords []string, maxPages int) []CrawledPage {
	var pages []CrawledPage

	req, err := http.NewRequest("GET", sitemapURL, nil)
	if err != nil {
		return pages
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return pages
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return pages
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pages
	}

	// Try parsing as sitemap index first
	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal(body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		// It's a sitemap index, parse each referenced sitemap
		for _, ref := range sitemapIndex.Sitemaps[:min(3, len(sitemapIndex.Sitemaps))] {
			subPages := c.parseSitemap(ref.Loc, keywords, maxPages-len(pages))
			pages = append(pages, subPages...)
			if len(pages) >= maxPages {
				break
			}
		}
		return pages
	}

	// Parse as regular sitemap
	var sitemap Sitemap
	if err := xml.Unmarshal(body, &sitemap); err != nil {
		return pages
	}

	// News/article URL patterns
	newsPatterns := regexp.MustCompile(`(?i)/(news|article|press|release|update|blog|post)/`)

	for _, u := range sitemap.URLs {
		if len(pages) >= maxPages {
			break
		}

		urlLower := strings.ToLower(u.Loc)

		// Filter by keywords or news patterns
		matchesKeyword := false
		for _, kw := range keywords[:min(5, len(keywords))] {
			if strings.Contains(urlLower, strings.ToLower(kw)) {
				matchesKeyword = true
				break
			}
		}

		if !matchesKeyword && !newsPatterns.MatchString(u.Loc) {
			continue
		}

		pages = append(pages, CrawledPage{
			URL:     u.Loc,
			Source:  "sitemap",
			LastMod: u.LastMod,
		})
	}

	return pages
}

// crawlLinks extracts links from the main page
func (c *WebCrawler) crawlLinks(site string, keywords []string, maxPages int) []CrawledPage {
	var pages []CrawledPage

	req, err := http.NewRequest("GET", site, nil)
	if err != nil {
		return pages
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return pages
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return pages
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return pages
	}

	parsed, _ := url.Parse(site)
	domain := parsed.Host

	// News/article URL patterns
	newsPatterns := []string{
		"/news/", "/press/", "/article/", "/blog/",
		"/release/", "/update/", "/announcement/", "/post/",
	}

	links := extractAllLinks(doc)

	for _, link := range links {
		if len(pages) >= maxPages {
			break
		}

		// Resolve relative URLs
		linkURL := link.URL
		if !strings.HasPrefix(linkURL, "http") {
			if parsed != nil {
				ref, err := url.Parse(linkURL)
				if err == nil {
					linkURL = parsed.ResolveReference(ref).String()
				}
			}
		}

		// Only same domain
		linkParsed, err := url.Parse(linkURL)
		if err != nil || linkParsed.Host != domain {
			continue
		}

		// Match news patterns
		isNewsLink := false
		urlLower := strings.ToLower(linkURL)
		for _, pattern := range newsPatterns {
			if strings.Contains(urlLower, pattern) {
				isNewsLink = true
				break
			}
		}

		if isNewsLink {
			title := strings.TrimSpace(link.Text)
			if len(title) > 100 {
				title = title[:100]
			}

			pages = append(pages, CrawledPage{
				URL:    linkURL,
				Title:  title,
				Source: "link_crawl",
			})
		}
	}

	return pages
}

// FetchPageContent fetches and extracts content from a page
func (c *WebCrawler) FetchPageContent(pageURL string) (*CrawledPage, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	page := &CrawledPage{
		URL:    pageURL,
		Source: "fetch",
	}

	// Extract title
	page.Title = extractTitle(doc)

	// Extract content
	page.Summary = extractContent(doc)

	// Extract published date
	page.Published = extractPublishedDate(doc)

	return page, nil
}

// extractAllLinks extracts all links from HTML
func extractAllLinks(n *html.Node) []linkInfo {
	var links []linkInfo

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
					break
				}
			}
			if href != "" {
				text := extractText(n)
				links = append(links, linkInfo{URL: href, Text: text})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	return links
}

// extractTitle extracts the page title
func extractTitle(doc *html.Node) string {
	var title string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.FirstChild != nil {
				title = strings.TrimSpace(extractText(n))
				return
			}
			if n.Data == "h1" && title == "" {
				title = strings.TrimSpace(extractText(n))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	if len(title) > 200 {
		title = title[:200]
	}

	return title
}

// extractContent extracts main content from the page
func extractContent(doc *html.Node) string {
	var content string

	// Priority selectors for content
	selectors := []string{"article", "main", "div.content", "div.post", "div.entry"}

	for _, sel := range selectors {
		elem := findElementBySelector(doc, sel)
		if elem != nil {
			content = extractText(elem)
			break
		}
	}

	if content == "" {
		// Fallback: extract all paragraph text
		content = extractParagraphs(doc)
	}

	// Truncate to 500 chars
	if len(content) > 500 {
		content = content[:500]
	}

	return strings.TrimSpace(content)
}

// extractPublishedDate extracts the publication date
func extractPublishedDate(doc *html.Node) string {
	var date string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if date != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			var property, content string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "property":
					property = attr.Val
				case "content":
					content = attr.Val
				}
			}
			if property == "article:published_time" && content != "" {
				date = content
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return date
}

// findElementBySelector finds an element by simple selector (tag or tag.class)
func findElementBySelector(n *html.Node, selector string) *html.Node {
	parts := strings.SplitN(selector, ".", 2)
	tag := parts[0]
	class := ""
	if len(parts) > 1 {
		class = parts[1]
	}

	var result *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if result != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag {
			if class == "" {
				result = n
				return
			}
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, class) {
					result = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	return result
}

// extractText extracts text content from a node
func extractText(n *html.Node) string {
	var text strings.Builder

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
			text.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	return strings.TrimSpace(text.String())
}

// extractParagraphs extracts text from paragraph elements
func extractParagraphs(doc *html.Node) string {
	var paragraphs []string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(extractText(n))
			if text != "" {
				paragraphs = append(paragraphs, text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Join first 10 paragraphs
	if len(paragraphs) > 10 {
		paragraphs = paragraphs[:10]
	}

	return strings.Join(paragraphs, " ")
}
