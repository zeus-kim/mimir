package i18n

import (
	"testing"
)

func TestSetAndGetLanguage(t *testing.T) {
	// Reset to default
	SetLanguage(EN)

	tests := []struct {
		lang     Language
		expected Language
	}{
		{EN, EN},
		{KO, KO},
		{JA, JA},
		{ZH, ZH},
	}

	for _, tt := range tests {
		SetLanguage(tt.lang)
		got := GetLanguage()
		if got != tt.expected {
			t.Errorf("SetLanguage(%v): got %v, want %v", tt.lang, got, tt.expected)
		}
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected Language
	}{
		{"en", EN},
		{"EN", EN},
		{"ko", KO},
		{"korean", KO},
		{"한국어", KO},
		{"ja", JA},
		{"japanese", JA},
		{"日本語", JA},
		{"zh", ZH},
		{"chinese", ZH},
		{"中文", ZH},
		{"invalid", EN}, // Default to EN
		{"", EN},
	}

	for _, tt := range tests {
		got := ParseLanguage(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLanguage(%q): got %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestGetMessages(t *testing.T) {
	// Test English
	SetLanguage(EN)
	m := Get()
	if m.AppName != "Mimir" {
		t.Errorf("EN AppName: got %q, want 'Mimir'", m.AppName)
	}
	if m.DomainPharma != "Pharmaceutical & Biotech" {
		t.Errorf("EN DomainPharma: got %q, want 'Pharmaceutical & Biotech'", m.DomainPharma)
	}

	// Test Korean
	SetLanguage(KO)
	m = Get()
	if m.AppName != "미미르" {
		t.Errorf("KO AppName: got %q, want '미미르'", m.AppName)
	}
	if m.DomainPharma != "제약/바이오" {
		t.Errorf("KO DomainPharma: got %q, want '제약/바이오'", m.DomainPharma)
	}

	// Test Japanese
	SetLanguage(JA)
	m = Get()
	if m.AppName != "ミーミル" {
		t.Errorf("JA AppName: got %q, want 'ミーミル'", m.AppName)
	}

	// Test Chinese
	SetLanguage(ZH)
	m = Get()
	if m.AppName != "弥米尔" {
		t.Errorf("ZH AppName: got %q, want '弥米尔'", m.AppName)
	}
}

func TestGetFor(t *testing.T) {
	// Should work regardless of current language setting
	m := GetFor(KO)
	if m.AppName != "미미르" {
		t.Errorf("GetFor(KO).AppName: got %q, want '미미르'", m.AppName)
	}

	m = GetFor(JA)
	if m.AppName != "ミーミル" {
		t.Errorf("GetFor(JA).AppName: got %q, want 'ミーミル'", m.AppName)
	}

	// Unknown language should return English
	m = GetFor(Language("xx"))
	if m.AppName != "Mimir" {
		t.Errorf("GetFor(xx).AppName: got %q, want 'Mimir'", m.AppName)
	}
}

func TestT(t *testing.T) {
	SetLanguage(EN)

	// Test simple key
	got := T("app_name")
	if got != "Mimir" {
		t.Errorf("T('app_name'): got %q, want 'Mimir'", got)
	}

	// Test key with formatting
	got = T("vertical_created", "test-vertical")
	expected := "Vertical 'test-vertical' created successfully"
	if got != expected {
		t.Errorf("T('vertical_created', 'test-vertical'): got %q, want %q", got, expected)
	}

	// Test Korean
	SetLanguage(KO)
	got = T("vertical_created", "test-vertical")
	expected = "버티컬 'test-vertical' 생성 완료"
	if got != expected {
		t.Errorf("T('vertical_created', 'test-vertical') in KO: got %q, want %q", got, expected)
	}

	// Test unknown key
	got = T("unknown_key")
	if got != "unknown_key" {
		t.Errorf("T('unknown_key'): got %q, want 'unknown_key'", got)
	}
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()

	if len(langs) != 7 {
		t.Errorf("expected 7 supported languages, got %d", len(langs))
	}

	expected := map[Language]bool{EN: true, KO: true, JA: true, ZH: true, ES: true, FR: true, DE: true}
	for _, l := range langs {
		if !expected[l] {
			t.Errorf("unexpected language in supported list: %v", l)
		}
	}
}

func TestExportImport(t *testing.T) {
	// Export Korean messages
	data, err := Export(KO)
	if err != nil {
		t.Fatalf("Export(KO) failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Export(KO) returned empty data")
	}

	// Import as a new language (just test the mechanism)
	err = Import(Language("test"), data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify imported
	m := GetFor(Language("test"))
	if m.AppName != "미미르" {
		t.Errorf("Imported AppName: got %q, want '미미르'", m.AppName)
	}
}

func TestAllTranslationsHaveRequiredFields(t *testing.T) {
	langs := SupportedLanguages()

	for _, lang := range langs {
		m := GetFor(lang)

		if m.AppName == "" {
			t.Errorf("%v: AppName is empty", lang)
		}
		if m.AppDescription == "" {
			t.Errorf("%v: AppDescription is empty", lang)
		}
		if m.DomainPharma == "" {
			t.Errorf("%v: DomainPharma is empty", lang)
		}
		if m.DomainAI == "" {
			t.Errorf("%v: DomainAI is empty", lang)
		}
		if m.ErrNotFound == "" {
			t.Errorf("%v: ErrNotFound is empty", lang)
		}
	}
}
