// Package bootstrap provides the vertical bootstrapping orchestrator.
// Port of ~/Projects/mimir/tools/bootstrap_vertical.py
//
// Pipeline:
//  1. Domain Understanding (LLM)
//  2. Ontology Generation
//  3. Multi-Source Discovery (RSS, YouTube, Gov, Academic, GitHub)
//  4. Source Validation & Scoring
//  5. Feed Pool Curation
//  6. Initial Crawl & Index
//  7. Ready
package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/user/mimir-mcp/internal/discovery"
	"github.com/user/mimir-mcp/internal/ranking"
)

// BootstrapConfig holds configuration for vertical bootstrapping
type BootstrapConfig struct {
	Topic          string   `json:"topic"`
	Languages      []string `json:"languages"`
	Depth          string   `json:"depth"` // quick, standard, thorough
	MinFeeds       int      `json:"min_feeds"`
	MaxFeeds       int      `json:"max_feeds"`
	TrustThreshold float64  `json:"trust_threshold"`
}

// DefaultConfig returns a BootstrapConfig with sensible defaults
func DefaultConfig(topic string) *BootstrapConfig {
	return &BootstrapConfig{
		Topic:          topic,
		Languages:      []string{"en", "ko"},
		Depth:          "standard",
		MinFeeds:       10,
		MaxFeeds:       100,
		TrustThreshold: 0.5,
	}
}

// BootstrapResult holds the result of a bootstrap operation
type BootstrapResult struct {
	Success           bool              `json:"success"`
	VerticalName      string            `json:"vertical_name"`
	DBPath            string            `json:"db_path"`
	DomainInfo        *DomainInfo       `json:"domain_info"`
	SourcesDiscovered map[string]int    `json:"sources_discovered"`
	FeedsCurated      int               `json:"feeds_curated"`
	DocumentsIndexed  int               `json:"documents_indexed"`
	FitPercent        float64           `json:"fit_percent"`
	DurationSeconds   float64           `json:"duration_seconds"`
	Errors            []string          `json:"errors,omitempty"`
}

// DomainInfo holds analyzed domain information
type DomainInfo struct {
	Definition     string            `json:"definition"`
	Subtopics      []string          `json:"subtopics"`
	Keywords       []string          `json:"keywords"`
	KeywordsByType map[string][]string `json:"keywords_by_type,omitempty"`
	Authorities    []string          `json:"authorities"`
	RelatedFields  []string          `json:"related_fields,omitempty"`
	FitExpr        string            `json:"fit_expr"`
	SearchQueries  []string          `json:"search_queries,omitempty"`
}

// Source represents a discovered source
type Source struct {
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Type        string  `json:"type"` // rss, youtube, github, academic, government, blog
	Language    string  `json:"language,omitempty"`
	Category    string  `json:"category,omitempty"`
	Tier        int     `json:"tier"` // 1=authority, 2=standard, 3=low
	TrustScore  float64 `json:"trust_score"`
	TFIDFScore  float64 `json:"tfidf_score,omitempty"`
	Validated   bool    `json:"validated"`
	FeedType    string  `json:"feed_type,omitempty"` // rss, api, github
	ItemCount   int     `json:"item_count,omitempty"`
}

// DomainAnalyzer interface for domain analysis (LLM-based or rule-based)
type DomainAnalyzer interface {
	Analyze(ctx context.Context, topic string, languages []string, depth string) (*DomainInfo, error)
}

// SourceValidator interface for source validation
type SourceValidator interface {
	ValidateBatch(ctx context.Context, sources []Source, languages []string, threshold float64) []Source
}

// FeedCurator interface for feed curation
type FeedCurator interface {
	Curate(sources []Source, authorities, keywords []string) []Source
}

// Discoverer interface for source discovery
type Discoverer interface {
	Name() string
	Discover(ctx context.Context, keywords []string, languages []string, authorities []string, limit int) ([]Source, error)
}

// VerticalBootstrapper orchestrates the vertical bootstrap process
type VerticalBootstrapper struct {
	config     *BootstrapConfig
	projectDir string
	homeDir    string
	errors     []string
	mu         sync.Mutex

	// Pluggable components
	analyzer    DomainAnalyzer
	validator   SourceValidator
	curator     FeedCurator
	discoverers []Discoverer
}

