// Package curator provides automated feed curation for mimir verticals.
// This file handles the auto-discovery pipeline: discover, validate, prune, and rebuild.
package curator

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Feed represents a basic feed with common fields used across curator functions
type Feed struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Language  string `json:"language"`
	ItemCount int    `json:"item_count"`
	Domain    string `json:"domain"`
}

// DiscoveredFeed represents a feed found during discovery (before validation)
// Alias for Feed to maintain API compatibility
type DiscoveredFeed = Feed

// AutoCurationResult holds the results of an auto-curation run
type AutoCurationResult struct {
	Vertical      string   `json:"vertical"`
	SitesSearched int      `json:"sites_searched"`
	FeedsFound    int      `json:"feeds_found"`
	FeedsValid    int      `json:"feeds_valid"`
	FeedsAdded    int      `json:"feeds_added"`
	Errors        []string `json:"errors,omitempty"`
}

// AutoCurator handles automated feed discovery and curation for a vertical
type AutoCurator struct {
	Vertical   string
	Topic      string
	Languages  []string
	Limit      int
	DBPath     string
	httpClient *http.Client

	// Results
	mu         sync.Mutex
	discovered []DiscoveredFeed
	validated  []DiscoveredFeed
}

// knownFeeds contains curated feed lists for common verticals
var knownFeeds = map[string][]struct {
	URL      string
	Title    string
	Language string
}{
	"bakery": {
		{"https://www.theperfectloaf.com/feed/", "The Perfect Loaf", "en"},
		{"https://www.kingarthurbaking.com/blog/feed", "King Arthur Baking", "en"},
		{"https://sallysbakingaddiction.com/feed/", "Sally's Baking Addiction", "en"},
		{"https://breadtopia.com/feed/", "Breadtopia", "en"},
		{"https://www.theclevercarrot.com/feed/", "The Clever Carrot", "en"},
		{"https://www.seriouseats.com/feeds/tags/baking", "Serious Eats Baking", "en"},
		{"https://www.weekendbakery.com/feed/", "Weekend Bakery", "en"},
		{"https://www.youtube.com/feeds/videos.xml?channel_id=UChBEbMKI1eCcejTtmI32UEw", "Joshua Weissman", "en"},
	},
	"wine": {
		{"https://www.wine-searcher.com/feed/", "Wine Searcher", "en"},
		{"https://www.winespectator.com/rss/rss", "Wine Spectator", "en"},
		{"https://www.decanter.com/feed/", "Decanter", "en"},
	},
	"coffee": {
		{"https://dailycoffeenews.com/feed/", "Daily Coffee News", "en"},
		{"https://www.perfectdailygrind.com/feed/", "Perfect Daily Grind", "en"},
		{"https://sprudge.com/feed", "Sprudge", "en"},
	},
	"politics": {
		{"https://www.yna.co.kr/rss/politics.xml", "연합뉴스 정치", "ko"},
		{"https://www.hani.co.kr/rss/politics/", "한겨레 정치", "ko"},
		{"https://rss.donga.com/politics.xml", "동아일보 정치", "ko"},
	},
}

// commonRSSPaths are typical paths where RSS feeds are found
var commonRSSPaths = []string{
	"/feed", "/feed/", "/rss", "/rss/", "/rss.xml", "/feed.xml",
	"/atom.xml", "/index.xml", "/blog/feed", "/blog/rss",
}

// NewAutoCurator creates a new AutoCurator for the given vertical
func NewAutoCurator(vertical, topic string, languages []string, limit int) (*AutoCurator, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	dbPath := filepath.Join(home, fmt.Sprintf(".mine-%s", vertical), "lite.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("vertical '%s' does not exist: %s", vertical, dbPath)
	}

	if limit <= 0 {
		limit = 20
	}

	return &AutoCurator{
		Vertical:  vertical,
		Topic:     topic,
		Languages: languages,
		Limit:     limit,
		DBPath:    dbPath,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}, nil
}

