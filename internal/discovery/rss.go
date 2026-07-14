package discovery

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DiscoveryMode controls depth vs speed tradeoff
type DiscoveryMode int

const (
	// ModeFast uses only top 5 patterns, 3s timeout - for bulk discovery
	ModeFast DiscoveryMode = iota
	// ModeEnhanced uses 20+ patterns, HTML parsing, sitemap - for quality
	ModeEnhanced
)

// FeedSource indicates how the feed was discovered
type FeedSource string

const (
	SourcePattern FeedSource = "pattern"
	SourceHTML    FeedSource = "html"
	SourceSitemap FeedSource = "sitemap"
)

// DiscoveredFeed represents a validated RSS/Atom feed
type DiscoveredFeed struct {
	Domain    string     `json:"domain"`
	URL       string     `json:"url"`
	Title     string     `json:"title"`
	ItemCount int        `json:"item_count"`
	Source    FeedSource `json:"source"`
	Language  string     `json:"language,omitempty"`
	Country   string     `json:"country,omitempty"`
}

// DomainInput represents a domain to check for RSS feeds
type DomainInput struct {
	Domain   string `json:"domain"`
	Language string `json:"language"`
	Country  string `json:"country"`
}

// RSSDiscoveryResult contains all feeds found for a domain
type RSSDiscoveryResult struct {
	Domain string           `json:"domain"`
	Feeds  []DiscoveredFeed `json:"feeds"`
	Error  string           `json:"error,omitempty"`
}

// Top 5 most effective RSS paths (for fast mode)
var rssPathsFast = []string{
	"/feed",
	"/rss",
	"/rss.xml",
	"/feed.xml",
	"/atom.xml",
}

// Extended RSS paths (for enhanced mode)
var rssPathsEnhanced = []string{
	// Basic
	"/feed", "/rss", "/rss.xml", "/feed.xml", "/atom.xml",
	// WordPress
	"/?feed=rss2", "/?feed=atom", "/wp-feed.php", "/comments/feed/",
	// Blogger/Blogspot
	"/feeds/posts/default", "/feeds/comments/default",
	// Arc CMS (NYT, newspapers)
	"/arc/outboundfeeds/rss/", "/arcio/rss/",
	// General patterns
	"/syndication/rss", "/export/rss", "/rss/index.xml", "/index.rss",
	"/rss/all", "/feeds/", "/feed/rss2/",
	// Section-based
	"/rss/news", "/rss/world", "/rss/politics", "/rss/business",
	"/rss/technology", "/rss/sports", "/rss/entertainment",
	"/feed/news", "/feed/world", "/feed/tech",
}

// RSSFinder discovers RSS feeds from domains
type RSSFinder struct {
	Mode        DiscoveryMode
	Timeout     time.Duration
	Concurrency int
	UserAgent   string
	httpClient  *http.Client
}

