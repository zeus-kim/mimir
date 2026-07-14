package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Name != "mimir-mcp" {
		t.Errorf("expected server name 'mimir-mcp', got '%s'", cfg.Server.Name)
	}

	if cfg.Server.Language != "en" {
		t.Errorf("expected default language 'en', got '%s'", cfg.Server.Language)
	}

	if cfg.TTS.Engine != "edge-tts" {
		t.Errorf("expected TTS engine 'edge-tts', got '%s'", cfg.TTS.Engine)
	}

	if cfg.Verticals.MinFitPercent != 50.0 {
		t.Errorf("expected min fit percent 50.0, got %f", cfg.Verticals.MinFitPercent)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid TTS engine",
			modify: func(c *Config) {
				c.TTS.Engine = "invalid-engine"
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Logging.Level = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid min fit percent",
			modify: func(c *Config) {
				c.Verticals.MinFitPercent = 150.0
			},
			wantErr: true,
		},
		{
			name: "negative min fit percent",
			modify: func(c *Config) {
				c.Verticals.MinFitPercent = -10.0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env
	origLang := os.Getenv("MIMIR_LANGUAGE")
	origDB := os.Getenv("MIMIR_DB_PATH")
	origFRED := os.Getenv("FRED_API_KEY")
	defer func() {
		os.Setenv("MIMIR_LANGUAGE", origLang)
		os.Setenv("MIMIR_DB_PATH", origDB)
		os.Setenv("FRED_API_KEY", origFRED)
	}()

	// Set test env
	os.Setenv("MIMIR_LANGUAGE", "ko")
	os.Setenv("MIMIR_DB_PATH", "/test/db.sqlite")
	os.Setenv("FRED_API_KEY", "test-key-123")

	cfg := DefaultConfig()
	cfg.loadFromEnv()

	if cfg.Server.Language != "ko" {
		t.Errorf("expected language 'ko', got '%s'", cfg.Server.Language)
	}

	if cfg.Database.Path != "/test/db.sqlite" {
		t.Errorf("expected db path '/test/db.sqlite', got '%s'", cfg.Database.Path)
	}

	if cfg.APIKeys.FRED != "test-key-123" {
		t.Errorf("expected FRED key 'test-key-123', got '%s'", cfg.APIKeys.FRED)
	}
}

func TestAvailableAPIs(t *testing.T) {
	cfg := DefaultConfig()

	// No keys set - check key-free APIs
	apis := cfg.AvailableAPIs()

	keyFreeAPIs := []string{
		"arxiv", "semantic_scholar", "huggingface", "papers_with_code",
		"federal_register", "court_listener", "yahoo_finance",
		"open_food_facts", "the_meal_db", "ercot",
		"pubmed", "clinical_trials", "fda", "sec",
	}

	for _, api := range keyFreeAPIs {
		if !apis[api] {
			t.Errorf("expected key-free API '%s' to be available", api)
		}
	}

	// Check key-required APIs are unavailable
	keyRequiredAPIs := []string{
		"fred", "congress", "eia", "usda", "propublica",
	}

	for _, api := range keyRequiredAPIs {
		if apis[api] {
			t.Errorf("expected key-required API '%s' to be unavailable without key", api)
		}
	}

	// Set a key and check
	cfg.APIKeys.FRED = "test-key"
	apis = cfg.AvailableAPIs()
	if !apis["fred"] {
		t.Error("expected FRED API to be available with key set")
	}
}

func TestGetVerticalDBPath(t *testing.T) {
	cfg := DefaultConfig()

	path := cfg.GetVerticalDBPath("pharma")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".mine-pharma", "lite.db")

	if path != expected {
		t.Errorf("expected '%s', got '%s'", expected, path)
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"server": {
			"name": "test-mimir",
			"language": "ja"
		},
		"logging": {
			"level": "debug"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Name != "test-mimir" {
		t.Errorf("expected server name 'test-mimir', got '%s'", cfg.Server.Name)
	}

	if cfg.Server.Language != "ja" {
		t.Errorf("expected language 'ja', got '%s'", cfg.Server.Language)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
}
