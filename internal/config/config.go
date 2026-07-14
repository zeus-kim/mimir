package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the complete application configuration
type Config struct {
	// Data directory for verticals
	DataDir string `json:"data_dir"`

	// Server settings
	Server ServerConfig `json:"server"`

	// Database settings
	Database DatabaseConfig `json:"database"`

	// TTS settings
	TTS TTSConfig `json:"tts"`

	// Delivery channels
	Delivery DeliveryConfig `json:"delivery"`

	// API keys (can also be set via environment)
	APIKeys APIKeysConfig `json:"api_keys"`

	// Vertical defaults
	Verticals VerticalsConfig `json:"verticals"`

	// Logging
	Logging LoggingConfig `json:"logging"`
}

type ServerConfig struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Language string `json:"language"` // en, ko, ja, zh, es, fr, de
}

type DatabaseConfig struct {
	Path           string `json:"path"`
	WALMode        bool   `json:"wal_mode"`
	BusyTimeoutMS  int    `json:"busy_timeout_ms"`
	MaxConnections int    `json:"max_connections"`
}

type TTSConfig struct {
	Engine   string `json:"engine"` // edge-tts, say, none
	Voice    string `json:"voice"`
	Language string `json:"language"`
	Speed    string `json:"speed"`
}

type DeliveryConfig struct {
	Default        string          `json:"default"`
	Telegram       TelegramConfig  `json:"telegram"`
	Slack          SlackConfig     `json:"slack"`
	Discord        DiscordConfig   `json:"discord"`
	Ntfy           NtfyConfig      `json:"ntfy"`
	Email          EmailConfig     `json:"email"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
}

type DiscordConfig struct {
	WebhookURL string `json:"webhook_url"`
}

type NtfyConfig struct {
	Server string `json:"server"`
	Topic  string `json:"topic"`
}

type EmailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

type APIKeysConfig struct {
	// US APIs
	FRED          string `json:"fred"`
	Congress      string `json:"congress"`
	EIA           string `json:"eia"`
	USDA          string `json:"usda"`
	ProPublica    string `json:"propublica"`
	OpenSecrets   string `json:"opensecrets"`
	VoteSmart     string `json:"votesmart"`
	OpenStates    string `json:"openstates"`
	CourtListener string `json:"courtlistener"`

	// European APIs
	ENTSOE string `json:"entsoe"`

	// Korean APIs
	DART           string `json:"dart"`
	KoreanAssembly string `json:"korean_assembly"`

	// Other
	AlphaVantage string `json:"alpha_vantage"`
	Spoonacular  string `json:"spoonacular"`
}

type VerticalsConfig struct {
	BaseDir          string   `json:"base_dir"`
	DefaultLanguages []string `json:"default_languages"`
	MinFitPercent    float64  `json:"min_fit_percent"`
	MaxFeeds         int      `json:"max_feeds"`
	PruneThreshold   float64  `json:"prune_threshold"`
}

type LoggingConfig struct {
	Level  string `json:"level"` // debug, info, warn, error
	Format string `json:"format"` // json, text
	File   string `json:"file"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DataDir: filepath.Join(home, ".mimir-verticals"),
		Server: ServerConfig{
			Name:     "mimir-mcp",
			Version:  "1.0.0",
			Language: "en",
		},
		Database: DatabaseConfig{
			Path:           filepath.Join(home, ".mine", "lite.db"),
			WALMode:        true,
			BusyTimeoutMS:  5000,
			MaxConnections: 1,
		},
		TTS: TTSConfig{
			Engine:   "edge-tts",
			Voice:    "en-US-AriaNeural",
			Language: "en",
			Speed:    "+0%",
		},
		Delivery: DeliveryConfig{
			Default: "none",
			Ntfy: NtfyConfig{
				Server: "https://ntfy.sh",
			},
		},
		Verticals: VerticalsConfig{
			BaseDir:          filepath.Join(home, ".mine-verticals"),
			DefaultLanguages: []string{"en", "ko"},
			MinFitPercent:    50.0,
			MaxFeeds:         200,
			PruneThreshold:   0.3,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load loads configuration from file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() {
	// Data directory
	if v := os.Getenv("MIMIR_DATA_DIR"); v != "" {
		c.DataDir = v
	}

	// Language
	if v := os.Getenv("MIMIR_LANGUAGE"); v != "" {
		c.Server.Language = v
	}

	// Database
	if v := os.Getenv("MIMIR_DB_PATH"); v != "" {
		c.Database.Path = v
	}

	// TTS
	if v := os.Getenv("MIMIR_TTS_ENGINE"); v != "" {
		c.TTS.Engine = v
	}

	// API Keys
	if v := os.Getenv("FRED_API_KEY"); v != "" {
		c.APIKeys.FRED = v
	}
	if v := os.Getenv("CONGRESS_API_KEY"); v != "" {
		c.APIKeys.Congress = v
	}
	if v := os.Getenv("EIA_API_KEY"); v != "" {
		c.APIKeys.EIA = v
	}
	if v := os.Getenv("USDA_API_KEY"); v != "" {
		c.APIKeys.USDA = v
	}
	if v := os.Getenv("PROPUBLICA_API_KEY"); v != "" {
		c.APIKeys.ProPublica = v
	}
	if v := os.Getenv("OPENSECRETS_API_KEY"); v != "" {
		c.APIKeys.OpenSecrets = v
	}
	if v := os.Getenv("VOTESMART_API_KEY"); v != "" {
		c.APIKeys.VoteSmart = v
	}
	if v := os.Getenv("OPENSTATES_API_KEY"); v != "" {
		c.APIKeys.OpenStates = v
	}
	if v := os.Getenv("COURTLISTENER_API_TOKEN"); v != "" {
		c.APIKeys.CourtListener = v
	}
	if v := os.Getenv("ENTSOE_API_TOKEN"); v != "" {
		c.APIKeys.ENTSOE = v
	}
	if v := os.Getenv("DART_API_KEY"); v != "" {
		c.APIKeys.DART = v
	}
	if v := os.Getenv("KOREAN_ASSEMBLY_API_KEY"); v != "" {
		c.APIKeys.KoreanAssembly = v
	}
	if v := os.Getenv("ALPHAVANTAGE_API_KEY"); v != "" {
		c.APIKeys.AlphaVantage = v
	}
	if v := os.Getenv("SPOONACULAR_API_KEY"); v != "" {
		c.APIKeys.Spoonacular = v
	}

	// Delivery
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		c.Delivery.Telegram.BotToken = v
	}
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		c.Delivery.Telegram.ChatID = v
	}
	if v := os.Getenv("SLACK_WEBHOOK_URL"); v != "" {
		c.Delivery.Slack.WebhookURL = v
	}
	if v := os.Getenv("DISCORD_WEBHOOK_URL"); v != "" {
		c.Delivery.Discord.WebhookURL = v
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	var errs []string

	// Validate TTS engine
	validEngines := map[string]bool{"edge-tts": true, "say": true, "none": true}
	if !validEngines[c.TTS.Engine] {
		errs = append(errs, fmt.Sprintf("invalid TTS engine: %s", c.TTS.Engine))
	}

	// Validate logging level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		errs = append(errs, fmt.Sprintf("invalid log level: %s", c.Logging.Level))
	}

	// Validate verticals config
	if c.Verticals.MinFitPercent < 0 || c.Verticals.MinFitPercent > 100 {
		errs = append(errs, "min_fit_percent must be between 0 and 100")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// AvailableAPIs returns which APIs are available based on configured keys
func (c *Config) AvailableAPIs() map[string]bool {
	return map[string]bool{
		// Always available (no key required)
		"arxiv":            true,
		"semantic_scholar": true,
		"huggingface":      true,
		"papers_with_code": true,
		"federal_register": true,
		"court_listener":   true,
		"yahoo_finance":    true,
		"open_food_facts":  true,
		"the_meal_db":      true,
		"ercot":            true,
		"pubmed":           true,
		"clinical_trials":  true,
		"fda":              true,
		"sec":              true,

		// Key required
		"fred":            c.APIKeys.FRED != "",
		"congress":        c.APIKeys.Congress != "",
		"eia":             c.APIKeys.EIA != "",
		"usda":            c.APIKeys.USDA != "",
		"propublica":      c.APIKeys.ProPublica != "",
		"opensecrets":     c.APIKeys.OpenSecrets != "",
		"votesmart":       c.APIKeys.VoteSmart != "",
		"openstates":      c.APIKeys.OpenStates != "",
		"entsoe":          c.APIKeys.ENTSOE != "",
		"dart":            c.APIKeys.DART != "",
		"korean_assembly": c.APIKeys.KoreanAssembly != "",
		"alpha_vantage":   c.APIKeys.AlphaVantage != "",
		"spoonacular":     c.APIKeys.Spoonacular != "",
	}
}

// GetVerticalDBPath returns the database path for a vertical
func (c *Config) GetVerticalDBPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fmt.Sprintf(".mine-%s", name), "lite.db")
}

// ConfigPath returns the default config file path
func (c *Config) ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mimir", "config.json")
}

// Default returns a pointer to the default config (for CLI use)
func Default() *Config {
	return DefaultConfig()
}
