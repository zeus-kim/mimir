package sources

import (
	"testing"
)

func TestNewAuthorityRegistry(t *testing.T) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	if reg.Count() == 0 {
		t.Error("Registry is empty, expected sources to be loaded")
	}

	t.Logf("Loaded %d sources across %d topics", reg.Count(), len(reg.Topics()))
}

func TestGetByTopic(t *testing.T) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	aiSources := reg.GetByTopic("ai")
	if len(aiSources) == 0 {
		t.Error("Expected AI sources, got none")
	}

	for _, src := range aiSources {
		if src.Topic != "ai" {
			t.Errorf("Expected topic 'ai', got '%s'", src.Topic)
		}
	}
}

func TestMatch(t *testing.T) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	tests := []struct {
		topic    string
		keywords []string
		wantMin  int
	}{
		{"health clinical trial", []string{"FDA", "NIH", "clinical"}, 1},
		{"AI research", []string{"machine learning", "LLM"}, 1},
		{"cryptocurrency", []string{"bitcoin", "blockchain"}, 1},
		{"unknown topic", []string{"nonexistent"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			matches := reg.Match(tt.topic, tt.keywords)
			if len(matches) < tt.wantMin {
				t.Errorf("Match(%q, %v) = %d sources, want at least %d",
					tt.topic, tt.keywords, len(matches), tt.wantMin)
			}

			// Check no duplicates
			seen := make(map[string]bool)
			for _, m := range matches {
				if seen[m.URL] {
					t.Errorf("Duplicate URL: %s", m.URL)
				}
				seen[m.URL] = true

				// Verify defaults are set
				if m.Source != "authority" {
					t.Errorf("Expected source 'authority', got '%s'", m.Source)
				}
				if m.TrustScore <= 0 {
					t.Errorf("Expected positive trust score, got %f", m.TrustScore)
				}
			}
		})
	}
}

func TestGetAuthorityFeeds(t *testing.T) {
	// Test pharma/clinical keywords - should match "health" topic in JSONL
	feeds, err := GetAuthorityFeeds("제약 임상시험", []string{"임상시험", "신약", "FDA"})
	if err != nil {
		t.Fatalf("GetAuthorityFeeds failed: %v", err)
	}

	if len(feeds) == 0 {
		t.Error("Expected health feeds for pharma keywords, got none")
	}

	// Check that health sources are matched (pharma keywords should match health)
	foundHealth := false
	for _, f := range feeds {
		if f.Topic == "health" {
			foundHealth = true
			break
		}
	}
	if !foundHealth && len(feeds) > 0 {
		t.Errorf("Expected health topic sources, got topics: %v", func() []string {
			topics := make(map[string]bool)
			for _, f := range feeds {
				topics[f.Topic] = true
			}
			result := make([]string, 0, len(topics))
			for k := range topics {
				result = append(result, k)
			}
			return result
		}())
	}
	t.Logf("Found %d feeds, health topic present: %v", len(feeds), foundHealth)
}

func TestGetByTier(t *testing.T) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	tier1 := reg.GetByTier(1)
	tier2 := reg.GetByTier(2)

	if len(tier1) > len(tier2) {
		t.Errorf("Tier 1 count (%d) should be <= tier 2 count (%d)",
			len(tier1), len(tier2))
	}

	for _, src := range tier1 {
		if src.Tier > 1 {
			t.Errorf("Expected tier <= 1, got tier %d for %s", src.Tier, src.URL)
		}
	}
}

func TestCountByTopic(t *testing.T) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	counts := reg.CountByTopic()
	if len(counts) == 0 {
		t.Error("Expected topic counts, got none")
	}

	total := 0
	for topic, count := range counts {
		t.Logf("%s: %d sources", topic, count)
		total += count
	}

	if total != reg.Count() {
		t.Errorf("Sum of topic counts (%d) != total count (%d)", total, reg.Count())
	}
}