// NewRSSFinder creates an RSS discovery instance
func NewRSSFinder(mode DiscoveryMode) *RSSFinder {
	timeout := 5 * time.Second
	if mode == ModeFast {
		timeout = 3 * time.Second
	}

	return &RSSFinder{
		Mode:        mode,
		Timeout:     timeout,
		Concurrency: 100,
		UserAgent:   "Mozilla/5.0 (compatible; mimir-mcp/1.0)",
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 3,
				IdleConnTimeout:     90 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// DiscoverDomain finds all RSS feeds for a single domain
func (f *RSSFinder) DiscoverDomain(ctx context.Context, domain string) RSSDiscoveryResult {
	return f.DiscoverDomainWithInput(ctx, DomainInput{Domain: domain})
}

// DiscoverDomainWithInput finds RSS feeds with language/country metadata
func (f *RSSFinder) DiscoverDomainWithInput(ctx context.Context, input DomainInput) RSSDiscoveryResult {
	result := RSSDiscoveryResult{Domain: input.Domain}
	foundURLs := make(map[string]bool)

	paths := rssPathsFast
	if f.Mode == ModeEnhanced {
		paths = rssPathsEnhanced
	}

	// Try HTTPS first, fall back to HTTP
	for _, scheme := range []string{"https", "http"} {
		baseURL := scheme + "://" + input.Domain

		// 1. Check known RSS paths
		for _, path := range paths {
			feedURL := baseURL + path
			if foundURLs[feedURL] {
				continue
			}

			feed := f.checkRSSURL(ctx, feedURL)
			if feed != nil {
				feed.Domain = input.Domain
				feed.Source = SourcePattern
				feed.Language = input.Language
				feed.Country = input.Country
				foundURLs[feed.URL] = true
				result.Feeds = append(result.Feeds, *feed)
			}
		}

		// Enhanced mode: also check HTML and sitemap
		if f.Mode == ModeEnhanced {
			// 2. Extract from HTML <link> tags
			htmlFeeds := f.extractFromHTML(ctx, baseURL)
			for _, feedURL := range htmlFeeds {
				if foundURLs[feedURL] {
					continue
				}
				feed := f.checkRSSURL(ctx, feedURL)
				if feed != nil {
					feed.Domain = input.Domain
					feed.Source = SourceHTML
					feed.Language = input.Language
					feed.Country = input.Country
					foundURLs[feed.URL] = true
					result.Feeds = append(result.Feeds, *feed)
				}
			}

			// 3. Check sitemap.xml
			sitemapFeeds := f.checkSitemap(ctx, baseURL)
			for _, feedURL := range sitemapFeeds {
				if foundURLs[feedURL] {
					continue
				}
				feed := f.checkRSSURL(ctx, feedURL)
				if feed != nil {
					feed.Domain = input.Domain
					feed.Source = SourceSitemap
					feed.Language = input.Language
					feed.Country = input.Country
					foundURLs[feed.URL] = true
					result.Feeds = append(result.Feeds, *feed)
				}
			}
		}

		// If we found feeds on HTTPS, skip HTTP
		if len(result.Feeds) > 0 {
			break
		}
	}

	if len(result.Feeds) == 0 {
		result.Error = "no feeds found"
	}

	return result
}

// DiscoverBatch processes multiple domains concurrently
func (f *RSSFinder) DiscoverBatch(ctx context.Context, domains []DomainInput) []RSSDiscoveryResult {
	results := make([]RSSDiscoveryResult, len(domains))
	sem := make(chan struct{}, f.Concurrency)
	var wg sync.WaitGroup

	for i, domain := range domains {
		wg.Add(1)
		go func(idx int, d DomainInput) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = f.DiscoverDomainWithInput(ctx, d)
		}(i, domain)
	}

	wg.Wait()
	return results
}

// checkRSSURL validates if a URL is a valid RSS/Atom feed
func (f *RSSFinder) checkRSSURL(ctx context.Context, feedURL string) *DiscoveredFeed {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// Read only first 2KB for quick validation
	content := make([]byte, 2048)
	n, err := io.ReadFull(resp.Body, content)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil
	}
	content = content[:n]

	text := string(content)

	// Quick check for RSS/Atom markers
	if !strings.Contains(text, "<rss") &&
		!strings.Contains(text, "<feed") &&
		!strings.Contains(text, "<channel") {
		return nil
	}

	// Extract title
	title := rssFeedExtractTitle(text)

	// Count items (estimate from first 2KB)
	itemCount := strings.Count(text, "<item") + strings.Count(text, "<entry")
	if itemCount == 0 {
		itemCount = 1 // Assume at least 1 if valid XML
	}

	// Use final URL after redirects
	finalURL := resp.Request.URL.String()

	return &DiscoveredFeed{
		URL:       finalURL,
		Title:     title,
		ItemCount: itemCount,
	}
}

// extractFromHTML parses HTML page for RSS link tags
func (f *RSSFinder) extractFromHTML(ctx context.Context, baseURL string) []string {
	var feeds []string

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		return feeds
	}
	req.Header.Set("User-Agent", f.UserAgent)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return feeds
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return feeds
	}

	// Read HTML (limit to 100KB to avoid huge pages)
	content := make([]byte, 100*1024)
	n, _ := io.ReadFull(resp.Body, content)
	html := string(content[:n])

	// Find <link rel="alternate" type="application/rss+xml" href="...">
	// Handle both attribute orders
	linkPatterns := []*regexp.Regexp{
		regexp.MustCompile(`<link[^>]+href=["']([^"']+)["'][^>]+type=["']application/(rss|atom)\+xml["']`),
		regexp.MustCompile(`<link[^>]+type=["']application/(rss|atom)\+xml["'][^>]+href=["']([^"']+)["']`),
	}

	for _, re := range linkPatterns {
		matches := re.FindAllStringSubmatch(html, -1)
		for _, m := range matches {
			var href string
			// Extract href from matches
			if len(m) >= 2 {
				// Check which group has the URL
				for _, g := range m[1:] {
					if strings.HasPrefix(g, "http") || strings.HasPrefix(g, "/") {
						href = g
						break
					}
				}
			}
			if href != "" {
				feeds = append(feeds, resolveURL(baseURL, href))
			}
		}
	}

	// Also find href with rss/feed/atom keywords ending in .xml
	xmlPattern := regexp.MustCompile(`href=["']([^"']*(?:rss|feed|atom)[^"']*\.xml)["']`)
	matches := xmlPattern.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			feeds = append(feeds, resolveURL(baseURL, m[1]))
		}
	}

	return uniqueStrings(feeds)
}