// NewVerticalBootstrapper creates a new bootstrapper with configuration
func NewVerticalBootstrapper(config *BootstrapConfig) *VerticalBootstrapper {
	homeDir, _ := os.UserHomeDir()

	return &VerticalBootstrapper{
		config:     config,
		homeDir:    homeDir,
		projectDir: filepath.Join(homeDir, "Projects", "mimir"),
		errors:     make([]string, 0),
	}
}

// SetAnalyzer sets a custom domain analyzer
func (vb *VerticalBootstrapper) SetAnalyzer(a DomainAnalyzer) {
	vb.analyzer = a
}

// SetValidator sets a custom source validator
func (vb *VerticalBootstrapper) SetValidator(v SourceValidator) {
	vb.validator = v
}

// SetCurator sets a custom feed curator
func (vb *VerticalBootstrapper) SetCurator(c FeedCurator) {
	vb.curator = c
}

// AddDiscoverer adds a source discoverer
func (vb *VerticalBootstrapper) AddDiscoverer(d Discoverer) {
	vb.discoverers = append(vb.discoverers, d)
}

// Run executes the full bootstrap pipeline
func (vb *VerticalBootstrapper) Run(ctx context.Context) (*BootstrapResult, error) {
	startTime := time.Now()

	// Generate vertical name from topic
	verticalName := vb.normalizeName(vb.config.Topic)
	dbPath := filepath.Join(vb.homeDir, fmt.Sprintf(".mine-%s", verticalName), "lite.db")

	log.Printf("Mimir Vertical Bootstrap: %s", vb.config.Topic)
	log.Printf("   Vertical: %s", verticalName)
	log.Printf("   Languages: %s", strings.Join(vb.config.Languages, ", "))
	log.Printf("   Depth: %s", vb.config.Depth)

	// Stage 1-2: Domain Analysis & Ontology
	log.Println("Stage 1-2: Domain Understanding & Ontology")
	domainInfo, err := vb.analyzeDomain(ctx)
	if err != nil {
		return vb.failResult(verticalName, dbPath, fmt.Sprintf("Domain analysis failed: %v", err)), nil
	}

	// Stage 3: Multi-Source Discovery
	log.Println("Stage 3: Multi-Source Discovery")
	sources := vb.discoverSources(ctx, domainInfo)

	// Stage 4: Source Validation & Scoring
	log.Println("Stage 4: Source Validation & Scoring")
	validated := vb.validateSources(ctx, sources, domainInfo)

	// Stage 5: Feed Pool Curation
	log.Println("Stage 5: Feed Pool Curation")
	curated := vb.curateFeeds(validated, domainInfo)

	// Stage 6: DB Creation & Indexing
	log.Println("Stage 6: Initial Crawl & Index")
	docsIndexed := vb.createAndIndex(ctx, verticalName, curated, domainInfo)

	// Stage 7: Complete
	log.Println("Stage 7: Ready")
	fit := vb.measureFit(dbPath, domainInfo.FitExpr)

	duration := time.Since(startTime)

	result := &BootstrapResult{
		Success:           true,
		VerticalName:      verticalName,
		DBPath:            dbPath,
		DomainInfo:        domainInfo,
		SourcesDiscovered: vb.countSources(sources),
		FeedsCurated:      len(curated),
		DocumentsIndexed:  docsIndexed,
		FitPercent:        fit,
		DurationSeconds:   duration.Seconds(),
		Errors:            vb.errors,
	}

	vb.printSummary(result)
	if err := vb.saveConfig(verticalName, domainInfo, curated); err != nil {
		vb.addError("Config save: " + err.Error())
	}

	return result, nil
}

// normalizeName converts topic to vertical name
func (vb *VerticalBootstrapper) normalizeName(topic string) string {
	name := strings.ToLower(strings.TrimSpace(topic))

	// Replace non-alphanumeric (keeping Korean characters)
	re := regexp.MustCompile(`[^a-z0-9가-힣]+`)
	name = re.ReplaceAllString(name, "-")

	// Collapse multiple dashes
	re = regexp.MustCompile(`-+`)
	name = re.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	if len(name) > 30 {
		name = name[:30]
	}
	if name == "" {
		name = "vertical"
	}

	return name
}

