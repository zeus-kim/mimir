package discovery

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Common RSS/Atom section paths to probe
var sectionPaths = []string{
	// Basic patterns
	"/rss", "/feed", "/feeds", "/rss.xml", "/feed.xml", "/atom.xml",
	// News sections
	"/rss/news", "/rss/world", "/rss/politics", "/rss/business",
	"/rss/technology", "/rss/science", "/rss/sports", "/rss/entertainment",
	"/rss/opinion", "/rss/lifestyle", "/rss/health", "/rss/culture",
	// Alternative patterns
	"/feeds/news", "/feeds/world", "/feeds/politics", "/feeds/business",
	"/section/news/rss", "/section/world/rss", "/section/politics/rss",
	// Category feeds
	"/category/news/feed", "/category/world/feed", "/category/politics/feed",
	// Arc CMS pattern
	"/arc/outboundfeeds/rss/", "/arcio/rss/",
}

// DomainCrawler discovers RSS feeds from a domain by crawling
type DomainCrawler struct {
	client     *http.Client
	maxWorkers int
	userAgent  string
}

// CrawlResult contains discovered feeds from a domain
type CrawlResult struct {
	Domain string   `json:"domain"`
	Feeds  []string `json:"feeds"`
	Errors []string `json:"errors,omitempty"`
}

// NewDomainCrawler creates a new crawler with sensible defaults
func NewDomainCrawler() *DomainCrawler {
	return &DomainCrawler{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		maxWorkers: 20,
		userAgent:  "Mozilla/5.0 (compatible; mimir-crawler/1.0)",
	}
}

// CrawlDomain discovers all RSS feeds from a domain
func (dc *DomainCrawler) CrawlDomain(domain string) (*CrawlResult, error) {
	result := &CrawlResult{
		Domain: domain,
		Feeds:  []string{},
	}

	baseURL := "https://" + domain
	foundFeeds := make(map[string]bool)
	var mu sync.Mutex

	addFeed := func(feed string) {
		mu.Lock()
		defer mu.Unlock()
		if !foundFeeds[feed] {
			foundFeeds[feed] = true
			result.Feeds = append(result.Feeds, feed)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stage 1: Extract RSS links from homepage HTML
	homepageFeeds := dc.extractRSSFromHTML(ctx, baseURL)
	for _, feed := range homepageFeeds {
		if dc.validateRSS(ctx, feed) {
			addFeed(feed)
		}
	}

	// Stage 2: Check common section paths concurrently
	var wg sync.WaitGroup
	sem := make(chan struct{}, dc.maxWorkers)

	for _, path := range sectionPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			feedURL := baseURL + p
			if dc.validateRSS(ctx, feedURL) {
				addFeed(feedURL)
			}
		}(path)
	}

	// Stage 3: Try with www prefix if domain doesn't have it
	if !strings.HasPrefix(domain, "www.") {
		wwwURL := "https://www." + domain
		wwwFeeds := dc.extractRSSFromHTML(ctx, wwwURL)
		for _, feed := range wwwFeeds {
			wg.Add(1)
			go func(f string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				if dc.validateRSS(ctx, f) {
					addFeed(f)
				}
			}(feed)
		}
	}

	wg.Wait()
	return result, nil
}

// CrawlDomains crawls multiple domains concurrently
func (dc *DomainCrawler) CrawlDomains(domains []string) []CrawlResult {
	results := make([]CrawlResult, len(domains))
	var wg sync.WaitGroup

	for i, domain := range domains {
		wg.Add(1)
		go func(idx int, d string) {
			defer wg.Done()
			result, err := dc.CrawlDomain(d)
			if err != nil {
				results[idx] = CrawlResult{
					Domain: d,
					Errors: []string{err.Error()},
				}
			} else {
				results[idx] = *result
			}
		}(i, domain)
	}

	wg.Wait()
	return results
}

// fetchPage retrieves page content with timeout
func (dc *DomainCrawler) fetchPage(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", dc.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")

	resp, err := dc.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	// Limit read to 512KB to prevent memory issues
	limited := io.LimitReader(resp.Body, 512*1024)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// Regex patterns for extracting RSS links from HTML
var (
	// <link rel="alternate" ... href="..." ... type="application/rss+xml">
	linkRelAltPattern = regexp.MustCompile(
		`(?i)<link[^>]+rel=["']alternate["'][^>]+href=["']([^"']+)["'][^>]*type=["']application/(rss|atom)\+xml["']`)

	// <link ... type="application/rss+xml" ... href="...">
	linkTypeFirstPattern = regexp.MustCompile(
		`(?i)<link[^>]+type=["']application/(rss|atom)\+xml["'][^>]+href=["']([^"']+)["']`)

	// href="...rss...xml" or href="...feed...xml" or href="...atom...xml"
	hrefRSSPattern = regexp.MustCompile(
		`(?i)href=["']([^"']*(?:rss|feed|atom)[^"']*\.xml)["']`)
)

// extractRSSFromHTML finds RSS/Atom feed links in HTML
func (dc *DomainCrawler) extractRSSFromHTML(ctx context.Context, pageURL string) []string {
	html, err := dc.fetchPage(ctx, pageURL)
	if err != nil || html == "" {
		return nil
	}

	feeds := make(map[string]bool)

	// Pattern 1: <link rel="alternate" href="..." type="application/rss+xml">
	for _, match := range linkRelAltPattern.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 {
			feedURL := resolveHref(pageURL, match[1])
			if feedURL != "" {
				feeds[feedURL] = true
			}
		}
	}

	// Pattern 2: <link type="application/rss+xml" href="...">
	for _, match := range linkTypeFirstPattern.FindAllStringSubmatch(html, -1) {
		if len(match) > 2 {
			feedURL := resolveHref(pageURL, match[2])
			if feedURL != "" {
				feeds[feedURL] = true
			}
		}
	}

	// Pattern 3: href="...rss/feed/atom...xml"
	for _, match := range hrefRSSPattern.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 {
			feedURL := resolveHref(pageURL, match[1])
			if feedURL != "" {
				feeds[feedURL] = true
			}
		}
	}

	result := make([]string, 0, len(feeds))
	for feed := range feeds {
		result = append(result, feed)
	}
	return result
}

// validateRSS checks if a URL returns valid RSS/Atom content
func (dc *DomainCrawler) validateRSS(ctx context.Context, feedURL string) bool {
	// Create a shorter timeout context for validation
	validateCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(validateCtx, "GET", feedURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", dc.userAgent)
	req.Header.Set("Accept", "application/rss+xml,application/atom+xml,application/xml,text/xml")

	resp, err := dc.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Read first 500 bytes to check for RSS/Atom markers
	buf := make([]byte, 500)
	n, err := resp.Body.Read(buf)
	if err != nil && n == 0 {
		return false
	}

	content := strings.ToLower(string(buf[:n]))
	return strings.Contains(content, "<rss") ||
		strings.Contains(content, "<feed") ||
		strings.Contains(content, "<channel")
}

// resolveHref resolves a potentially relative URL against a base
func resolveHref(baseURL, href string) string {
	if href == "" {
		return ""
	}

	// Already absolute
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	return base.ResolveReference(ref).String()
}

// DiscoverDomainFeeds is a convenience function for quick discovery
func DiscoverDomainFeeds(domain string) ([]string, error) {
	crawler := NewDomainCrawler()
	result, err := crawler.CrawlDomain(domain)
	if err != nil {
		return nil, err
	}
	return result.Feeds, nil
}
