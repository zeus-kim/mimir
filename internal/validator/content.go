package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ContentValidator validates content from feeds for domain relevance
type ContentValidator struct {
	Topic      string
	Keywords   []string
	SampleSize int
	client     *http.Client
	llmClient  *LLMClient
}

// Feed represents a feed to validate
type Feed struct {
	URL            string  `json:"url"`
	FeedType       string  `json:"feed_type"` // rss, atom, api
	RelevanceScore float64 `json:"relevance_score,omitempty"`
	RelevanceReason string `json:"relevance_reason,omitempty"`
}

// RelevanceResult contains the relevance assessment from LLM
type RelevanceResult struct {
	Relevance float64 `json:"relevance"`
	Reason    string  `json:"reason"`
}

// LLMClient handles OpenAI API calls
type LLMClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
}

// NewContentValidator creates a new content validator
func NewContentValidator(topic string, keywords []string, sampleSize int) *ContentValidator {
	if sampleSize <= 0 {
		sampleSize = 3
	}
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	cv := &ContentValidator{
		Topic:      topic,
		Keywords:   keywords,
		SampleSize: sampleSize,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	cv.initLLMClient()
	return cv
}

// initLLMClient initializes the OpenAI client
func (cv *ContentValidator) initLLMClient() {
	apiKey := os.Getenv("OPENAI_API_KEY")

	// Try macOS Keychain if env var not set
	if apiKey == "" {
		cmd := exec.Command("security", "find-generic-password", "-s", "OPENAI_API_KEY", "-w")
		out, err := cmd.Output()
		if err == nil {
			apiKey = strings.TrimSpace(string(out))
		}
	}

	if apiKey != "" {
		cv.llmClient = &LLMClient{
			apiKey: apiKey,
			httpClient: &http.Client{
				Timeout: 30 * time.Second,
			},
			model: "gpt-4o-mini",
		}
	}
}

// HasLLM returns true if LLM client is available
func (cv *ContentValidator) HasLLM() bool {
	return cv.llmClient != nil
}

// ValidateFeeds validates a list of feeds and returns those meeting minimum relevance
func (cv *ContentValidator) ValidateFeeds(feeds []Feed, minRelevance float64) []Feed {
	if cv.llmClient == nil {
		// No LLM available, return all feeds unvalidated
		return feeds
	}

	var (
		validated []Feed
		mu        sync.Mutex
		wg        sync.WaitGroup
		sem       = make(chan struct{}, 5) // Max 5 concurrent validations
	)

	for _, feed := range feeds {
		wg.Add(1)
		go func(f Feed) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			result := cv.validateOne(f)
			if result != nil && result.Relevance >= minRelevance {
				f.RelevanceScore = result.Relevance
				f.RelevanceReason = result.Reason
				mu.Lock()
				validated = append(validated, f)
				mu.Unlock()
			}
		}(feed)
	}

	wg.Wait()
	return validated
}

// validateOne validates a single feed
func (cv *ContentValidator) validateOne(feed Feed) *RelevanceResult {
	if feed.URL == "" {
		return nil
	}

	samples := cv.fetchSamples(feed.URL, feed.FeedType)
	if len(samples) == 0 {
		return &RelevanceResult{
			Relevance: 0.3,
			Reason:    "Failed to fetch samples",
		}
	}

	return cv.checkRelevance(samples)
}

// fetchSamples fetches sample content from a feed URL
func (cv *ContentValidator) fetchSamples(url, feedType string) []string {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Mimir Validator)")

	resp, err := cv.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	content := string(body)

	// Detect feed type
	if feedType == "rss" || feedType == "atom" ||
		strings.Contains(content, "<rss") || strings.Contains(content, "<feed") {
		return cv.parseRSSSamples(content)
	}

	// JSON API
	if feedType == "api" || strings.HasPrefix(strings.TrimSpace(content), "{") {
		return cv.parseAPISamples(content)
	}

	return nil
}

// RSS/Atom XML structures
type rssChannel struct {
	Items []rssItem `xml:"channel>item"`
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
}

type atomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Content string `xml:"content"`
}

// parseRSSSamples parses RSS/Atom feed for sample content
func (cv *ContentValidator) parseRSSSamples(content string) []string {
	var samples []string

	// Try RSS format
	var rss rssChannel
	if err := xml.Unmarshal([]byte(content), &rss); err == nil && len(rss.Items) > 0 {
		for i, item := range rss.Items {
			if i >= cv.SampleSize {
				break
			}
			text := item.Title + " " + stripHTML(item.Description)
			if len(text) > 500 {
				text = text[:500]
			}
			if strings.TrimSpace(text) != "" {
				samples = append(samples, strings.TrimSpace(text))
			}
		}
		if len(samples) > 0 {
			return samples
		}
	}

	// Try Atom format
	var atom atomFeed
	if err := xml.Unmarshal([]byte(content), &atom); err == nil && len(atom.Entries) > 0 {
		for i, entry := range atom.Entries {
			if i >= cv.SampleSize {
				break
			}
			desc := entry.Summary
			if desc == "" {
				desc = entry.Content
			}
			text := entry.Title + " " + stripHTML(desc)
			if len(text) > 500 {
				text = text[:500]
			}
			if strings.TrimSpace(text) != "" {
				samples = append(samples, strings.TrimSpace(text))
			}
		}
	}

	return samples
}

// parseAPISamples parses JSON API response for sample content
func (cv *ContentValidator) parseAPISamples(content string) []string {
	var samples []string

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil
	}

	// OpenAlex format
	if results, ok := data["results"].([]interface{}); ok {
		for i, item := range results {
			if i >= cv.SampleSize {
				break
			}
			if m, ok := item.(map[string]interface{}); ok {
				if title, ok := m["title"].(string); ok && title != "" {
					samples = append(samples, title)
				}
			}
		}
	}

	return samples
}

