package discovery

import (
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

// RSSDiscoverer discovers RSS feeds via Feedspot, web search, and site crawling
type RSSDiscoverer struct {
	httpClient *http.Client
	userAgent  string
	timeout    time.Duration
}

// NewRSSDiscoverer creates a new RSS feed discoverer
func NewRSSDiscoverer() *RSSDiscoverer {
	return &RSSDiscoverer{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		timeout:    10 * time.Second,
	}
}

func (d *RSSDiscoverer) Name() string { return "rss" }

// Discover finds RSS feeds for the given topic and keywords
func (d *RSSDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	return d.DiscoverWithLanguages(topic, keywords, []string{"en"}, nil, limit)
}

// DiscoverWithLanguages discovers RSS feeds with language and authority filters
func (d *RSSDiscoverer) DiscoverWithLanguages(topic string, keywords, languages, authorities []string, limit int) ([]Source, error) {
	var sources []Source
	seen := make(map[string]bool)
	var mu sync.Mutex

	// Search terms
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 3 {
		searchTerms = searchTerms[:3]
	}

	var wg sync.WaitGroup

	// 1. Feedspot directory (synchronous - quick)
	for _, term := range searchTerms {
		feeds := d.searchFeedspot(term, limit/3)
		for _, feed := range feeds {
			if !seen[feed.URL] {
				seen[feed.URL] = true
				sources = append(sources, feed)
			}
		}
	}

	// 2. Web search (concurrent)
	for _, term := range searchTerms {
		for _, lang := range languages[:min(2, len(languages))] {
			wg.Add(1)
			go func(t, l string) {
				defer wg.Done()
				feeds := d.searchWeb(t, l, limit/4)
				mu.Lock()
				for _, feed := range feeds {
					if !seen[feed.URL] {
						seen[feed.URL] = true
						sources = append(sources, feed)
					}
				}
				mu.Unlock()
			}(term, lang)
		}
	}

	// 3. Authority site RSS discovery (concurrent)
	for _, domain := range authorities[:min(5, len(authorities))] {
		wg.Add(1)
		go func(dom string) {
			defer wg.Done()
			siteURL := "https://" + dom
			feeds := d.discoverOnSite(siteURL)
			mu.Lock()
			for _, feed := range feeds {
				if !seen[feed.URL] {
					seen[feed.URL] = true
					sources = append(sources, feed)
				}
			}
			mu.Unlock()
		}(domain)
	}

	wg.Wait()

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// searchFeedspot searches Feedspot's curated feed directory
func (d *RSSDiscoverer) searchFeedspot(keyword string, limit int) []Source {
	var sources []Source

	slug := strings.ReplaceAll(strings.ToLower(keyword), " ", "_")
	feedspotURL := fmt.Sprintf("https://rss.feedspot.com/%s_rss_feeds/", slug)

	req, err := http.NewRequest("GET", feedspotURL, nil)
	if err != nil {
		return sources
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return sources
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return sources
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sources
	}

	// Extract feed URLs from the page
	feedPattern := regexp.MustCompile(`href="(https?://[^"]+(?:rss|feed|xml)[^"]*)"`)
	matches := feedPattern.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		feedURL := match[1]

		if d.looksLikeFeed(feedURL) {
			parsed, _ := url.Parse(feedURL)
			title := parsed.Host
			if parsed != nil && parsed.Host != "" {
				title = parsed.Host
			}

			sources = append(sources, Source{
				URL:         feedURL,
				Title:       title,
				Description: fmt.Sprintf("Feedspot discovery for %s", keyword),
				Type:        "rss",
			})

			if len(sources) >= limit {
				break
			}
		}
	}

	return sources
}

// searchWeb uses DuckDuckGo to find RSS feeds
func (d *RSSDiscoverer) searchWeb(keyword, lang string, limit int) []Source {
	var sources []Source

	queries := []string{
		keyword + " RSS feed",
		keyword + " blog RSS",
		"best " + keyword + " feeds",
	}

	for _, query := range queries[:2] {
		searchURL := "https://lite.duckduckgo.com/lite/"

		data := url.Values{
			"q":  {query},
			"kl": {fmt.Sprintf("%s-%s", lang, lang)},
		}

		req, err := http.NewRequest("POST", searchURL, strings.NewReader(data.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", d.userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := d.httpClient.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		// Extract URLs from search results
		urlPattern := regexp.MustCompile(`href="(https?://[^"]+)"`)
		matches := urlPattern.FindAllStringSubmatch(string(body), -1)

		// Check each URL for RSS feeds
		for _, match := range matches[:min(5, len(matches))] {
			if len(match) < 2 {
				continue
			}
			siteURL := match[1]
			if strings.Contains(siteURL, "duckduckgo") {
				continue
			}

			// Try to discover RSS on this site
			siteFeeds := d.discoverOnSite(siteURL)
			sources = append(sources, siteFeeds...)
		}
	}

	return sources[:min(limit, len(sources))]
}

// discoverOnSite finds RSS feeds on a specific site
func (d *RSSDiscoverer) discoverOnSite(siteURL string) []Source {
	var sources []Source

	parsed, err := url.Parse(siteURL)
	if err != nil {
		return sources
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	// Common RSS paths
	commonPaths := []string{"/feed", "/feed/", "/rss", "/rss/", "/rss.xml", "/feed.xml", "/atom.xml", "/index.xml"}

	// 1. Try to find RSS links on the main page
	req, err := http.NewRequest("GET", siteURL, nil)
	if err != nil {
		return sources
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return sources
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sources
	}

	// Parse HTML for RSS link elements
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err == nil {
		rssLinks := findRSSLinkElements(doc)
		for _, link := range rssLinks {
			feedURL := link
			if !strings.HasPrefix(feedURL, "http") {
				ref, err := url.Parse(feedURL)
				if err == nil {
					feedURL = parsed.ResolveReference(ref).String()
				}
			}

			sources = append(sources, Source{
				URL:         feedURL,
				Title:       parsed.Host,
				Description: "Site RSS feed",
				Type:        "rss",
			})
		}
	}

	// 2. If no RSS found, try common paths
	if len(sources) == 0 {
		for _, path := range commonPaths {
			testURL := baseURL + path
			if d.isValidFeed(testURL) {
				sources = append(sources, Source{
					URL:         testURL,
					Title:       parsed.Host,
					Description: "Discovered RSS feed",
					Type:        "rss",
					Validated:   true,
				})
				break
			}
		}
	}

	return sources
}

// isValidFeed checks if a URL returns valid RSS/Atom content
func (d *RSSDiscoverer) isValidFeed(feedURL string) bool {
	req, err := http.NewRequest("HEAD", feedURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", d.userAgent)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "rss") ||
		strings.Contains(contentType, "atom")
}

// looksLikeFeed checks if a URL looks like an RSS feed
func (d *RSSDiscoverer) looksLikeFeed(u string) bool {
	lower := strings.ToLower(u)
	return strings.Contains(lower, "/feed") ||
		strings.Contains(lower, "/rss") ||
		strings.Contains(lower, "atom") ||
		strings.HasSuffix(lower, ".xml") ||
		strings.Contains(lower, "/feeds/")
}

// findRSSLinkElements finds <link> elements with RSS/Atom types
func findRSSLinkElements(n *html.Node) []string {
	var links []string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			var linkType, href string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "type":
					linkType = attr.Val
				case "href":
					href = attr.Val
				}
			}

			if href != "" && (strings.Contains(linkType, "rss") || strings.Contains(linkType, "atom")) {
				links = append(links, href)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)

	return links
}

// ValidateFeed validates an RSS/Atom feed URL
func (d *RSSDiscoverer) ValidateFeed(feedURL string) (bool, error) {
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("feed returned status %d", resp.StatusCode)
	}

	// Read first 1KB to check content
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	content := string(buf[:n])

	// Check for RSS/Atom markers
	isRSS := strings.Contains(content, "<rss") || strings.Contains(content, "<feed") || strings.Contains(content, "<atom")
	return isRSS, nil
}