// AutoCurate runs the full auto-curation pipeline
func (c *AutoCurator) AutoCurate() (*AutoCurationResult, error) {
	result := &AutoCurationResult{Vertical: c.Vertical}

	// Step 1: Generate search queries
	queries := c.generateQueries()

	// Step 2: Search for sites
	sites := c.searchSites(queries)
	result.SitesSearched = len(sites)

	// Step 3: Discover RSS feeds on sites (parallel)
	feeds := c.discoverFeeds(sites)
	result.FeedsFound = len(feeds)

	// Step 4: Validate feeds (parallel)
	validated := c.validateFeeds(feeds)

	// Step 5: Add known feeds for this vertical
	validated = c.addKnownFeeds(validated)
	result.FeedsValid = len(validated)

	// Step 6: Add to database
	added, err := c.addToDatabase(validated)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.FeedsAdded = added

	c.validated = validated
	return result, nil
}

// GetValidatedFeeds returns the validated feeds from the last curation run
func (c *AutoCurator) GetValidatedFeeds() []DiscoveredFeed {
	return c.validated
}

// generateQueries creates search queries for the topic
func (c *AutoCurator) generateQueries() []string {
	queries := []string{
		c.Topic + " best RSS feeds blogs",
		c.Topic + " RSS feed list",
		"best " + c.Topic + " blogs to follow",
	}

	// Add Korean queries if ko language is enabled
	for _, lang := range c.Languages {
		if lang == "ko" {
			fields := strings.Fields(c.Topic)
			if len(fields) > 0 {
				queries = append(queries, fields[0]+" 블로그 RSS")
			}
			break
		}
	}

	return queries
}

// searchSites searches DuckDuckGo and Feedspot for relevant sites
func (c *AutoCurator) searchSites(queries []string) []string {
	seen := make(map[string]bool)
	var sites []string

	// Search Feedspot first (most reliable)
	feedspotSites := c.searchFeedspot(c.Topic)
	for _, site := range feedspotSites {
		domain := getDomain(site)
		if !seen[domain] {
			seen[domain] = true
			sites = append(sites, site)
		}
	}

	// Search DuckDuckGo for each query
	for _, query := range queries {
		results := c.searchDuckDuckGo(query, 10)
		for _, site := range results {
			domain := getDomain(site)
			if !seen[domain] {
				seen[domain] = true
				sites = append(sites, site)
			}
		}
	}

	return sites
}

// searchDuckDuckGo searches using DuckDuckGo Lite
func (c *AutoCurator) searchDuckDuckGo(query string, limit int) []string {
	endpoint := "https://lite.duckduckgo.com/lite/"

	data := url.Values{}
	data.Set("q", query)
	data.Set("kl", "us-en")

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return c.extractURLsFromHTML(string(body), limit)
}

