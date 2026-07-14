package discovery

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// BlogDiscoverer discovers blog feeds from various platforms
type BlogDiscoverer struct {
	httpClient *http.Client
	userAgent  string
}

// BlogPlatform represents a blogging platform with its RSS patterns
type BlogPlatform struct {
	Name        string
	SearchQuery string // DuckDuckGo search pattern
	RSSPattern  func(blogURL string) string
}

var blogPlatforms = []BlogPlatform{
	{
		Name:        "medium",
		SearchQuery: "site:medium.com %s",
		RSSPattern: func(blogURL string) string {
			// Medium tag feeds
			return "" // Handled separately with tag feeds
		},
	},
	{
		Name:        "substack",
		SearchQuery: "site:substack.com %s",
		RSSPattern: func(blogURL string) string {
			parsed, _ := url.Parse(blogURL)
			if parsed != nil {
				return fmt.Sprintf("%s://%s/feed", parsed.Scheme, parsed.Host)
			}
			return ""
		},
	},
	{
		Name:        "wordpress",
		SearchQuery: "site:wordpress.com %s",
		RSSPattern: func(blogURL string) string {
			return strings.TrimSuffix(blogURL, "/") + "/feed/"
		},
	},
	{
		Name:        "tistory",
		SearchQuery: "site:tistory.com %s",
		RSSPattern: func(blogURL string) string {
			parsed, _ := url.Parse(blogURL)
			if parsed != nil {
				return fmt.Sprintf("%s://%s/rss", parsed.Scheme, parsed.Host)
			}
			return ""
		},
	},
	{
		Name:        "naver",
		SearchQuery: "site:blog.naver.com %s",
		RSSPattern: func(blogURL string) string {
			// Extract blog ID from URL
			re := regexp.MustCompile(`blog\.naver\.com/([^/]+)`)
			if matches := re.FindStringSubmatch(blogURL); len(matches) > 1 {
				return fmt.Sprintf("https://rss.blog.naver.com/%s.xml", matches[1])
			}
			return ""
		},
	},
	{
		Name:        "ghost",
		SearchQuery: "site:ghost.io %s",
		RSSPattern: func(blogURL string) string {
			return strings.TrimSuffix(blogURL, "/") + "/rss/"
		},
	},
	{
		Name:        "blogger",
		SearchQuery: "site:blogspot.com %s",
		RSSPattern: func(blogURL string) string {
			parsed, _ := url.Parse(blogURL)
			if parsed != nil {
				return fmt.Sprintf("%s://%s/feeds/posts/default", parsed.Scheme, parsed.Host)
			}
			return ""
		},
	},
}

// NewBlogDiscoverer creates a new blog discoverer
func NewBlogDiscoverer() *BlogDiscoverer {
	return &BlogDiscoverer{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		userAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
	}
}

func (d *BlogDiscoverer) Name() string { return "blog" }

// Discover finds blog feeds for the given topic and keywords
func (d *BlogDiscoverer) Discover(topic string, keywords []string, limit int) ([]Source, error) {
	var sources []Source
	seen := make(map[string]bool)

	// Search terms: topic + first 3 keywords
	searchTerms := append([]string{topic}, keywords...)
	if len(searchTerms) > 3 {
		searchTerms = searchTerms[:3]
	}

	for _, kw := range searchTerms {
		// Medium tag feeds (direct)
		mediumTag := strings.ReplaceAll(strings.ToLower(kw), " ", "-")
		mediumURL := fmt.Sprintf("https://medium.com/feed/tag/%s", mediumTag)
		if !seen[mediumURL] {
			seen[mediumURL] = true
			sources = append(sources, Source{
				URL:         mediumURL,
				Title:       fmt.Sprintf("Medium: %s", kw),
				Description: fmt.Sprintf("Medium posts tagged with %s", kw),
				Type:        "rss",
			})
		}

		// Search each platform
		for _, platform := range blogPlatforms {
			if platform.Name == "medium" {
				continue // Already handled above
			}

			found := d.searchPlatform(kw, platform)
			for _, s := range found {
				if !seen[s.URL] {
					seen[s.URL] = true
					sources = append(sources, s)
				}
			}
		}
	}

	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources, nil
}

// searchPlatform searches a specific platform for blogs
func (d *BlogDiscoverer) searchPlatform(keyword string, platform BlogPlatform) []Source {
	var sources []Source

	query := fmt.Sprintf(platform.SearchQuery, keyword)
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

	// Extract URLs from search results
	urlPattern := regexp.MustCompile(`href="(https?://[^"]+)"`)
	matches := urlPattern.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		blogURL := match[1]

		// Filter for platform domain
		if !strings.Contains(strings.ToLower(blogURL), platform.Name) {
			continue
		}
		// Skip DuckDuckGo URLs
		if strings.Contains(blogURL, "duckduckgo") {
			continue
		}

		// Get RSS URL
		rssURL := platform.RSSPattern(blogURL)
		if rssURL == "" {
			continue
		}

		parsed, _ := url.Parse(blogURL)
		title := platform.Name
		if parsed != nil {
			title = strings.Trim(parsed.Path, "/")
			if title == "" {
				title = parsed.Host
			}
		}

		sources = append(sources, Source{
			URL:         rssURL,
			Title:       title,
			Description: fmt.Sprintf("%s blog about %s", platform.Name, keyword),
			Type:        "rss",
		})

		if len(sources) >= 3 {
			break
		}
	}

	return sources
}

// GuessRSSURL attempts to guess the RSS URL for a blog
func GuessRSSURL(blogURL string) string {
	parsed, err := url.Parse(blogURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Host)

	// Platform-specific patterns
	for _, platform := range blogPlatforms {
		if strings.Contains(host, platform.Name) {
			return platform.RSSPattern(blogURL)
		}
	}

	// Generic patterns to try
	base := strings.TrimSuffix(blogURL, "/")
	patterns := []string{
		base + "/feed",
		base + "/rss",
		base + "/feed.xml",
		base + "/rss.xml",
		base + "/atom.xml",
		base + "/index.xml",
	}

	for _, pattern := range patterns {
		return pattern // Return first one; caller should validate
	}

	return ""
}
