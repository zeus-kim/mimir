package discovery

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SeedConfig contains configuration for seed generation
type SeedConfig struct {
	PersonalSeedCount int // Seeds per language for bootstrap (default: 5000)
	HubSeedCount      int // Total seeds for hub pool (default: 200000)
	MinFeedsPerLang   int // Minimum feeds to include a language (default: 100)
}

// DefaultSeedConfig returns the default configuration
func DefaultSeedConfig() SeedConfig {
	return SeedConfig{
		PersonalSeedCount: 5000,
		HubSeedCount:      200000,
		MinFeedsPerLang:   100,
	}
}

// TargetLanguageRatios defines target internet share by language
var TargetLanguageRatios = map[string]float64{
	"en":    0.25, // English 25%
	"zh":    0.20, // Chinese 20%
	"es":    0.08, // Spanish 8%
	"ar":    0.05, // Arabic 5%
	"pt":    0.04, // Portuguese 4%
	"ja":    0.03, // Japanese 3%
	"ru":    0.03, // Russian 3%
	"de":    0.02, // German 2%
	"fr":    0.02, // French 2%
	"ko":    0.02, // Korean 2%
	"it":    0.01, // Italian 1%
	"hi":    0.01, // Hindi 1%
	"other": 0.24, // Other 24%
}