// searchFeedspot searches Feedspot directory
func (c *AutoCurator) searchFeedspot(topic string) []string {
	slug := strings.ReplaceAll(strings.ToLower(topic), " ", "_")
	feedspotURL := fmt.Sprintf("https://rss.feedspot.com/%s_rss_feeds/", slug)

	req, err := http.NewRequest("GET", feedspotURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	return c.extractFeedspotURLs(string(body))
}

// extractURLsFromHTML extracts URLs from HTML content
func (c *AutoCurator) extractURLsFromHTML(html string, limit int) []string {
	// Match href="https://..." pattern, excluding DDG internal links
	re := regexp.MustCompile(`href="(https?://[^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)

	var urls []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		u := match[1]

		// Skip DDG internal links
		if strings.Contains(u, "duckduckgo") {
			continue
		}

		if !seen[u] && len(urls) < limit {
			seen[u] = true
			urls = append(urls, u)
		}
	}

	return urls
}

// extractFeedspotURLs extracts feed URLs from Feedspot HTML
func (c *AutoCurator) extractFeedspotURLs(html string) []string {
	re := regexp.MustCompile(`href="(https?://[^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)

	var urls []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		u := match[1]

		// Skip Feedspot, social media links
		lower := strings.ToLower(u)
		if strings.Contains(lower, "feedspot") ||
			strings.Contains(lower, "facebook") ||
			strings.Contains(lower, "twitter") {
			continue
		}

		if !seen[u] && len(urls) < 20 {
			seen[u] = true
			urls = append(urls, u)
		}
	}

	return urls
}

// discoverFeeds discovers RSS feeds on the given sites (parallel)
func (c *AutoCurator) discoverFeeds(sites []string) []DiscoveredFeed {
	if len(sites) > 30 {
		sites = sites[:30]
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []DiscoveredFeed
	)

	sem := make(chan struct{}, 10) // Limit concurrency

	for _, site := range sites {
		wg.Add(1)
		go func(siteURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			feeds := c.discoverRSSOnSite(siteURL)
			if len(feeds) > 0 {
				mu.Lock()
				results = append(results, feeds...)
				mu.Unlock()
			}
		}(site)
	}

	wg.Wait()
	return results
}

// discoverRSSOnSite finds RSS feeds on a single site
func (c *AutoCurator) discoverRSSOnSite(siteURL string) []DiscoveredFeed {
	var feeds []DiscoveredFeed

	parsed, err := url.Parse(siteURL)
	if err != nil {
		return nil
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	// 1. Check main page for RSS link tags
	req, err := http.NewRequest("GET", siteURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024)) // Read max 100KB
	if err != nil {
		return nil
	}
	html := string(body)

	// Look for <link type="application/rss+xml"> or <link type="application/atom+xml">
	rssLinkRe := regexp.MustCompile(`<link[^>]+type=["']application/(rss|atom)\+xml["'][^>]*href=["']([^"']+)["'][^>]*>`)
	matches := rssLinkRe.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			feedURL := c.resolveURL(baseURL, siteURL, match[2])
			title := extractLinkTitleFromTag(match[0], parsed.Host)
			feeds = append(feeds, DiscoveredFeed{
				URL:    feedURL,
				Title:  title,
				Domain: parsed.Host,
			})
		}
	}

	// Also check alternate attribute order: href before type
	rssLinkRe2 := regexp.MustCompile(`<link[^>]+href=["']([^"']+)["'][^>]+type=["']application/(rss|atom)\+xml["'][^>]*>`)
	matches2 := rssLinkRe2.FindAllStringSubmatch(html, -1)

	for _, match := range matches2 {
		if len(match) >= 2 {
			feedURL := c.resolveURL(baseURL, siteURL, match[1])
			feeds = append(feeds, DiscoveredFeed{
				URL:    feedURL,
				Title:  parsed.Host,
				Domain: parsed.Host,
			})
		}
	}

	// Look for <a href="...feed..."> links
	feedLinkRe := regexp.MustCompile(`<a[^>]+href=["']([^"']*(?:feed|rss)[^"']*)["'][^>]*>`)
	feedMatches := feedLinkRe.FindAllStringSubmatch(html, -1)

	for _, match := range feedMatches {
		if len(match) >= 2 {
			href := match[1]
			// Skip feedback links
			if strings.Contains(strings.ToLower(href), "feedback") {
				continue
			}
			feedURL := c.resolveURL(baseURL, siteURL, href)
			feeds = append(feeds, DiscoveredFeed{
				URL:    feedURL,
				Title:  parsed.Host,
				Domain: parsed.Host,
			})
		}
	}

	// 2. Try common RSS paths if no feeds found
	if len(feeds) == 0 {
		for _, path := range commonRSSPaths {
			testURL := baseURL + path
			if c.checkIsRSS(testURL) {
				feeds = append(feeds, DiscoveredFeed{
					URL:    testURL,
					Title:  parsed.Host,
					Domain: parsed.Host,
				})
				break // Found one, stop probing
			}
		}
	}

	return feeds
}

// resolveURL resolves a potentially relative URL
func (c *AutoCurator) resolveURL(baseURL, pageURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}

	if strings.HasPrefix(href, "/") {
		return baseURL + href
	}

	// Relative path
	base, err := url.Parse(pageURL)
	if err != nil {
		return baseURL + "/" + href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return baseURL + "/" + href
	}
	return base.ResolveReference(ref).String()
}

// checkIsRSS checks if a URL returns RSS/Atom content
func (c *AutoCurator) checkIsRSS(feedURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("HEAD", feedURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "rss") ||
		strings.Contains(contentType, "atom")
}