// analyzeDomain performs Stage 1-2: LLM domain analysis
func (vb *VerticalBootstrapper) analyzeDomain(ctx context.Context) (*DomainInfo, error) {
	if vb.analyzer != nil {
		info, err := vb.analyzer.Analyze(ctx, vb.config.Topic, vb.config.Languages, vb.config.Depth)
		if err == nil && len(info.Keywords) > 0 {
			log.Printf("  Definition: %.60s...", info.Definition)
			log.Printf("  Subtopics: %s", strings.Join(info.Subtopics[:min(5, len(info.Subtopics))], ", "))
			log.Printf("  Keywords: %d", len(info.Keywords))
			log.Printf("  Authorities: %d", len(info.Authorities))
			return info, nil
		}
		vb.addError(fmt.Sprintf("Domain analysis: %v", err))
	}

	// Fallback to rule-based analysis
	log.Println("  (No LLM - using rule-based analysis)")
	return vb.fallbackDomainAnalysis(), nil
}

// fallbackDomainAnalysis provides rule-based analysis when LLM is unavailable
func (vb *VerticalBootstrapper) fallbackDomainAnalysis() *DomainInfo {
	topic := vb.config.Topic
	topicLower := strings.ToLower(topic)

	// Known domain expansions
	expansions := map[string]struct {
		keywords    []string
		authorities []string
		subtopics   []string
	}{
		"bakery": {
			keywords:    []string{"bread", "baking", "pastry", "sourdough", "croissant", "cake", "빵", "베이커리", "제빵"},
			authorities: []string{"kingarthurbaking.com", "theperfectloaf.com", "seriouseats.com"},
			subtopics:   []string{"sourdough", "pastry", "cake", "bread", "dessert"},
		},
		"coffee": {
			keywords:    []string{"coffee", "espresso", "barista", "brewing", "roasting", "커피", "에스프레소", "바리스타"},
			authorities: []string{"dailycoffeenews.com", "perfectdailygrind.com", "sprudge.com"},
			subtopics:   []string{"espresso", "brewing", "roasting", "latte art", "coffee beans"},
		},
		"wine": {
			keywords:    []string{"wine", "vineyard", "winery", "sommelier", "grape", "vintage", "와인", "포도주"},
			authorities: []string{"winespectator.com", "decanter.com", "wine-searcher.com"},
			subtopics:   []string{"red wine", "white wine", "champagne", "vineyard", "tasting"},
		},
		"pharma": {
			keywords:    []string{"pharmaceutical", "drug", "FDA", "clinical", "trial", "biotech", "제약", "신약", "임상"},
			authorities: []string{"fda.gov", "clinicaltrials.gov", "nih.gov", "nature.com"},
			subtopics:   []string{"clinical trials", "drug approval", "biotech", "vaccines", "therapy"},
		},
		"legal": {
			keywords:    []string{"law", "legal", "attorney", "court", "lawsuit", "litigation", "법률", "변호사", "소송"},
			authorities: []string{"law.cornell.edu", "scotusblog.com", "lawfare.blog"},
			subtopics:   []string{"corporate law", "litigation", "contracts", "intellectual property"},
		},
	}

	// Find matching expansion
	var keywords, authorities, subtopics []string
	for key, expansion := range expansions {
		if strings.Contains(topicLower, key) {
			keywords = expansion.keywords
			authorities = expansion.authorities
			subtopics = expansion.subtopics
			break
		}
	}

	// Default expansion if no match
	if keywords == nil {
		words := regexp.MustCompile(`\w+`).FindAllString(topicLower, -1)
		keywords = words
		for _, w := range words {
			if len(w) > 3 {
				keywords = append(keywords, w+"s", w+"ing")
			}
		}
		subtopics = words
	}

	// Build FTS expression
	fitExpr := strings.Join(keywords[:min(10, len(keywords))], " OR ")

	return &DomainInfo{
		Definition:    fmt.Sprintf("%s related content", topic),
		Subtopics:     subtopics,
		Keywords:      keywords,
		Authorities:   authorities,
		RelatedFields: []string{},
		FitExpr:       fitExpr,
		SearchQueries: []string{
			topic + " RSS feed",
			topic + " blog",
			"best " + topic + " sites",
			topic + " news",
		},
	}
}

