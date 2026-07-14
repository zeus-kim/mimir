package curator

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// TierFeed extends Feed with tier and trust scoring for curation
type TierFeed struct {
	URL        string  `json:"url"`
	Title      string  `json:"title,omitempty"`
	Language   string  `json:"language,omitempty"`
	Category   string  `json:"category,omitempty"`
	Tier       int     `json:"tier"`
	TrustScore float64 `json:"trust_score"`
	Source     string  `json:"source,omitempty"`
	FeedType   string  `json:"feed_type,omitempty"`
	Domain     string  `json:"domain,omitempty"`
	ItemCount  int     `json:"item_count,omitempty"`
}

// TierExportData represents the JSON export format for tiered feeds
type TierExportData struct {
	Version string     `json:"version"`
	Count   int        `json:"count"`
	Feeds   []TierFeed `json:"feeds"`
}

// TierCurator handles feed tier assignment and quality-based curation
// 피드풀 큐레이션 - 검증된 소스를 티어별로 분류
type TierCurator struct {
	MinFeeds int
	MaxFeeds int
}

// NewTierCurator creates a new TierCurator with default or custom feed limits
func NewTierCurator(minFeeds, maxFeeds int) *TierCurator {
	if minFeeds <= 0 {
		minFeeds = 10
	}
	if maxFeeds <= 0 {
		maxFeeds = 100
	}
	return &TierCurator{
		MinFeeds: minFeeds,
		MaxFeeds: maxFeeds,
	}
}

// CurateTiers takes validated sources, authority domains, and keywords,
// then assigns tiers and returns curated feeds
func (tc *TierCurator) CurateTiers(validatedSources []TierFeed, authorities []string, keywords []string) []TierFeed {
	// Build authority domain set (lowercased)
	authoritySet := make(map[string]bool)
	for _, a := range authorities {
		authoritySet[strings.ToLower(a)] = true
	}

	// Assign tier to each source
	for i := range validatedSources {
		tier := tc.AssignTier(&validatedSources[i], authoritySet)
		validatedSources[i].Tier = tier
	}

	// Sort by tier (ascending) then by trust score (descending)
	sort.Slice(validatedSources, func(i, j int) bool {
		if validatedSources[i].Tier != validatedSources[j].Tier {
			return validatedSources[i].Tier < validatedSources[j].Tier
		}
		return validatedSources[i].TrustScore > validatedSources[j].TrustScore
	})

	// Tier allocation limits
	tierLimits := map[int]int{
		1: tc.MaxFeeds / 3, // ~33% for tier 1
		2: tc.MaxFeeds / 2, // ~50% for tier 2
		3: tc.MaxFeeds / 4, // ~25% for tier 3
	}
	tierCounts := map[int]int{1: 0, 2: 0, 3: 0}

	// Curate with tier limits
	var curated []TierFeed
	for _, source := range validatedSources {
		tier := source.Tier
		if tier < 1 || tier > 3 {
			tier = 2
		}

		if tierCounts[tier] < tierLimits[tier] && len(curated) < tc.MaxFeeds {
			curated = append(curated, source)
			tierCounts[tier]++
		}
	}

	// Ensure minimum feeds are included
	if len(curated) < tc.MinFeeds {
		remaining := tc.MinFeeds - len(curated)
		curatedSet := make(map[string]bool)
		for _, f := range curated {
			curatedSet[f.URL] = true
		}

		for _, source := range validatedSources {
			if !curatedSet[source.URL] {
				curated = append(curated, source)
				remaining--
				if remaining <= 0 {
					break
				}
			}
		}
	}

	return curated
}

// AssignTier determines the tier (1, 2, or 3) for a source
// Tier 1: Authority sources, government, academic, high trust (0.8+)
// Tier 2: Default tier for moderate sources
// Tier 3: Low trust sources (<0.5)
func (tc *TierCurator) AssignTier(source *TierFeed, authoritySet map[string]bool) int {
	domain := strings.ToLower(source.Domain)
	if domain == "" {
		domain = extractDomain(source.URL)
	}
	trustScore := source.TrustScore
	sourceType := source.Source
	feedType := source.FeedType

	// Tier 1: 권위 소스
	// - Authority domain match
	// - Government sources
	// - Academic sources (OpenAlex, arXiv, Semantic Scholar)
	// - API feeds
	// - High trust score (0.8+)
	for auth := range authoritySet {
		if strings.Contains(domain, auth) {
			return 1
		}
	}

	// Government and academic source types
	tier1Sources := map[string]bool{
		"gov_search":       true,
		"openalex":         true,
		"arxiv":            true,
		"semantic_scholar": true,
	}
	if tier1Sources[sourceType] {
		return 1
	}

	if feedType == "api" {
		return 1
	}

	if trustScore >= 0.8 {
		return 1
	}

	// Tier 3: 낮은 신뢰도
	// - Trust score below 0.5
	// - Path discovery with low trust
	if trustScore < 0.5 {
		return 3
	}

	if sourceType == "path_discovery" && trustScore < 0.6 {
		return 3
	}

	// Tier 2: Default tier for everything else
	return 2
}