// validateFeeds validates discovered feeds (parallel)
func (c *AutoCurator) validateFeeds(feeds []DiscoveredFeed) []DiscoveredFeed {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		valid []DiscoveredFeed
	)

	sem := make(chan struct{}, 10)

	for _, feed := range feeds {
		wg.Add(1)
		go func(f DiscoveredFeed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			validated := c.validateFeed(f)
			if validated != nil {
				// Check language filter
				for _, lang := range c.Languages {
					if validated.Language == lang {
						mu.Lock()
						valid = append(valid, *validated)
						mu.Unlock()
						break
					}
				}
			}
		}(feed)
	}

	wg.Wait()
	return valid
}

// validateFeed validates a single feed
func (c *AutoCurator) validateFeed(feed DiscoveredFeed) *DiscoveredFeed {
	// URL filters - skip comment feeds
	lower := strings.ToLower(feed.URL)
	if strings.Contains(lower, "/comments") ||
		strings.Contains(lower, "comment-page") ||
		strings.Contains(lower, "feedback") ||
		strings.Contains(lower, "respond") {
		return nil
	}

	req, err := http.NewRequest("GET", feed.URL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024)) // Read max 50KB
	if err != nil {
		return nil
	}
	content := string(body)

	// Check if it's RSS/Atom
	if !strings.Contains(content, "<rss") &&
		!strings.Contains(content, "<feed") &&
		!strings.Contains(content, "<channel") {
		return nil
	}

	// Extract title
	title := feed.Title
	titleRe := regexp.MustCompile(`<title>([^<]+)</title>`)
	if match := titleRe.FindStringSubmatch(content); len(match) >= 2 {
		title = strings.TrimSpace(match[1])
		// Truncate long titles
		if len(title) > 100 {
			title = title[:100]
		}
	}

	// Skip comment feeds by title
	if strings.HasPrefix(strings.ToLower(title), "comments") {
		return nil
	}

	// Count items (minimum 3)
	itemRe := regexp.MustCompile(`<(item|entry)[\s>]`)
	items := itemRe.FindAllString(content, -1)
	if len(items) < 3 {
		return nil
	}

	// Detect language
	lang := detectFeedLanguage(content)

	parsed, _ := url.Parse(feed.URL)
	domain := ""
	if parsed != nil {
		domain = parsed.Host
	}

	return &DiscoveredFeed{
		URL:       feed.URL,
		Title:     title,
		Language:  lang,
		ItemCount: len(items),
		Domain:    domain,
	}
}

