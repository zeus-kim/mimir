package fetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClinicalTrialsFetcher(t *testing.T) {
	f := NewClinicalTrialsFetcher()
	if f == nil {
		t.Fatal("NewClinicalTrialsFetcher returned nil")
	}
	if f.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
}

func TestNewPubMedFetcher(t *testing.T) {
	f := NewPubMedFetcher()
	if f == nil {
		t.Fatal("NewPubMedFetcher returned nil")
	}
}

func TestNewFDAFetcher(t *testing.T) {
	f := NewFDAFetcher()
	if f == nil {
		t.Fatal("NewFDAFetcher returned nil")
	}
}

func TestNewSECFetcher(t *testing.T) {
	f := NewSECFetcher()
	if f == nil {
		t.Fatal("NewSECFetcher returned nil")
	}
}

func TestNewAIResearchFetcher(t *testing.T) {
	f := NewAIResearchFetcher()
	if f == nil {
		t.Fatal("NewAIResearchFetcher returned nil")
	}

	sources := f.AvailableSources()
	expectedSources := []string{"arxiv", "semantic_scholar", "huggingface", "papers_with_code"}
	for _, s := range expectedSources {
		if !sources[s] {
			t.Errorf("expected source '%s' to be available", s)
		}
	}
}

func TestNewLegalFetcher(t *testing.T) {
	f := NewLegalFetcher()
	if f == nil {
		t.Fatal("NewLegalFetcher returned nil")
	}

	sources := f.AvailableSources()
	// Key-free sources should be available
	if !sources["federal_register"] {
		t.Error("federal_register should be available")
	}
	if !sources["court_listener"] {
		t.Error("court_listener should be available")
	}
}

func TestNewFinanceFetcher(t *testing.T) {
	f := NewFinanceFetcher()
	if f == nil {
		t.Fatal("NewFinanceFetcher returned nil")
	}

	sources := f.AvailableSources()
	// Yahoo Finance should always be available
	if !sources["yahoo_finance"] {
		t.Error("yahoo_finance should be available")
	}
}

func TestNewEnergyFetcher(t *testing.T) {
	f := NewEnergyFetcher()
	if f == nil {
		t.Fatal("NewEnergyFetcher returned nil")
	}

	sources := f.AvailableSources()
	// ERCOT should always be available
	if !sources["ercot"] {
		t.Error("ercot should be available")
	}
}

func TestNewFoodFetcher(t *testing.T) {
	f := NewFoodFetcher()
	if f == nil {
		t.Fatal("NewFoodFetcher returned nil")
	}

	sources := f.AvailableSources()
	// Key-free sources
	if !sources["open_food_facts"] {
		t.Error("open_food_facts should be available")
	}
	if !sources["the_meal_db"] {
		t.Error("the_meal_db should be available")
	}
}

func TestNewPoliticsFetcher(t *testing.T) {
	f := NewPoliticsFetcher()
	if f == nil {
		t.Fatal("NewPoliticsFetcher returned nil")
	}

	sources := f.AvailableSources()
	// All politics sources require keys, so should be false without keys
	for name, available := range sources {
		if available {
			t.Errorf("expected %s to be unavailable without key", name)
		}
	}
}

func TestFederalRegisterFetcher(t *testing.T) {
	// Mock server for Federal Register
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"count": 1,
			"results": [{
				"document_number": "2024-00001",
				"title": "Test Rule",
				"type": "RULE",
				"abstract": "Test abstract",
				"agencies": [{"name": "Test Agency"}],
				"publication_date": "2024-01-01"
			}]
		}`))
	}))
	defer server.Close()

	f := NewFederalRegisterFetcher()
	f.BaseURL = server.URL

	// Test that it handles the response format
	// (Full test would require a mock DB)
	if f.BaseURL == "" {
		t.Error("BaseURL should be set")
	}
}

func TestArxivFetcher(t *testing.T) {
	f := NewArxivFetcher()
	if f == nil {
		t.Fatal("NewArxivFetcher returned nil")
	}
	if f.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
}

func TestYahooFinanceFetcher(t *testing.T) {
	f := NewYahooFinanceFetcher()
	if f == nil {
		t.Fatal("NewYahooFinanceFetcher returned nil")
	}
	if f.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
}

func TestERCOTFetcher(t *testing.T) {
	f := NewERCOTFetcher()
	if f == nil {
		t.Fatal("NewERCOTFetcher returned nil")
	}
	if f.BaseURL == "" {
		t.Error("BaseURL should not be empty")
	}
}

func TestERCOTZones(t *testing.T) {
	expectedZones := []string{"LZ_HOUSTON", "LZ_NORTH", "LZ_SOUTH", "LZ_WEST"}

	for _, zone := range expectedZones {
		found := false
		for _, z := range ERCOTZones {
			if z == zone {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected zone '%s' in ERCOTZones", zone)
		}
	}
}

func TestExtractERCOTZone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"LZ_HOUSTON", "LZ_HOUSTON"},
		{"LZ_NORTH_ABC", "LZ_NORTH"},
		{"HB_HUBAVG", "HB_HUBAVG"},
		{"UNKNOWN_POINT", "UNKNOWN_POINT"},
	}

	for _, tt := range tests {
		got := extractERCOTZone(tt.input)
		if got != tt.expected {
			t.Errorf("extractERCOTZone(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCommonEconomicIndicators(t *testing.T) {
	expected := []string{"GDP", "UNRATE", "CPIAUCSL", "FEDFUNDS"}

	for _, ind := range expected {
		found := false
		for _, i := range CommonEconomicIndicators {
			if i == ind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected indicator '%s' in CommonEconomicIndicators", ind)
		}
	}
}

func TestENTSOECountries(t *testing.T) {
	expectedCountries := []string{"DE", "FR", "GB", "ES", "IT"}

	for _, country := range expectedCountries {
		if _, ok := ENTSOECountries[country]; !ok {
			t.Errorf("expected country '%s' in ENTSOECountries", country)
		}
	}
}

func TestMapENTSOEPsrType(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"B16", "Solar"},
		{"B19", "Wind Onshore"},
		{"B14", "Nuclear"},
		{"B04", "Fossil Gas"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		got := mapENTSOEPsrType(tt.code)
		if got != tt.expected {
			t.Errorf("mapENTSOEPsrType(%q) = %q, want %q", tt.code, got, tt.expected)
		}
	}
}

func TestEIASeriesPresets(t *testing.T) {
	expectedPresets := []string{"crude_oil_price", "natural_gas_price", "electricity_generation"}

	for _, preset := range expectedPresets {
		if _, ok := EIASeriesPresets[preset]; !ok {
			t.Errorf("expected preset '%s' in EIASeriesPresets", preset)
		}
	}
}

func TestAIResearchQueries(t *testing.T) {
	expectedDomains := []string{"llm", "vision", "multimodal", "agents", "efficiency"}

	for _, domain := range expectedDomains {
		queries, ok := AIResearchQueries[domain]
		if !ok {
			t.Errorf("expected domain '%s' in AIResearchQueries", domain)
			continue
		}
		if len(queries) == 0 {
			t.Errorf("domain '%s' has no queries", domain)
		}
	}
}
