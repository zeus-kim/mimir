package vertical

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/mimir-mcp/internal/i18n"
)

// skipIfNoFTS5 skips tests that require FTS5 if it's not available
func skipIfNoFTS5(t *testing.T, err error) bool {
	if err != nil && strings.Contains(err.Error(), "fts5") {
		t.Skip("FTS5 not available, skipping test")
		return true
	}
	return false
}

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.baseDir != tmpDir {
		t.Errorf("expected baseDir '%s', got '%s'", tmpDir, m.baseDir)
	}
}

func TestNewManagerDefaultDir(t *testing.T) {
	m := NewManager("")

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".mimir-verticals")

	if m.baseDir != expected {
		t.Errorf("expected default baseDir '%s', got '%s'", expected, m.baseDir)
	}
}

func TestManagerCreate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	v, err := m.Create(
		"test-vertical",
		"pharma",
		"Test vertical for pharma",
		[]string{"drug", "clinical trial"},
		[]string{"en"},
	)

	if skipIfNoFTS5(t, err) {
		return
	}

	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if v.Name != "test-vertical" {
		t.Errorf("expected name 'test-vertical', got '%s'", v.Name)
	}

	if v.Domain != "pharma" {
		t.Errorf("expected domain 'pharma', got '%s'", v.Domain)
	}

	if len(v.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(v.Keywords))
	}

	// Check that DB was created
	if _, err := os.Stat(v.DBPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Check that config was saved
	configPath := filepath.Join(tmpDir, "test-vertical", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestManagerCreateDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Create("test", "pharma", "", []string{"test"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = m.Create("test", "pharma", "", []string{"test"}, []string{"en"})
	if err == nil {
		t.Error("expected error when creating duplicate vertical")
	}
}

func TestManagerGet(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create first
	_, err := m.Create("test", "ai", "Test AI vertical", []string{"ml", "ai"}, []string{"en", "ko"})
	if skipIfNoFTS5(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Get
	v, err := m.Get("test")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if v.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", v.Name)
	}

	if v.Domain != "ai" {
		t.Errorf("expected domain 'ai', got '%s'", v.Domain)
	}

	if len(v.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(v.Languages))
	}
}

func TestManagerGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent vertical")
	}
}

func TestManagerList(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create multiple verticals
	_, err := m.Create("pharma-v1", "pharma", "", []string{"drug"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}
	m.Create("ai-v1", "ai", "", []string{"ml"}, []string{"en"})
	m.Create("legal-v1", "legal", "", []string{"law"}, []string{"en"})

	list, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 verticals, got %d", len(list))
	}

	// Check names
	names := make(map[string]bool)
	for _, v := range list {
		names[v.Name] = true
	}

	if !names["pharma-v1"] || !names["ai-v1"] || !names["legal-v1"] {
		t.Error("missing expected verticals in list")
	}
}

func TestManagerListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	list, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("expected 0 verticals, got %d", len(list))
	}
}

func TestManagerDelete(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Create("to-delete", "pharma", "", []string{"test"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	// Verify exists
	if !m.Exists("to-delete") {
		t.Fatal("vertical should exist before delete")
	}

	// Delete
	err = m.Delete("to-delete")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify gone
	if m.Exists("to-delete") {
		t.Error("vertical should not exist after delete")
	}
}

func TestManagerDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	err := m.Delete("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent vertical")
	}
}

func TestManagerExists(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	if m.Exists("test") {
		t.Error("vertical should not exist initially")
	}

	_, err := m.Create("test", "pharma", "", []string{"test"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	if !m.Exists("test") {
		t.Error("vertical should exist after creation")
	}
}

func TestManagerUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Create("test", "pharma", "Original", []string{"drug"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	v, _ := m.Get("test")
	v.Description = "Updated description"
	v.Keywords = []string{"drug", "clinical trial", "FDA"}

	err = m.Update(v)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	// Reload and verify
	v2, _ := m.Get("test")
	if v2.Description != "Updated description" {
		t.Errorf("expected updated description, got '%s'", v2.Description)
	}

	if len(v2.Keywords) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(v2.Keywords))
	}
}

func TestManagerUpdateSettings(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Create("test", "pharma", "", []string{"drug"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	newSettings := Settings{
		MinFitPercent:  60.0,
		MaxFeeds:       100,
		PruneThreshold: 0.5,
		FetchInterval:  "6h",
		EnabledAPIs:    []string{"pubmed", "fda"},
	}

	err = m.UpdateSettings("test", newSettings)
	if err != nil {
		t.Fatalf("UpdateSettings() error: %v", err)
	}

	v, _ := m.Get("test")
	if v.Settings.MinFitPercent != 60.0 {
		t.Errorf("expected MinFitPercent 60.0, got %f", v.Settings.MinFitPercent)
	}

	if v.Settings.MaxFeeds != 100 {
		t.Errorf("expected MaxFeeds 100, got %d", v.Settings.MaxFeeds)
	}
}

func TestManagerCreateFromPreset(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	v, err := m.CreateFromPreset("ai-research", "ai", []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}
	if err != nil {
		t.Fatalf("CreateFromPreset() error: %v", err)
	}

	if v.Name != "ai-research" {
		t.Errorf("expected name 'ai-research', got '%s'", v.Name)
	}

	if v.Domain != "ai" {
		t.Errorf("expected domain 'ai', got '%s'", v.Domain)
	}

	// Should have preset keywords
	if len(v.Keywords) == 0 {
		t.Error("expected preset keywords to be set")
	}

	// Should have enabled APIs from preset
	if len(v.Settings.EnabledAPIs) == 0 {
		t.Error("expected preset APIs to be set")
	}
}

func TestManagerCreateFromPresetInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.CreateFromPreset("test", "invalid-domain", []string{"en"})
	if err == nil {
		t.Error("expected error for invalid domain preset")
	}
}

func TestGetDomainPresets(t *testing.T) {
	presets := GetDomainPresets()

	expectedDomains := []string{"pharma", "ai", "legal", "finance", "politics", "energy", "food", "tech"}

	for _, domain := range expectedDomains {
		preset, ok := presets[domain]
		if !ok {
			t.Errorf("missing preset for domain '%s'", domain)
			continue
		}

		if preset.Domain != domain {
			t.Errorf("preset domain mismatch: expected '%s', got '%s'", domain, preset.Domain)
		}

		if len(preset.Keywords) == 0 {
			t.Errorf("preset '%s' has no keywords", domain)
		}

		if len(preset.APIs) == 0 {
			t.Errorf("preset '%s' has no APIs", domain)
		}
	}
}

func TestManagerOpenDB(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	_, err := m.Create("test", "pharma", "", []string{"drug"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	db, err := m.OpenDB("test")
	if err != nil {
		t.Fatalf("OpenDB() error: %v", err)
	}
	defer db.Close()

	// Should be able to query
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
}

func TestManagerI18nIntegration(t *testing.T) {
	// Set Korean
	i18n.SetLanguage(i18n.KO)

	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create should use Korean error messages
	_, err := m.Create("test", "pharma", "", []string{"drug"}, []string{"en"})
	if skipIfNoFTS5(t, err) {
		return
	}

	_, err = m.Create("test", "pharma", "", []string{"drug"}, []string{"en"})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}

	// Error message should be in Korean
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}

	// Reset
	i18n.SetLanguage(i18n.EN)
}