// addKnownFeeds adds curated feeds for the vertical
func (c *AutoCurator) addKnownFeeds(feeds []DiscoveredFeed) []DiscoveredFeed {
	known, ok := knownFeeds[c.Vertical]
	if !ok {
		return feeds
	}

	for _, kf := range known {
		// Check if language is in filter
		allowed := false
		for _, lang := range c.Languages {
			if kf.Language == lang {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}

		feeds = append(feeds, DiscoveredFeed{
			URL:      kf.URL,
			Title:    kf.Title,
			Language: kf.Language,
		})
	}

	return feeds
}

// addToDatabase adds validated feeds to the database
func (c *AutoCurator) addToDatabase(feeds []DiscoveredFeed) (int, error) {
	if len(feeds) == 0 {
		return 0, nil
	}

	// Limit to configured max
	if len(feeds) > c.Limit {
		feeds = feeds[:c.Limit]
	}

	db, err := sql.Open("sqlite3", c.DBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return 0, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	now := time.Now().Unix()
	added := 0

	for _, feed := range feeds {
		result, err := db.Exec(`
			INSERT OR IGNORE INTO feeds (url, name, language, status, tier, created_at)
			VALUES (?, ?, ?, 'active', 1, ?)
		`, feed.URL, feed.Title, feed.Language, now)

		if err != nil {
			continue
		}

		rows, _ := result.RowsAffected()
		added += int(rows)
	}

	return added, nil
}

// MeasureDomainFit measures the domain fit percentage for a vertical
func MeasureDomainFit(vertical string, keywords []string) (float64, int, int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0, 0, err
	}

	dbPath := filepath.Join(home, fmt.Sprintf(".mine-%s", vertical), "lite.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&mode=ro")
	if err != nil {
		return 0, 0, 0, err
	}
	defer db.Close()

	// Total document count
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&total); err != nil {
		return 0, 0, 0, err
	}

	if total == 0 {
		return 0, 0, 0, nil
	}

	// Matching documents using FTS
	query := strings.Join(keywords, " OR ")
	var matching int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM documents_fts
		WHERE documents_fts MATCH ?
	`, query).Scan(&matching); err != nil {
		// FTS might not exist, try simple LIKE
		matching = 0
		for _, kw := range keywords {
			var count int
			db.QueryRow(`
				SELECT COUNT(*) FROM documents
				WHERE title LIKE ? OR summary LIKE ?
			`, "%"+kw+"%", "%"+kw+"%").Scan(&count)
			matching += count
		}
		// Deduplicate estimate
		if len(keywords) > 0 {
			matching = matching / len(keywords)
		}
	}

	fit := float64(matching) / float64(total) * 100
	return fit, matching, total, nil
}

// PruneLowQualityFeeds removes feeds that don't contribute to domain fit
func PruneLowQualityFeeds(vertical string, keywords []string, minFit float64) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	dbPath := filepath.Join(home, fmt.Sprintf(".mine-%s", vertical), "lite.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if len(keywords) == 0 {
		return 0, fmt.Errorf("no keywords provided")
	}

	// Find feeds with low domain fit contribution
	rows, err := db.Query(`
		SELECT f.id, f.url, f.name,
			   COUNT(d.id) as doc_count,
			   SUM(CASE WHEN d.title LIKE ? OR d.summary LIKE ? THEN 1 ELSE 0 END) as matching
		FROM feeds f
		LEFT JOIN documents d ON d.feed_id = f.id
		GROUP BY f.id
		HAVING doc_count > 5 AND (matching * 1.0 / doc_count) < ?
	`, "%"+keywords[0]+"%", "%"+keywords[0]+"%", minFit/100)

	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var feedsToRemove []int64
	for rows.Next() {
		var id int64
		var feedURL, name string
		var docCount, matching int
		if err := rows.Scan(&id, &feedURL, &name, &docCount, &matching); err != nil {
			continue
		}
		feedsToRemove = append(feedsToRemove, id)
	}

	// Mark feeds as inactive instead of deleting
	removed := 0
	for _, id := range feedsToRemove {
		result, err := db.Exec("UPDATE feeds SET status = 'pruned' WHERE id = ?", id)
		if err == nil {
			n, _ := result.RowsAffected()
			removed += int(n)
		}
	}

	return removed, nil
}

// RebuildFTSIndex rebuilds the FTS index for a vertical
func RebuildFTSIndex(vertical string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dbPath := filepath.Join(home, fmt.Sprintf(".mine-%s", vertical), "lite.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=30000")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO documents_fts(documents_fts) VALUES('rebuild')")
	return err
}

// Helper functions

// extractDomain extracts the domain from a URL (lowercase)
func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Host)
}

// getDomain extracts the domain from a URL
func getDomain(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	return parsed.Host
}

func extractLinkTitleFromTag(linkTag, defaultTitle string) string {
	re := regexp.MustCompile(`title=["']([^"']+)["']`)
	if match := re.FindStringSubmatch(linkTag); len(match) >= 2 {
		return match[1]
	}
	return defaultTitle
}

func detectFeedLanguage(content string) string {
	// Simple language detection based on character presence
	koRe := regexp.MustCompile(`[가-힣]{3,}`)
	jaRe := regexp.MustCompile(`[ぁ-んァ-ン]{3,}`)
	zhRe := regexp.MustCompile(`[一-鿿]{3,}`)

	sample := content
	if len(sample) > 5000 {
		sample = sample[:5000]
	}

	if koRe.MatchString(sample) {
		return "ko"
	}
	if jaRe.MatchString(sample) {
		return "ja"
	}
	if zhRe.MatchString(sample) {
		return "zh"
	}

	return "en"
}
