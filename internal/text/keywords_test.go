package text

import (
	"reflect"
	"testing"
)

func TestIsStopword(t *testing.T) {
	tests := []struct {
		word     string
		lang     string
		expected bool
	}{
		// English stopwords
		{"the", "en", true},
		{"and", "en", true},
		{"technology", "en", false},

		// Korean stopwords
		{"있는", "ko", true},
		{"인공지능", "ko", false},

		// Japanese stopwords
		{"する", "ja", true},
		{"AI", "ja", false},

		// Chinese stopwords
		{"的", "zh", true},
		{"人工智能", "zh", false},

		// Universal stopwords
		{"http", "en", true},
		{"www", "ko", true},
		{"com", "ja", true},

		// Too short
		{"a", "en", true},
		{"I", "en", true},

		// Digits only
		{"123", "en", true},
		{"2024", "en", true},

		// English stopwords checked for all languages
		{"the", "ko", true},
		{"and", "ja", true},
	}

	for _, tt := range tests {
		t.Run(tt.word+"_"+tt.lang, func(t *testing.T) {
			result := IsStopword(tt.word, tt.lang)
			if result != tt.expected {
				t.Errorf("IsStopword(%q, %q) = %v, want %v", tt.word, tt.lang, result, tt.expected)
			}
		})
	}
}

func TestNormalizeKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Hello World  ", "hello world"},
		{"AI", "ai"},
		{"NVIDIA", "nvidia"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeKeyword(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeKeyword(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCleanKeywords(t *testing.T) {
	keywords := []string{"AI", "the", "machine learning", "and", "AI", "technology", "123"}
	expected := []string{"AI", "machine learning", "technology"}

	result := CleanKeywords(keywords, "en")

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("CleanKeywords() = %v, want %v", result, expected)
	}
}

func TestComputeSpecificity(t *testing.T) {
	categories := []CategoryKeywords{
		{Category: "tech", Keywords: []string{"AI", "machine learning", "software"}},
		{Category: "finance", Keywords: []string{"AI", "stock", "market"}},
		{Category: "health", Keywords: []string{"AI", "medical", "treatment"}},
	}

	specificity := ComputeSpecificity(categories)

	// AI appears in all 3 categories, should have low specificity
	if specificity["ai"] >= 0.5 {
		t.Errorf("AI specificity = %v, expected < 0.5", specificity["ai"])
	}

	// Unique keywords should have specificity = 1.0
	if specificity["software"] != 1.0 {
		t.Errorf("software specificity = %v, expected 1.0", specificity["software"])
	}
	if specificity["stock"] != 1.0 {
		t.Errorf("stock specificity = %v, expected 1.0", specificity["stock"])
	}
}

func TestFilterBySpecificity(t *testing.T) {
	keywords := []string{"AI", "machine learning", "software"}
	specificity := Specificity{
		"ai":               0.2,
		"machine learning": 0.8,
		"software":         1.0,
	}

	result := FilterBySpecificity(keywords, specificity, 0.5)
	expected := []string{"machine learning", "software"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("FilterBySpecificity() = %v, want %v", result, expected)
	}
}

func TestMergeKeywords(t *testing.T) {
	existing := []string{"technology", "software", "AI"}
	seeds := []string{"AI", "machine learning", "NVIDIA"}

	result := MergeKeywords(existing, seeds)
	expected := []string{"AI", "machine learning", "NVIDIA", "technology", "software"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("MergeKeywords() = %v, want %v", result, expected)
	}
}

func TestDeduplicate(t *testing.T) {
	keywords := []string{"AI", "ai", "Machine Learning", "machine learning", "NVIDIA"}
	expected := []string{"AI", "Machine Learning", "NVIDIA"}

	result := Deduplicate(keywords)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Deduplicate() = %v, want %v", result, expected)
	}
}

func TestAddStopwords(t *testing.T) {
	// Add custom stopwords
	AddStopwords("en", []string{"customword"})

	if !IsStopword("customword", "en") {
		t.Error("Custom stopword not recognized")
	}
	if !IsStopword("CustomWord", "en") {
		t.Error("Custom stopword case-insensitive check failed")
	}
}