// ExportTierJSON exports curated feeds to a JSON file
func (tc *TierCurator) ExportTierJSON(curated []TierFeed, outputPath string) error {
	exportFeeds := make([]TierFeed, len(curated))
	for i, f := range curated {
		exportFeeds[i] = TierFeed{
			URL:        f.URL,
			Title:      f.Title,
			Language:   defaultStr(f.Language, "en"),
			Category:   f.Category,
			Tier:       defaultTier(f.Tier),
			TrustScore: defaultTrust(f.TrustScore),
			Source:     f.Source,
			FeedType:   defaultStr(f.FeedType, "rss"),
		}
	}

	exportData := TierExportData{
		Version: "1.0",
		Count:   len(exportFeeds),
		Feeds:   exportFeeds,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}

// QualityScore calculates a composite quality score for a feed
// considering trust score, tier, and other factors
func (tc *TierCurator) QualityScore(feed *TierFeed) float64 {
	baseScore := feed.TrustScore

	// Tier bonus: tier 1 gets +0.2, tier 2 gets 0, tier 3 gets -0.1
	switch feed.Tier {
	case 1:
		baseScore += 0.2
	case 3:
		baseScore -= 0.1
	}

	// Clamp to [0, 1]
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	if baseScore < 0.0 {
		baseScore = 0.0
	}

	return baseScore
}

// DeduplicateTierFeeds removes duplicate feeds by URL domain
func (tc *TierCurator) DeduplicateTierFeeds(feeds []TierFeed) []TierFeed {
	seen := make(map[string]bool)
	var result []TierFeed

	for _, f := range feeds {
		domain := f.Domain
		if domain == "" {
			domain = extractDomain(f.URL)
		}
		if !seen[domain] {
			seen[domain] = true
			result = append(result, f)
		}
	}

	return result
}

// DeduplicateByExactURL removes exact duplicate URLs
func (tc *TierCurator) DeduplicateByExactURL(feeds []TierFeed) []TierFeed {
	seen := make(map[string]bool)
	var result []TierFeed

	for _, f := range feeds {
		normalized := normalizeFeedURL(f.URL)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, f)
		}
	}

	return result
}

// RankByQualityScore sorts feeds by quality score (highest first)
func (tc *TierCurator) RankByQualityScore(feeds []TierFeed) []TierFeed {
	result := make([]TierFeed, len(feeds))
	copy(result, feeds)

	sort.Slice(result, func(i, j int) bool {
		return tc.QualityScore(&result[i]) > tc.QualityScore(&result[j])
	})

	return result
}

// SelectTopN returns the top N feeds by quality
func (tc *TierCurator) SelectTopN(feeds []TierFeed, n int) []TierFeed {
	ranked := tc.RankByQualityScore(feeds)
	if len(ranked) <= n {
		return ranked
	}
	return ranked[:n]
}

// GetTierDistribution returns feed counts by tier
func (tc *TierCurator) GetTierDistribution(feeds []TierFeed) map[int]int {
	stats := map[int]int{1: 0, 2: 0, 3: 0}
	for _, f := range feeds {
		tier := f.Tier
		if tier < 1 || tier > 3 {
			tier = 2
		}
		stats[tier]++
	}
	return stats
}

// ConvertFromFeed converts a basic Feed to a TierFeed
func ConvertFromFeed(f Feed, trustScore float64, source, feedType string) TierFeed {
	return TierFeed{
		URL:        f.URL,
		Title:      f.Title,
		Language:   f.Language,
		Domain:     f.Domain,
		ItemCount:  f.ItemCount,
		TrustScore: trustScore,
		Source:     source,
		FeedType:   feedType,
	}
}

// ConvertManyFromFeeds converts a slice of basic Feeds to TierFeeds
func ConvertManyFromFeeds(feeds []Feed, defaultTrustScore float64, source, feedType string) []TierFeed {
	result := make([]TierFeed, len(feeds))
	for i, f := range feeds {
		result[i] = ConvertFromFeed(f, defaultTrustScore, source, feedType)
	}
	return result
}

// Helper functions

func normalizeFeedURL(rawURL string) string {
	// Normalize: lowercase, remove trailing slash, remove www prefix
	lower := strings.ToLower(rawURL)
	lower = strings.TrimSuffix(lower, "/")
	// Remove protocol for comparison
	lower = strings.TrimPrefix(lower, "https://")
	lower = strings.TrimPrefix(lower, "http://")
	lower = strings.TrimPrefix(lower, "www.")
	return lower
}

func defaultStr(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func defaultTier(val int) int {
	if val < 1 || val > 3 {
		return 2
	}
	return val
}

func defaultTrust(val float64) float64 {
	if val == 0 {
		return 0.5
	}
	return val
}