// discoverSources performs Stage 3: Multi-source discovery
func (vb *VerticalBootstrapper) discoverSources(ctx context.Context, domainInfo *DomainInfo) map[string][]Source {
	sources := map[string][]Source{
		"authority":  {},
		"rss":        {},
		"youtube":    {},
		"government": {},
		"academic":   {},
		"github":     {},
		"blogs":      {},
	}

	keywords := domainInfo.Keywords
	if len(keywords) == 0 {
		keywords = []string{vb.config.Topic}
	}
	authorities := domainInfo.Authorities

	// Get authority feeds first (always included)
	authorityFeeds := vb.getAuthorityFeeds(vb.config.Topic, keywords)
	sources["authority"] = authorityFeeds
	if len(authorityFeeds) > 0 {
		log.Printf("  authority: %d (confirmed)", len(authorityFeeds))
	}

	// Discovery limits by depth
	limits := map[string]map[string]int{
		"quick":    {"rss": 10, "youtube": 5, "government": 3, "academic": 5, "github": 3, "blogs": 5},
		"standard": {"rss": 30, "youtube": 15, "government": 10, "academic": 15, "github": 10, "blogs": 15},
		"thorough": {"rss": 50, "youtube": 30, "government": 20, "academic": 30, "github": 20, "blogs": 30},
	}
	limitMap := limits[vb.config.Depth]
	if limitMap == nil {
		limitMap = limits["standard"]
	}

	// Use built-in discoverers if none provided
	if len(vb.discoverers) == 0 {
		vb.discoverers = vb.defaultDiscoverers()
	}

	// Parallel discovery
	var wg sync.WaitGroup
	resultCh := make(chan struct {
		name    string
		sources []Source
		err     error
	}, len(vb.discoverers))

	for _, d := range vb.discoverers {
		wg.Add(1)
		go func(disc Discoverer) {
			defer wg.Done()
			name := disc.Name()
			limit := limitMap[name]
			if limit == 0 {
				limit = 10
			}

			result, err := disc.Discover(ctx, keywords, vb.config.Languages, authorities, limit)
			resultCh <- struct {
				name    string
				sources []Source
				err     error
			}{name, result, err}
		}(d)
	}

	// Wait and collect results
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for r := range resultCh {
		if r.err != nil {
			vb.addError(fmt.Sprintf("%s discovery: %v", r.name, r.err))
			log.Printf("  %s: failed - %v", r.name, r.err)
		} else {
			sources[r.name] = r.sources
			log.Printf("  %s: %d discovered", r.name, len(r.sources))
		}
	}

	return sources
}

// getAuthorityFeeds returns known authority feeds for the topic
func (vb *VerticalBootstrapper) getAuthorityFeeds(topic string, keywords []string) []Source {
	// This would integrate with authority_sources.py equivalent
	// For now, return empty - actual implementation would check known authority lists
	return []Source{}
}

// defaultDiscoverers returns built-in discoverers
func (vb *VerticalBootstrapper) defaultDiscoverers() []Discoverer {
	return []Discoverer{
		&RSSDiscovererAdapter{},
		&GitHubDiscovererAdapter{},
	}
}