// checkSitemap looks for feed URLs in sitemap.xml
func (f *RSSFinder) checkSitemap(ctx context.Context, baseURL string) []string {
	var feeds []string

	sitemapURL := baseURL + "/sitemap.xml"
	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return feeds
	}
	req.Header.Set("User-Agent", f.UserAgent)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return feeds
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return feeds
	}

	// Read sitemap (limit to 1MB)
	content := make([]byte, 1024*1024)
	n, _ := io.ReadFull(resp.Body, content)
	text := string(content[:n])

	// Find URLs containing rss/feed/atom
	locPattern := regexp.MustCompile(`<loc>([^<]*(?:rss|feed|atom)[^<]*)</loc>`)
	matches := locPattern.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			feeds = append(feeds, m[1])
		}
	}

	return uniqueStrings(feeds)
}

// rssFeedExtractTitle extracts the <title> from RSS/Atom content
func rssFeedExtractTitle(content string) string {
	re := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
	match := re.FindStringSubmatch(content)
	if len(match) >= 2 {
		title := strings.TrimSpace(match[1])
		// Handle CDATA
		title = strings.TrimPrefix(title, "<![CDATA[")
		title = strings.TrimSuffix(title, "]]>")
		if len(title) > 100 {
			title = title[:100]
		}
		return title
	}
	return ""
}

// Note: resolveURL is defined in discovery.go

// uniqueStrings removes duplicates from a slice
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// QuickValidateFeed checks if a single URL is a valid RSS feed
func QuickValidateFeed(ctx context.Context, feedURL string) *DiscoveredFeed {
	finder := NewRSSFinder(ModeFast)
	return finder.checkRSSURL(ctx, feedURL)
}

// ScoreFeed assigns a quality score to a discovered feed
func ScoreFeed(feed DiscoveredFeed) float64 {
	score := 0.0

	// More items is better
	if feed.ItemCount >= 10 {
		score += 0.3
	} else if feed.ItemCount >= 5 {
		score += 0.2
	} else if feed.ItemCount >= 1 {
		score += 0.1
	}

	// Having a title is good
	if feed.Title != "" {
		score += 0.2
	}

	// HTTPS is preferred
	if strings.HasPrefix(feed.URL, "https://") {
		score += 0.1
	}

	// Source-based scoring
	switch feed.Source {
	case SourceHTML:
		score += 0.2 // HTML autodiscovery is usually accurate
	case SourcePattern:
		score += 0.15
	case SourceSitemap:
		score += 0.1
	}

	// Prefer main feeds over section feeds
	lowerURL := strings.ToLower(feed.URL)
	if strings.HasSuffix(lowerURL, "/feed") ||
		strings.HasSuffix(lowerURL, "/rss") ||
		strings.HasSuffix(lowerURL, "/feed.xml") ||
		strings.HasSuffix(lowerURL, "/rss.xml") {
		score += 0.15
	}

	return score
}

// RankFeeds sorts feeds by quality score (descending)
func RankFeeds(feeds []DiscoveredFeed) []DiscoveredFeed {
	// Score all feeds
	type scoredFeed struct {
		feed  DiscoveredFeed
		score float64
	}

	scored := make([]scoredFeed, len(feeds))
	for i, f := range feeds {
		scored[i] = scoredFeed{feed: f, score: ScoreFeed(f)}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	result := make([]DiscoveredFeed, len(scored))
	for i, sf := range scored {
		result[i] = sf.feed
	}
	return result
}
