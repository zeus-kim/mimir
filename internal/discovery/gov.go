package discovery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// GovDiscoverer discovers government and public institution sources
type GovDiscoverer struct {
	httpClient *http.Client
	userAgent  string
}

// Language-specific government domain patterns
var govDomains = map[string][]string{
	"ko": {".go.kr", ".or.kr", ".ac.kr"},
	"en": {".gov", ".edu", ".org"},
	"ja": {".go.jp", ".or.jp", ".ac.jp"},
	"de": {".gov.de", ".bund.de"},
	"fr": {".gouv.fr", ".gov.fr"},
	"cn": {".gov.cn", ".edu.cn"},
}

// NewGovDiscoverer creates a new government source discoverer
func NewGovDiscoverer() *GovDiscoverer {
	return &GovDiscoverer{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	}
}

func (d *GovDiscoverer) Name() string { return "gov" }

// Discover finds government and public institution sources
func (d *GovDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	return d.DiscoverWithLanguages(topic, keywords, []string{"en"}, limit)
}

// DiscoverWithLanguages finds government sources for specific languages
func (d *GovDiscoverer) DiscoverWithLanguages(topic string, keywords []string, languages []string, limit int) ([]Source, error) {
	var sources []Source
	seen := make(map[string]bool)

	// Search terms
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 3 {
		searchTerms = searchTerms[:3]
	}

	for _, lang := range languages {
		domains, ok := govDomains[lang]
		if !ok {
			domains = govDomains["en"]
		}

		for _, term := range searchTerms {
			for _, domain := range domains[:min(2, len(domains))] {
				found := d.searchGov(term, domain)
				for _, s := range found {
					if !seen[s.URL] {
						seen[s.URL] = true
						s.Language = lang
						sources = append(sources, s)
					}
				}

				if len(sources) >= limit {
					break
				}
			}
		}
	}

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// searchGov searches for government sites using DuckDuckGo
func (d *GovDiscoverer) searchGov(keyword, domainSuffix string) []Source {
	var sources []Source

	query := fmt.Sprintf("site:%s %s RSS", domainSuffix, keyword)
	searchURL := "https://lite.duckduckgo.com/lite/"

	req, err := http.NewRequest("POST", searchURL, strings.NewReader(url.Values{"q": {query}}.Encode()))
	if err != nil {
		return sources
	}
	req.Header.Set("User-Agent", d.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return sources
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sources
	}

	// Parse HTML to extract links
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return sources
	}

	links := extractLinks(doc)

	for _, link := range links {
		href := link.URL
		if !strings.Contains(href, domainSuffix) {
			continue
		}
		if !strings.HasPrefix(href, "http") {
			continue
		}
		if strings.Contains(href, "duckduckgo") {
			continue
		}

		title := link.Text
		if title == "" || len(title) > 100 {
			title = href
		}

		sources = append(sources, Source{
			URL:         href,
			Title:       title,
			Description: fmt.Sprintf("Government source from %s", domainSuffix),
			Type:        "webpage",
			Score:       0.8, // Government sources get high trust score
		})

		if len(sources) >= 5 {
			break
		}
	}

	return sources
}

// DiscoverRSSFromGovSite attempts to find RSS feeds on a government site
func (d *GovDiscoverer) DiscoverRSSFromGovSite(siteURL string) ([]Source, error) {
	var sources []Source

	// Common government RSS paths
	paths := []string{
		"/rss",
		"/rss.xml",
		"/feed",
		"/feeds",
		"/news/rss",
		"/press/rss",
		"/updates/rss",
		"/content/rss.xml",
	}

	parsed, err := url.Parse(siteURL)
	if err != nil {
		return nil, err
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	// Try common paths
	for _, path := range paths {
		testURL := baseURL + path
		if d.isValidFeed(testURL) {
			sources = append(sources, Source{
				URL:         testURL,
				Title:       fmt.Sprintf("%s RSS", parsed.Host),
				Description: "Government RSS feed",
				Type:        "rss",
				Score:       0.9,
				Validated:   true,
			})
		}
	}

	// Also check main page for RSS links
	pageFeeds := d.findRSSLinksOnPage(siteURL)
	sources = append(sources, pageFeeds...)

	return sources, nil
}

// isValidFeed checks if a URL returns a valid RSS/Atom feed
func (d *GovDiscoverer) isValidFeed(feedURL string) bool {
	req, err := http.NewRequest("HEAD", feedURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
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

// findRSSLinksOnPage parses a page for RSS link elements
func (d *GovDiscoverer) findRSSLinksOnPage(pageURL string) []Source {
	var sources []Source

	req, err := http.NewRequest("GET", pageURL, nil)
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

	// Find RSS/Atom link elements
	rssLinkPattern := regexp.MustCompile(`<link[^>]+type=["'](application/(rss|atom)\+xml)["'][^>]*href=["']([^"']+)["']`)
	matches := rssLinkPattern.FindAllStringSubmatch(string(body), -1)

	parsed, _ := url.Parse(pageURL)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		feedURL := match[3]

		// Handle relative URLs
		if !strings.HasPrefix(feedURL, "http") {
			if parsed != nil {
				ref, err := url.Parse(feedURL)
				if err == nil {
					feedURL = parsed.ResolveReference(ref).String()
				}
			}
		}

		sources = append(sources, Source{
			URL:         feedURL,
			Title:       parsed.Host,
			Description: "Government RSS feed",
			Type:        "rss",
			Score:       0.85,
		})
	}

	return sources
}

// linkInfo holds extracted link information
type linkInfo struct {
	URL  string
	Text string
}

// extractLinks extracts links from parsed HTML
func extractLinks(n *html.Node) []linkInfo {
	var links []linkInfo

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href, text string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
					break
				}
			}
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				text = strings.TrimSpace(n.FirstChild.Data)
			}
			if href != "" {
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