// validateSources performs Stage 4: Source validation & scoring (3 stages)
func (vb *VerticalBootstrapper) validateSources(ctx context.Context, sources map[string][]Source, domainInfo *DomainInfo) []Source {
	keywords := domainInfo.Keywords

	// Authority sources skip validation (always included)
	authoritySources := sources["authority"]
	for i := range authoritySources {
		authoritySources[i].Type = "authority"
		authoritySources[i].Validated = true
		authoritySources[i].TrustScore = 0.95
	}

	// Collect all non-authority sources
	var allSources []Source
	for sourceType, items := range sources {
		if sourceType == "authority" {
			continue
		}
		for _, item := range items {
			item.Type = sourceType
			allSources = append(allSources, item)
		}
	}

	log.Printf("Validation target: %d sources (+%d authority)", len(allSources), len(authoritySources))

	// Stage 1: Technical validation (URL accessible, RSS parseable)
	var validated []Source
	if vb.validator != nil {
		validated = vb.validator.ValidateBatch(ctx, allSources, vb.config.Languages, vb.config.TrustThreshold)
	} else {
		// Default: basic URL validation
		validated = vb.basicValidation(allSources)
	}
	log.Printf("  Stage 1 technical validation: %d passed", len(validated))

	// Stage 2: TF-IDF quick filter (before LLM calls)
	if len(keywords) > 0 && len(validated) > 0 {
		classifier := ranking.NewRelevanceClassifier(keywords, vb.config.Topic)

		var tfidfPassed []Source
		for _, item := range validated {
			text := strings.Join([]string{
				item.Title,
				item.Description,
				item.URL,
				item.Category,
			}, " ")

			score := classifier.Score(text)
			item.TFIDFScore = score

			// Threshold 0.15 (generous - URL alone can pass)
			if score >= 0.15 {
				tfidfPassed = append(tfidfPassed, item)
			} else if item.Type == "academic" || item.Type == "government" {
				// Trusted source types pass with default score
				item.TFIDFScore = 0.3
				tfidfPassed = append(tfidfPassed, item)
			}
		}

		log.Printf("  Stage 2 TF-IDF filter: %d passed (%d excluded)", len(tfidfPassed), len(validated)-len(tfidfPassed))
		validated = tfidfPassed
	}

	// Stage 3: LLM content validation would go here (expensive, so only on remaining)
	// This would integrate with content_validator.py equivalent
	log.Printf("  Stage 3 LLM validation: %d passed (skipped - no LLM)", len(validated))

	// Bayesian ranking (rank by similarity to good sources)
	if len(validated) > 0 && len(authoritySources) > 0 {
		ranker := ranking.NewBayesianRanker(2.0)

		// Learn from authority sources as positive examples
		for _, auth := range authoritySources {
			text := auth.Title + " " + auth.Description
			ranker.AddPositive(text)
		}

		// Prepare candidates for ranking
		candidates := make([]map[string]interface{}, len(validated))
		for i, item := range validated {
			candidates[i] = map[string]interface{}{
				"text":  item.Title + " " + item.Description,
				"index": i,
			}
		}

		ranked := ranker.Rank(candidates)
		reordered := make([]Source, len(ranked))
		for i, r := range ranked {
			idx := r.Data.(map[string]interface{})["index"].(int)
			reordered[i] = validated[idx]
			reordered[i].TrustScore = r.BayesianScore
		}
		validated = reordered
		log.Println("  Bayesian ranking complete")
	}

	// Combine authority + validated sources
	final := append(authoritySources, validated...)
	log.Printf("  Final validation complete: %d", len(final))

	return final
}

// basicValidation performs simple URL validation
func (vb *VerticalBootstrapper) basicValidation(sources []Source) []Source {
	var validated []Source
	for _, s := range sources {
		if s.URL != "" {
			s.Validated = true
			s.TrustScore = 0.5
			validated = append(validated, s)
		}
	}
	return validated
}

// curateFeeds performs Stage 5: Feed pool curation
func (vb *VerticalBootstrapper) curateFeeds(validated []Source, domainInfo *DomainInfo) []Source {
	if vb.curator != nil {
		curated := vb.curator.Curate(validated, domainInfo.Authorities, domainInfo.Keywords)
		vb.logTierCounts(curated)
		return curated
	}

	// Default curation: assign tiers and sort
	curated := vb.defaultCuration(validated, domainInfo)
	vb.logTierCounts(curated)
	return curated
}