// checkRelevance uses LLM to assess content relevance
func (cv *ContentValidator) checkRelevance(samples []string) *RelevanceResult {
	if len(samples) == 0 {
		return &RelevanceResult{Relevance: 0, Reason: "No samples"}
	}
	if cv.llmClient == nil {
		return &RelevanceResult{Relevance: 0.5, Reason: "No LLM available"}
	}

	// Build sample text
	var sampleLines []string
	for i, s := range samples {
		if i >= 5 {
			break
		}
		if len(s) > 200 {
			s = s[:200]
		}
		sampleLines = append(sampleLines, "- "+s)
	}
	sampleText := strings.Join(sampleLines, "\n")

	prompt := fmt.Sprintf(`Evaluate if this content is relevant to "%s".

Keywords: %s

Content samples:
%s

Respond in JSON only (no explanation):
{"relevance": 0.0-1.0, "reason": "one-line reasoning"}

Scale:
- 1.0: Completely focused on this topic
- 0.7-0.9: Highly relevant
- 0.4-0.6: Partially relevant
- 0.0-0.3: Barely relevant`,
		cv.Topic,
		strings.Join(cv.Keywords, ", "),
		sampleText,
	)

	result, err := cv.llmClient.complete(prompt)
	if err != nil {
		return &RelevanceResult{Relevance: 0.5, Reason: "LLM call failed"}
	}

	return result
}

// complete sends a chat completion request to OpenAI
func (lc *LLMClient) complete(prompt string) (*RelevanceResult, error) {
	reqBody := map[string]interface{}{
		"model":      lc.model,
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+lc.apiKey)

	resp, err := lc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	text := response.Choices[0].Message.Content

	// Parse JSON from response
	jsonRegex := regexp.MustCompile(`\{[^}]+\}`)
	match := jsonRegex.FindString(text)
	if match == "" {
		return &RelevanceResult{Relevance: 0.5, Reason: "Failed to parse response"}, nil
	}

	var result RelevanceResult
	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return &RelevanceResult{Relevance: 0.5, Reason: "Failed to parse JSON"}, nil
	}

	return &result, nil
}

// stripHTML removes HTML tags from text
var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return htmlTagRegex.ReplaceAllString(s, "")
}

// QuickCheck performs a simple keyword-based relevance check without LLM
// Returns a score from 0.0 to 1.0
func (cv *ContentValidator) QuickCheck(samples []string) float64 {
	if len(samples) == 0 || len(cv.Keywords) == 0 {
		return 0.5
	}

	var totalMatches float64
	var totalChecks float64

	for _, sample := range samples {
		sampleLower := strings.ToLower(sample)
		for _, keyword := range cv.Keywords {
			totalChecks++
			if strings.Contains(sampleLower, strings.ToLower(keyword)) {
				totalMatches++
			}
		}
	}

	if totalChecks == 0 {
		return 0.5
	}

	return totalMatches / totalChecks
}

// DetectLanguage attempts to detect the language of content
// Returns ISO 639-1 code (e.g., "en", "ko", "ja")
func DetectLanguage(text string) string {
	if text == "" {
		return "unknown"
	}

	// Simple heuristic based on character ranges
	koreanCount := 0
	japaneseCount := 0
	chineseCount := 0
	latinCount := 0

	for _, r := range text {
		switch {
		case r >= '가' && r <= '힯': // Korean Hangul
			koreanCount++
		case (r >= '぀' && r <= 'ゟ') || (r >= '゠' && r <= 'ヿ'): // Japanese Hiragana/Katakana
			japaneseCount++
		case r >= '一' && r <= '鿿': // CJK Unified Ideographs
			chineseCount++
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			latinCount++
		}
	}

	total := koreanCount + japaneseCount + chineseCount + latinCount
	if total == 0 {
		return "unknown"
	}

	// Return dominant script
	maxCount := latinCount
	lang := "en"

	if koreanCount > maxCount {
		maxCount = koreanCount
		lang = "ko"
	}
	if japaneseCount > maxCount {
		maxCount = japaneseCount
		lang = "ja"
	}
	if chineseCount > maxCount {
		lang = "zh"
	}

	return lang
}

// IsSpamContent checks if content appears to be spam or noise
func IsSpamContent(text string) bool {
	if len(text) < 20 {
		return true
	}

	textLower := strings.ToLower(text)

	// Spam indicators
	spamPatterns := []string{
		"click here",
		"buy now",
		"free money",
		"limited time",
		"act now",
		"call now",
		"100% free",
		"no obligation",
		"work from home",
		"$$",
	}

	spamCount := 0
	for _, pattern := range spamPatterns {
		if strings.Contains(textLower, pattern) {
			spamCount++
		}
	}

	// Multiple spam indicators = likely spam
	return spamCount >= 3
}

// QualityScore returns a content quality score based on various heuristics
func QualityScore(text string) float64 {
	if text == "" {
		return 0.0
	}

	score := 1.0

	// Penalize very short content
	if len(text) < 50 {
		score -= 0.3
	}

	// Penalize excessive caps
	capsCount := 0
	letterCount := 0
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			capsCount++
			letterCount++
		} else if r >= 'a' && r <= 'z' {
			letterCount++
		}
	}
	if letterCount > 0 && float64(capsCount)/float64(letterCount) > 0.5 {
		score -= 0.2
	}

	// Penalize spam content
	if IsSpamContent(text) {
		score -= 0.4
	}

	if score < 0 {
		score = 0
	}

	return score
}
