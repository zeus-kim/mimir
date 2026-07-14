// Package validator provides source validation and trust scoring for feeds.
package validator

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Source represents a feed source to validate.
type Source struct {
	URL        string  `json:"url"`
	FeedType   string  `json:"feed_type"`   // "rss", "api", "github"
	SourceType string  `json:"source"`      // "feedspot", "gov_search", etc.
	Tier       int     `json:"tier"`        // 1=high, 2=medium, 3=low
	Title      string  `json:"title"`
	ItemCount  int     `json:"item_count"`
	Language   string  `json:"language"`
	TrustScore float64 `json:"trust_score"`
	Validated  bool    `json:"validated"`
}

// Validator validates feed sources and calculates trust scores.
type Validator struct {
	client     *http.Client
	userAgent  string
	maxWorkers int
}

// NewValidator creates a new source validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		userAgent:  "Mozilla/5.0 (Mimir Validator)",
		maxWorkers: 10,
	}
}

// ValidateBatch validates multiple sources concurrently and returns those
// meeting the trust threshold, sorted by trust score descending.
func (v *Validator) ValidateBatch(sources []Source, languages []string, trustThreshold float64) []Source {
	var (
		mu        sync.Mutex
		validated []Source
		wg        sync.WaitGroup
		sem       = make(chan struct{}, v.maxWorkers)
	)

	for _, src := range sources {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(s Source) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			result, ok := v.validateOne(s, languages)
			if !ok {
				return
			}

			if result.TrustScore >= trustThreshold {
				mu.Lock()
				validated = append(validated, result)
				mu.Unlock()
			}
		}(src)
	}

	wg.Wait()

	// Sort by trust score descending
	sort.Slice(validated, func(i, j int) bool {
		return validated[i].TrustScore > validated[j].TrustScore
	})

	return validated
}

// validateOne validates a single source based on its type.
func (v *Validator) validateOne(source Source, languages []string) (Source, bool) {
	if source.URL == "" {
		return Source{}, false
	}

	feedType := source.FeedType
	if feedType == "" {
		feedType = "rss"
	}

	switch feedType {
	case "api":
		return v.validateAPI(source)
	case "github":
		return v.validateGitHub(source)
	default:
		return v.validateRSS(source, languages)
	}
}

// validateRSS validates an RSS/Atom feed.
func (v *Validator) validateRSS(source Source, languages []string) (Source, bool) {
	req, err := http.NewRequest("GET", source.URL, nil)
	if err != nil {
		return Source{}, false
	}
	req.Header.Set("User-Agent", v.userAgent)

	resp, err := v.client.Do(req)
	if err != nil {
		return Source{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Source{}, false
	}

	// Read limited content (5KB)
	limited := io.LimitReader(resp.Body, 5000)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Source{}, false
	}

	content := string(body)

	// Check for RSS/Atom markers
	if !strings.Contains(content, "<rss") &&
		!strings.Contains(content, "<feed") &&
		!strings.Contains(content, "<channel") {
		return Source{}, false
	}

	// Parse and count items
	itemCount := v.countFeedItems(body)
	if itemCount < 1 {
		return Source{}, false
	}

	// Extract title
	title := v.extractFeedTitle(body)
	if title == "" {
		if u, err := url.Parse(source.URL); err == nil {
			title = u.Host
		}
	}

	// Skip comment feeds
	if strings.HasPrefix(strings.ToLower(title), "comment") {
		return Source{}, false
	}

	// Detect language
	lang := detectLanguage(content)

	// Calculate trust score
	trustScore := v.calculateTrust(source, itemCount, lang, languages)

	result := source
	result.Title = truncate(title, 100)
	result.ItemCount = itemCount
	result.Language = lang
	result.TrustScore = trustScore
	result.Validated = true

	return result, true
}

// validateAPI validates an API source (always valid with high trust).
func (v *Validator) validateAPI(source Source) (Source, bool) {
	result := source
	result.TrustScore = 0.9
	result.Validated = true
	return result, true
}

// validateGitHub validates a GitHub source.
func (v *Validator) validateGitHub(source Source) (Source, bool) {
	// GitHub releases Atom feed
	if strings.Contains(source.URL, "releases.atom") {
		req, err := http.NewRequest("HEAD", source.URL, nil)
		if err != nil {
			return v.passGitHubLowTrust(source)
		}
		req.Header.Set("User-Agent", v.userAgent)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return v.passGitHubLowTrust(source)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			result := source
			result.TrustScore = 0.7
			result.Validated = true
			return result, true
		}
	}

	return v.passGitHubLowTrust(source)
}

func (v *Validator) passGitHubLowTrust(source Source) (Source, bool) {
	result := source
	result.TrustScore = 0.5
	result.Validated = true
	return result, true
}

// countFeedItems counts <item> or <entry> elements in the feed.
func (v *Validator) countFeedItems(content []byte) int {
	// Try RSS format
	var rss struct {
		Channel struct {
			Items []struct{} `xml:"item"`
		} `xml:"channel"`
	}
	if xml.Unmarshal(content, &rss) == nil && len(rss.Channel.Items) > 0 {
		return len(rss.Channel.Items)
	}

	// Try Atom format
	var atom struct {
		Entries []struct{} `xml:"entry"`
	}
	if xml.Unmarshal(content, &atom) == nil && len(atom.Entries) > 0 {
		return len(atom.Entries)
	}

	// Fallback: count via string matching
	count := strings.Count(string(content), "<item") + strings.Count(string(content), "<entry")
	return count
}

// extractFeedTitle extracts the title from RSS/Atom feed.
func (v *Validator) extractFeedTitle(content []byte) string {
	// Try RSS format
	var rss struct {
		Channel struct {
			Title string `xml:"title"`
		} `xml:"channel"`
	}
	if xml.Unmarshal(content, &rss) == nil && rss.Channel.Title != "" {
		return strings.TrimSpace(rss.Channel.Title)
	}

	// Try Atom format
	var atom struct {
		Title string `xml:"title"`
	}
	if xml.Unmarshal(content, &atom) == nil && atom.Title != "" {
		return strings.TrimSpace(atom.Title)
	}

	return ""
}

// Language detection patterns
var (
	koreanPattern  = regexp.MustCompile(`[가-힣]{5,}`)
	japanesePattern = regexp.MustCompile(`[ぁ-んァ-ン]{5,}`)
	chinesePattern = regexp.MustCompile(`[一-龥]{5,}`)
)

// detectLanguage performs simple language detection based on character patterns.
func detectLanguage(content string) string {
	if koreanPattern.MatchString(content) {
		return "ko"
	}
	if japanesePattern.MatchString(content) {
		return "ja"
	}
	if chinesePattern.MatchString(content) {
		return "zh"
	}
	return "en"
}

// calculateTrust computes trust score for a source.
func (v *Validator) calculateTrust(source Source, itemCount int, lang string, targetLanguages []string) float64 {
	score := 0.5 // base score

	// Source type bonus
	switch source.SourceType {
	case "feedspot", "gov_search":
		score += 0.2
	case "site_discovery", "path_discovery":
		score += 0.1
	}

	// Item count bonus
	if itemCount >= 10 {
		score += 0.1
	} else if itemCount >= 5 {
		score += 0.05
	}

	// Language matching bonus
	for _, targetLang := range targetLanguages {
		if lang == targetLang {
			score += 0.1
			break
		}
	}

	// Tier bonus
	tier := source.Tier
	if tier == 0 {
		tier = 2 // default tier
	}
	switch tier {
	case 1:
		score += 0.15
	case 3:
		score -= 0.1
	}

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