// defaultCuration performs basic tier assignment and curation
func (vb *VerticalBootstrapper) defaultCuration(sources []Source, domainInfo *DomainInfo) []Source {
	authoritySet := make(map[string]bool)
	for _, a := range domainInfo.Authorities {
		authoritySet[strings.ToLower(a)] = true
	}

	// Assign tiers
	for i := range sources {
		sources[i].Tier = vb.assignTier(&sources[i], authoritySet)
	}

	// Sort by tier (ascending) then trust score (descending)
	// Simple bubble sort for clarity
	for i := 0; i < len(sources); i++ {
		for j := i + 1; j < len(sources); j++ {
			if sources[j].Tier < sources[i].Tier ||
				(sources[j].Tier == sources[i].Tier && sources[j].TrustScore > sources[i].TrustScore) {
				sources[i], sources[j] = sources[j], sources[i]
			}
		}
	}

	// Apply tier limits
	tierCounts := map[int]int{1: 0, 2: 0, 3: 0}
	tierLimits := map[int]int{
		1: vb.config.MaxFeeds / 3,
		2: vb.config.MaxFeeds / 2,
		3: vb.config.MaxFeeds / 4,
	}

	var curated []Source
	for _, s := range sources {
		tier := s.Tier
		if tierCounts[tier] < tierLimits[tier] && len(curated) < vb.config.MaxFeeds {
			curated = append(curated, s)
			tierCounts[tier]++
		}
	}

	// Ensure minimum feeds
	if len(curated) < vb.config.MinFeeds {
		remaining := vb.config.MinFeeds - len(curated)
		for _, s := range sources {
			found := false
			for _, c := range curated {
				if c.URL == s.URL {
					found = true
					break
				}
			}
			if !found {
				curated = append(curated, s)
				remaining--
				if remaining <= 0 {
					break
				}
			}
		}
	}

	return curated
}

// assignTier assigns a tier to a source
func (vb *VerticalBootstrapper) assignTier(source *Source, authoritySet map[string]bool) int {
	// Extract domain from URL
	domain := extractDomain(source.URL)

	// Tier 1: Authority sources
	for auth := range authoritySet {
		if strings.Contains(domain, auth) {
			return 1
		}
	}
	if source.Type == "government" || source.Type == "academic" {
		return 1
	}
	if source.FeedType == "api" {
		return 1
	}
	if source.TrustScore >= 0.8 {
		return 1
	}

	// Tier 3: Low trust
	if source.TrustScore < 0.5 {
		return 3
	}

	// Tier 2: Default
	return 2
}

// logTierCounts logs the tier distribution
func (vb *VerticalBootstrapper) logTierCounts(sources []Source) {
	tierCounts := make(map[int]int)
	for _, s := range sources {
		tierCounts[s.Tier]++
	}
	log.Printf("  Curation complete: %d feeds", len(sources))
	for tier := 1; tier <= 3; tier++ {
		if count := tierCounts[tier]; count > 0 {
			log.Printf("     Tier %d: %d", tier, count)
		}
	}
}

