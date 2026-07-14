package vertical

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zeus-kim/mimir/internal/db"
	"github.com/zeus-kim/mimir/internal/i18n"
	"github.com/zeus-kim/mimir/internal/logger"
)

// Vertical represents a vertical search engine instance
type Vertical struct {
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords"`
	Languages   []string  `json:"languages"`
	DBPath      string    `json:"db_path"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Settings    Settings  `json:"settings"`
}

// Settings holds vertical-specific settings
type Settings struct {
	MinFitPercent   float64  `json:"min_fit_percent"`
	MaxFeeds        int      `json:"max_feeds"`
	PruneThreshold  float64  `json:"prune_threshold"`
	FetchInterval   string   `json:"fetch_interval"`
	EnabledAPIs     []string `json:"enabled_apis"`
	ExcludedDomains []string `json:"excluded_domains"`
}

// Stats holds vertical statistics
type Stats struct {
	Documents     int       `json:"documents"`
	Feeds         int       `json:"feeds"`
	FitPercent    float64   `json:"fit_percent"`
	LastFetch     time.Time `json:"last_fetch"`
	LastPrune     time.Time `json:"last_prune"`
	TotalFetches  int       `json:"total_fetches"`
	TotalPruned   int       `json:"total_pruned"`
}

// Manager manages vertical instances
type Manager struct {
	baseDir string
	log     *logger.Logger
}

// NewManager creates a new vertical manager
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".mimir-verticals")
	}
	return &Manager{
		baseDir: baseDir,
		log:     logger.Default().WithField("component", "vertical-manager"),
	}
}

// Create creates a new vertical
func (m *Manager) Create(name, domain, description string, keywords, languages []string) (*Vertical, error) {
	// Check if exists
	if m.Exists(name) {
		return nil, fmt.Errorf("%s", i18n.T("vertical_exists", name))
	}

	// Create directory
	vertDir := m.getVerticalDir(name)
	if err := os.MkdirAll(vertDir, 0755); err != nil {
		return nil, fmt.Errorf("creating vertical directory: %w", err)
	}

	// Create vertical config
	v := &Vertical{
		Name:        name,
		Domain:      domain,
		Description: description,
		Keywords:    keywords,
		Languages:   languages,
		DBPath:      filepath.Join(vertDir, "lite.db"),
		Created:     time.Now(),
		Updated:     time.Now(),
		Settings: Settings{
			MinFitPercent:  50.0,
			MaxFeeds:       200,
			PruneThreshold: 0.3,
			FetchInterval:  "3h",
			EnabledAPIs:    []string{},
		},
	}

	// Save config
	if err := m.saveConfig(v); err != nil {
		os.RemoveAll(vertDir)
		return nil, err
	}

	// Initialize database
	database, err := db.Open(v.DBPath)
	if err != nil {
		os.RemoveAll(vertDir)
		return nil, fmt.Errorf("creating database: %w", err)
	}
	defer database.Close()

	if err := database.EnsureSchema(); err != nil {
		os.RemoveAll(vertDir)
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	m.log.Info("%s", i18n.T("vertical_created", name))
	return v, nil
}

// Get retrieves a vertical by name
func (m *Manager) Get(name string) (*Vertical, error) {
	configPath := filepath.Join(m.getVerticalDir(name), "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", i18n.T("vertical_not_found", name))
		}
		return nil, err
	}

	var v Vertical
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	return &v, nil
}

// List returns all verticals
func (m *Manager) List() ([]*Vertical, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Vertical{}, nil
		}
		return nil, err
	}

	var verticals []*Vertical
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		v, err := m.Get(entry.Name())
		if err != nil {
			continue
		}
		verticals = append(verticals, v)
	}

	return verticals, nil
}

// Delete deletes a vertical
func (m *Manager) Delete(name string) error {
	if !m.Exists(name) {
		return fmt.Errorf("%s", i18n.T("vertical_not_found", name))
	}

	vertDir := m.getVerticalDir(name)
	if err := os.RemoveAll(vertDir); err != nil {
		return err
	}

	m.log.Info("%s", i18n.T("vertical_deleted", name))
	return nil
}

// Exists checks if a vertical exists
func (m *Manager) Exists(name string) bool {
	configPath := filepath.Join(m.getVerticalDir(name), "config.json")
	_, err := os.Stat(configPath)
	return err == nil
}

// Update updates a vertical's configuration
func (m *Manager) Update(v *Vertical) error {
	if !m.Exists(v.Name) {
		return fmt.Errorf("%s", i18n.T("vertical_not_found", v.Name))
	}

	v.Updated = time.Now()
	return m.saveConfig(v)
}

// UpdateSettings updates a vertical's settings
func (m *Manager) UpdateSettings(name string, settings Settings) error {
	v, err := m.Get(name)
	if err != nil {
		return err
	}

	v.Settings = settings
	v.Updated = time.Now()
	return m.saveConfig(v)
}

// GetStats returns statistics for a vertical
func (m *Manager) GetStats(name string) (*Stats, error) {
	v, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	database, err := db.Open(v.DBPath)
	if err != nil {
		return nil, err
	}
	defer database.Close()

	stats := &Stats{}

	// Count documents
	var docCount int
	row := database.QueryRow("SELECT COUNT(*) FROM documents")
	if err := row.Scan(&docCount); err == nil {
		stats.Documents = docCount
	}

	// Count feeds
	var feedCount int
	row = database.QueryRow("SELECT COUNT(*) FROM feeds WHERE active = 1")
	if err := row.Scan(&feedCount); err == nil {
		stats.Feeds = feedCount
	}

	// Calculate fit percent
	if docCount > 0 && len(v.Keywords) > 0 {
		var matchCount int
		query := "SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH ?"
		keywords := "(" + joinKeywords(v.Keywords) + ")"
		row = database.QueryRow(query, keywords)
		if err := row.Scan(&matchCount); err == nil {
			stats.FitPercent = float64(matchCount) / float64(docCount) * 100
		}
	}

	// Get metadata
	statsPath := filepath.Join(m.getVerticalDir(name), "stats.json")
	if data, err := os.ReadFile(statsPath); err == nil {
		json.Unmarshal(data, stats)
	}

	return stats, nil
}

// SaveStats saves statistics for a vertical
func (m *Manager) SaveStats(name string, stats *Stats) error {
	statsPath := filepath.Join(m.getVerticalDir(name), "stats.json")
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statsPath, data, 0644)
}

// OpenDB opens the database for a vertical
func (m *Manager) OpenDB(name string) (*db.DB, error) {
	v, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	return db.Open(v.DBPath)
}

// Helper methods

func (m *Manager) getVerticalDir(name string) string {
	return filepath.Join(m.baseDir, name)
}

func (m *Manager) saveConfig(v *Vertical) error {
	configPath := filepath.Join(m.getVerticalDir(v.Name), "config.json")
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func joinKeywords(keywords []string) string {
	result := ""
	for i, kw := range keywords {
		if i > 0 {
			result += " OR "
		}
		result += "\"" + kw + "\""
	}
	return result
}

// Domain presets

// DomainPreset contains default settings for a domain
type DomainPreset struct {
	Domain      string   `json:"domain"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	APIs        []string `json:"apis"`
}