// SeedFeed represents a feed entry in the seed pool
type SeedFeed struct {
	URL      string `json:"url"`
	Domain   string `json:"domain"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Country  string `json:"country"`
	FeedType string `json:"feed_type"`
	Language string `json:"language,omitempty"`
}

// SeedGenerator generates seed pools from a feed database
type SeedGenerator struct {
	db     *sql.DB
	config SeedConfig
}

// NewSeedGenerator creates a new seed generator
func NewSeedGenerator(db *sql.DB, config SeedConfig) *SeedGenerator {
	return &SeedGenerator{
		db:     db,
		config: config,
	}
}

// SeedStats contains statistics about the generated seeds
type SeedStats struct {
	LanguageCounts   map[string]int
	BootstrapFiles   int
	HubTotalFeeds    int
	LanguageGaps     []LanguageGap
	CategoryBreakdown map[string]int
}

// LanguageGap represents a language that is underrepresented
type LanguageGap struct {
	Language    string
	Current     int
	Target      int
	Gap         int
	IsCritical  bool // Gap > 1000
}

// GenerateSeeds generates bootstrap and hub seed files
func (sg *SeedGenerator) GenerateSeeds(outputDir string) (*SeedStats, error) {
	// Create output directories
	bootstrapDir := filepath.Join(outputDir, "bootstrap")
	fullDir := filepath.Join(outputDir, "full")

	if err := os.MkdirAll(bootstrapDir, 0755); err != nil {
		return nil, fmt.Errorf("creating bootstrap dir: %w", err)
	}
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return nil, fmt.Errorf("creating full dir: %w", err)
	}

	// Collect feeds by language
	feedsByLang, err := sg.collectFeedsByLanguage()
	if err != nil {
		return nil, fmt.Errorf("collecting feeds: %w", err)
	}

	stats := &SeedStats{
		LanguageCounts:    make(map[string]int),
		CategoryBreakdown: make(map[string]int),
	}

	// Record language counts
	for lang, feeds := range feedsByLang {
		stats.LanguageCounts[lang] = len(feeds)
	}

	// Generate bootstrap seeds (per language)
	bootstrapCount, err := sg.generateBootstrapSeeds(feedsByLang, bootstrapDir)
	if err != nil {
		return nil, fmt.Errorf("generating bootstrap seeds: %w", err)
	}
	stats.BootstrapFiles = bootstrapCount

	// Generate hub seeds (all languages combined)
	hubStats, err := sg.generateHubSeeds(feedsByLang, fullDir)
	if err != nil {
		return nil, fmt.Errorf("generating hub seeds: %w", err)
	}
	stats.HubTotalFeeds = hubStats.total
	stats.CategoryBreakdown = hubStats.byCategory

	// Calculate language gaps
	stats.LanguageGaps = sg.calculateLanguageGaps(feedsByLang)

	return stats, nil
}

// collectFeedsByLanguage queries the database and groups feeds by language
func (sg *SeedGenerator) collectFeedsByLanguage() (map[string][]SeedFeed, error) {
	query := `
		SELECT url, domain, title, category, language, country, feed_type
		FROM feeds
		WHERE language != 'unknown' AND language != ''
		ORDER BY RANDOM()
	`

	rows, err := sg.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedsByLang := make(map[string][]SeedFeed)

	for rows.Next() {
		var f SeedFeed
		var title, category, country, feedType sql.NullString

		err := rows.Scan(&f.URL, &f.Domain, &title, &category, &f.Language, &country, &feedType)
		if err != nil {
			continue
		}

		f.Title = title.String
		f.Category = category.String
		f.Country = country.String
		f.FeedType = feedType.String

		feedsByLang[f.Language] = append(feedsByLang[f.Language], f)
	}

	return feedsByLang, rows.Err()
}

// generateBootstrapSeeds creates per-language seed files with category distribution
func (sg *SeedGenerator) generateBootstrapSeeds(feedsByLang map[string][]SeedFeed, outputDir string) (int, error) {
	filesCreated := 0

	for lang, feeds := range feedsByLang {
		if len(feeds) < sg.config.MinFeedsPerLang {
			continue
		}

		selected := sg.selectWithCategoryDistribution(feeds, sg.config.PersonalSeedCount)

		// Remove language field for output (it's in the filename)
		for i := range selected {
			selected[i].Language = ""
		}

		seedFile := filepath.Join(outputDir, lang+".json")
		if err := writeJSONFile(seedFile, selected); err != nil {
			return filesCreated, fmt.Errorf("writing %s: %w", seedFile, err)
		}

		filesCreated++
	}

	return filesCreated, nil
}

type hubStats struct {
	total      int
	byCategory map[string]int
}

// generateHubSeeds creates the full seed pool split by category
func (sg *SeedGenerator) generateHubSeeds(feedsByLang map[string][]SeedFeed, outputDir string) (*hubStats, error) {
	// Combine all feeds
	var allFeeds []SeedFeed
	for _, feeds := range feedsByLang {
		allFeeds = append(allFeeds, feeds...)
	}

	// Deduplicate by URL
	seen := make(map[string]bool)
	var uniqueFeeds []SeedFeed
	for _, f := range allFeeds {
		if !seen[f.URL] {
			seen[f.URL] = true
			uniqueFeeds = append(uniqueFeeds, f)
		}
	}

	// Limit to max hub size
	if len(uniqueFeeds) > sg.config.HubSeedCount {
		uniqueFeeds = uniqueFeeds[:sg.config.HubSeedCount]
	}

	// Split by category
	categories := map[string][]SeedFeed{
		"news":  {},
		"blogs": {},
		"tech":  {},
		"other": {},
	}

	for _, f := range uniqueFeeds {
		switch f.Category {
		case "news":
			categories["news"] = append(categories["news"], f)
		case "blog":
			categories["blogs"] = append(categories["blogs"], f)
		case "tech":
			categories["tech"] = append(categories["tech"], f)
		default:
			categories["other"] = append(categories["other"], f)
		}
	}

	// Write category files
	stats := &hubStats{
		total:      len(uniqueFeeds),
		byCategory: make(map[string]int),
	}

	for name, feeds := range categories {
		outFile := filepath.Join(outputDir, name+".json")
		if err := writeJSONFile(outFile, feeds); err != nil {
			return nil, fmt.Errorf("writing %s: %w", outFile, err)
		}
		stats.byCategory[name] = len(feeds)
	}

	return stats, nil
}

// selectWithCategoryDistribution selects feeds using round-robin across categories
func (sg *SeedGenerator) selectWithCategoryDistribution(feeds []SeedFeed, target int) []SeedFeed {
	// Group by category
	byCategory := make(map[string][]SeedFeed)
	for _, f := range feeds {
		cat := f.Category
		if cat == "" {
			cat = "other"
		}
		byCategory[cat] = append(byCategory[cat], f)
	}

	// Get sorted category list for deterministic ordering
	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Round-robin selection
	actualTarget := min(target, len(feeds))
	selected := make([]SeedFeed, 0, actualTarget)
	idx := 0

	for len(selected) < actualTarget {
		cat := categories[idx%len(categories)]
		if len(byCategory[cat]) > 0 {
			selected = append(selected, byCategory[cat][0])
			byCategory[cat] = byCategory[cat][1:]
		}
		idx++

		// Check if all categories exhausted
		allEmpty := true
		for _, feeds := range byCategory {
			if len(feeds) > 0 {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			break
		}
	}

	return selected
}

// calculateLanguageGaps identifies underrepresented languages
func (sg *SeedGenerator) calculateLanguageGaps(feedsByLang map[string][]SeedFeed) []LanguageGap {
	// Calculate total unique feeds
	total := 0
	for _, feeds := range feedsByLang {
		total += len(feeds)
	}

	var gaps []LanguageGap

	for lang, targetRatio := range TargetLanguageRatios {
		if lang == "other" {
			continue
		}

		current := len(feedsByLang[lang])
		targetCount := int(targetRatio * float64(sg.config.HubSeedCount))
		gap := targetCount - current

		if gap > 0 {
			gaps = append(gaps, LanguageGap{
				Language:   lang,
				Current:    current,
				Target:     targetCount,
				Gap:        gap,
				IsCritical: gap > 1000,
			})
		}
	}

	// Sort by gap size descending
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Gap > gaps[j].Gap
	})

	return gaps
}

// writeJSONFile writes data as formatted JSON
func writeJSONFile(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(data)
}

// PrintStats outputs seed generation statistics
func (stats *SeedStats) PrintStats() {
	fmt.Println("=== Feed Counts by Language ===")

	// Sort languages by count descending
	type langCount struct {
		lang  string
		count int
	}
	sorted := make([]langCount, 0, len(stats.LanguageCounts))
	for lang, count := range stats.LanguageCounts {
		sorted = append(sorted, langCount{lang, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	for _, lc := range sorted {
		fmt.Printf("  %-10s: %8d\n", lc.lang, lc.count)
	}

	fmt.Printf("\n=== Bootstrap Seeds ===\n")
	fmt.Printf("  Languages: %d\n", stats.BootstrapFiles)

	fmt.Printf("\n=== Hub Seeds ===\n")
	fmt.Printf("  Total: %d\n", stats.HubTotalFeeds)
	for cat, count := range stats.CategoryBreakdown {
		fmt.Printf("  %s: %d\n", cat, count)
	}

	if len(stats.LanguageGaps) > 0 {
		fmt.Printf("\n=== Language Gaps (vs Target) ===\n")
		for _, gap := range stats.LanguageGaps {
			marker := ""
			if gap.IsCritical {
				marker = " [CRITICAL]"
			}
			fmt.Printf("  %s: current %d, target %d, gap %d%s\n",
				gap.Language, gap.Current, gap.Target, gap.Gap, marker)
		}
	}
}

// GenerateSeedsFromPath is a convenience function that opens the DB and generates seeds
func GenerateSeedsFromPath(dbPath, outputDir string, config *SeedConfig) (*SeedStats, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	cfg := DefaultSeedConfig()
	if config != nil {
		cfg = *config
	}

	generator := NewSeedGenerator(db, cfg)
	return generator.GenerateSeeds(outputDir)
}