// createAndIndex performs Stage 6: DB creation & indexing
func (vb *VerticalBootstrapper) createAndIndex(ctx context.Context, verticalName string, feeds []Source, domainInfo *DomainInfo) int {
	verticalDir := filepath.Join(vb.homeDir, fmt.Sprintf(".mine-%s", verticalName))
	dbPath := filepath.Join(verticalDir, "lite.db")

	// Create directory
	if err := os.MkdirAll(verticalDir, 0755); err != nil {
		vb.addError(fmt.Sprintf("Create directory: %v", err))
		return 0
	}

	// Backup existing DB
	if _, err := os.Stat(dbPath); err == nil {
		backupPath := fmt.Sprintf("%s.bak.%d", dbPath, time.Now().Unix())
		if err := os.Rename(dbPath, backupPath); err == nil {
			log.Printf("   Existing DB backed up: %s", filepath.Base(backupPath))
		}
	}

	// Run mimir init
	mimirBin := filepath.Join(vb.homeDir, "bin", "mimir")
	if _, err := os.Stat(mimirBin); os.IsNotExist(err) {
		mimirBin = filepath.Join(vb.projectDir, "mimir")
	}

	cmd := exec.CommandContext(ctx, mimirBin, "init", "-db", dbPath)
	cmd.Env = append(os.Environ(), "MIMIR_SEED_NEWS=0")
	if err := cmd.Run(); err != nil {
		vb.addError(fmt.Sprintf("mimir init: %v", err))
		// Continue anyway - we'll create the tables manually
	}

	// Register feeds in database
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		vb.addError(fmt.Sprintf("Open DB: %v", err))
		return 0
	}
	defer db.Close()

	// Ensure feeds table exists
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS feeds (
		id INTEGER PRIMARY KEY,
		url TEXT UNIQUE NOT NULL,
		name TEXT,
		language TEXT DEFAULT 'en',
		category TEXT,
		status TEXT DEFAULT 'active',
		tier INTEGER DEFAULT 2,
		created_at INTEGER
	)`)

	// Ensure documents table exists
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY,
		url TEXT UNIQUE,
		title TEXT,
		content TEXT,
		summary TEXT,
		feed_id INTEGER,
		published_at INTEGER,
		created_at INTEGER
	)`)

	// Ensure FTS table exists
	_, _ = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		title, summary, content='documents', content_rowid='id'
	)`)

	now := time.Now().Unix()
	registered := 0
	for _, feed := range feeds {
		_, err := db.Exec(`
			INSERT OR IGNORE INTO feeds (url, name, language, category, status, tier, created_at)
			VALUES (?, ?, ?, ?, 'active', ?, ?)
		`, feed.URL, truncate(feed.Title, 200), feed.Language, feed.Category, feed.Tier, now)
		if err == nil {
			registered++
		} else {
			vb.addError(fmt.Sprintf("Feed insert: %v", err))
		}
	}

	log.Printf("  Feeds registered: %d", registered)

	// Run fetch
	cmd = exec.CommandContext(ctx, mimirBin, "fetch", "-db", dbPath, "-limit-feeds", "50", "-items-per-feed", "30")
	_ = cmd.Run() // Ignore errors - fetch may not be available

	// Count documents
	var docs int
	row := db.QueryRow("SELECT COUNT(*) FROM documents")
	if err := row.Scan(&docs); err != nil {
		docs = 0
	}

	log.Printf("  RSS collected: %d documents", docs)

	// If few documents, try web crawling
	if docs < 10 {
		crawled := vb.crawlWebsites(ctx, dbPath, domainInfo)
		docs += crawled
	}

	log.Printf("  Total documents: %d", docs)
	return docs
}

// crawlWebsites performs web crawling for RSS-less sites
func (vb *VerticalBootstrapper) crawlWebsites(ctx context.Context, dbPath string, domainInfo *DomainInfo) int {
	// This would integrate with web_content_fetcher.py equivalent
	// For now, return 0 - actual implementation would crawl authority sites
	return 0
}

// measureFit measures the fit percentage
func (vb *VerticalBootstrapper) measureFit(dbPath string, expr string) float64 {
	if expr == "" {
		return 0.0
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0.0
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0.0
	}
	defer db.Close()

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&total); err != nil || total == 0 {
		return 0.0
	}

	var matched int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH ?", expr).Scan(&matched); err != nil {
		return 0.0
	}

	return float64(matched) / float64(total) * 100.0
}

// saveConfig saves configuration to curate_config.json
func (vb *VerticalBootstrapper) saveConfig(verticalName string, domainInfo *DomainInfo, feeds []Source) error {
	configPath := filepath.Join(vb.projectDir, "curate_config.json")

	// Load existing config
	config := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &config)
	}

	// Collect feed URLs (tier 1-2 only, max 50)
	var feedURLs []string
	for _, f := range feeds {
		if f.Tier <= 2 && len(feedURLs) < 50 {
			feedURLs = append(feedURLs, f.URL)
		}
	}

	config[verticalName] = map[string]interface{}{
		"expr":          domainInfo.FitExpr,
		"language":      strings.Join(vb.config.Languages, ","),
		"auto_discover": false,
		"feed_urls":     feedURLs,
		"authorities":   domainInfo.Authorities,
		"keywords":      domainInfo.Keywords,
		"description":   truncate(domainInfo.Definition, 200),
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	log.Println("  Config saved: curate_config.json")
	return nil
}

// printSummary prints the result summary
func (vb *VerticalBootstrapper) printSummary(result *BootstrapResult) {
	log.Println("")
	log.Println("Bootstrap Complete!")
	log.Println(strings.Repeat("=", 60))
	log.Printf("Vertical: %s", result.VerticalName)
	log.Printf("DB: %s", result.DBPath)
	log.Printf("Feeds: %d", result.FeedsCurated)
	log.Printf("Documents: %d", result.DocumentsIndexed)
	log.Printf("Fit: %.1f%%", result.FitPercent)
	log.Printf("Duration: %.1f seconds", result.DurationSeconds)

	if len(result.Errors) > 0 {
		log.Printf("")
		log.Printf("Warnings: %d", len(result.Errors))
		for i, err := range result.Errors {
			if i >= 5 {
				break
			}
			log.Printf("   - %s", err)
		}
	}

	log.Println("")
	log.Println("Next steps:")
	log.Printf("  - Search: ~/bin/mimir search -db %s 'query'", result.DBPath)
	log.Printf("  - Fetch: ~/bin/mimir fetch -db %s", result.DBPath)
	log.Printf("  - MCP: MIMIR_DATA_DIR=~/.mine-%s mimir-mcp", result.VerticalName)
}

// failResult creates a failure result
func (vb *VerticalBootstrapper) failResult(name, dbPath, reason string) *BootstrapResult {
	return &BootstrapResult{
		Success:      false,
		VerticalName: name,
		DBPath:       dbPath,
		DomainInfo:   &DomainInfo{},
		Errors:       []string{reason},
	}
}

// countSources counts sources by type
func (vb *VerticalBootstrapper) countSources(sources map[string][]Source) map[string]int {
	counts := make(map[string]int)
	for k, v := range sources {
		counts[k] = len(v)
	}
	return counts
}

// addError adds an error to the error list (thread-safe)
func (vb *VerticalBootstrapper) addError(err string) {
	vb.mu.Lock()
	defer vb.mu.Unlock()
	vb.errors = append(vb.errors, err)
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func extractDomain(u string) string {
	// Simple domain extraction
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx != -1 {
		u = u[:idx]
	}
	return strings.ToLower(u)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RSSDiscovererAdapter adapts the existing discovery.RSSDiscoverer
type RSSDiscovererAdapter struct {
	inner *discovery.RSSDiscoverer
}

func (d *RSSDiscovererAdapter) Name() string { return "rss" }

func (d *RSSDiscovererAdapter) Discover(ctx context.Context, keywords []string, languages []string, authorities []string, limit int) ([]Source, error) {
	if d.inner == nil {
		d.inner = discovery.NewRSSDiscoverer()
	}

	topic := strings.Join(keywords[:min(3, len(keywords))], " ")
	sources, err := d.inner.Discover(topic, keywords, limit)
	if err != nil {
		return nil, err
	}

	// Convert to our Source type
	result := make([]Source, len(sources))
	for i, s := range sources {
		result[i] = Source{
			URL:         s.URL,
			Title:       s.Title,
			Description: s.Description,
			Type:        s.Type,
			Language:    s.Language,
			TrustScore:  s.Score,
			Validated:   s.Validated,
			FeedType:    "rss",
		}
	}

	return result, nil
}

// GitHubDiscovererAdapter adapts the existing discovery.GitHubDiscoverer
type GitHubDiscovererAdapter struct {
	inner *discovery.GitHubDiscoverer
}

func (d *GitHubDiscovererAdapter) Name() string { return "github" }

func (d *GitHubDiscovererAdapter) Discover(ctx context.Context, keywords []string, languages []string, authorities []string, limit int) ([]Source, error) {
	if d.inner == nil {
		d.inner = discovery.NewGitHubDiscoverer()
	}

	topic := strings.Join(keywords[:min(3, len(keywords))], " ")
	sources, err := d.inner.Discover(topic, keywords, limit)
	if err != nil {
		return nil, err
	}

	// Convert to our Source type
	result := make([]Source, len(sources))
	for i, s := range sources {
		result[i] = Source{
			URL:         s.URL,
			Title:       s.Title,
			Description: s.Description,
			Type:        "github",
			TrustScore:  s.Score,
			Validated:   s.Validated,
			FeedType:    "github",
		}
	}

	return result, nil
}