// GetDomainPresets returns presets for all supported domains
func GetDomainPresets() map[string]DomainPreset {
	return map[string]DomainPreset{
		"pharma": {
			Domain:      "pharma",
			Name:        i18n.Get().DomainPharma,
			Description: "Clinical trials, drug approvals, biotech research",
			Keywords:    []string{"clinical trial", "FDA", "drug", "pharmaceutical", "biotech", "therapy", "approval"},
			APIs:        []string{"clinical_trials", "pubmed", "fda", "sec"},
		},
		"ai": {
			Domain:      "ai",
			Name:        i18n.Get().DomainAI,
			Description: "Machine learning, deep learning, LLMs, AI research",
			Keywords:    []string{"machine learning", "deep learning", "neural network", "LLM", "transformer", "AI"},
			APIs:        []string{"arxiv", "semantic_scholar", "huggingface", "papers_with_code"},
		},
		"legal": {
			Domain:      "legal",
			Name:        i18n.Get().DomainLegal,
			Description: "Court cases, legislation, regulations, legal news",
			Keywords:    []string{"court", "law", "regulation", "legislation", "ruling", "legal", "compliance"},
			APIs:        []string{"federal_register", "court_listener", "congress"},
		},
		"finance": {
			Domain:      "finance",
			Name:        i18n.Get().DomainFinance,
			Description: "Markets, economics, company filings, financial analysis",
			Keywords:    []string{"market", "stock", "finance", "economy", "investment", "SEC", "earnings"},
			APIs:        []string{"yahoo_finance", "sec", "fred"},
		},
		"politics": {
			Domain:      "politics",
			Name:        i18n.Get().DomainPolitics,
			Description: "Political news, policy, elections, government",
			Keywords:    []string{"politics", "election", "congress", "policy", "government", "legislation"},
			APIs:        []string{"propublica", "opensecrets"},
		},
		"energy": {
			Domain:      "energy",
			Name:        i18n.Get().DomainEnergy,
			Description: "Energy markets, renewable energy, utilities, grid data",
			Keywords:    []string{"energy", "power", "renewable", "solar", "wind", "electricity", "grid"},
			APIs:        []string{"ercot", "eia", "entsoe"},
		},
		"food": {
			Domain:      "food",
			Name:        i18n.Get().DomainFood,
			Description: "Nutrition, recipes, food industry, restaurants",
			Keywords:    []string{"food", "nutrition", "recipe", "restaurant", "cooking", "diet"},
			APIs:        []string{"open_food_facts", "the_meal_db", "usda"},
		},
		"tech": {
			Domain:      "tech",
			Name:        i18n.Get().DomainTech,
			Description: "Technology news, startups, open source, development",
			Keywords:    []string{"technology", "startup", "software", "open source", "programming", "cloud"},
			APIs:        []string{"arxiv", "huggingface"},
		},
	}
}

// CreateFromPreset creates a vertical from a domain preset
func (m *Manager) CreateFromPreset(name, domain string, languages []string) (*Vertical, error) {
	presets := GetDomainPresets()
	preset, ok := presets[domain]
	if !ok {
		return nil, fmt.Errorf("unknown domain: %s", domain)
	}

	if languages == nil {
		languages = []string{"en"}
	}

	v, err := m.Create(name, domain, preset.Description, preset.Keywords, languages)
	if err != nil {
		return nil, err
	}

	// Set enabled APIs from preset
	v.Settings.EnabledAPIs = preset.APIs
	if err := m.saveConfig(v); err != nil {
		return nil, err
	}

	return v, nil
}
