// Package category provides category/keyword mapping with multi-language support.
package category

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Categories defines the 12 supported categories.
var Categories = []string{
	"politics", "finance", "tech", "sports", "culture",
	"world", "business", "crypto", "science", "health",
	"security", "climate",
}

// TopLanguages defines the top 10 languages for priority processing.
var TopLanguages = []string{
	"en", "ko", "ja", "zh", "de", "fr", "es", "pt", "ru", "ar",
}

// AllLanguages defines all 74 supported languages.
var AllLanguages = []string{
	"am", "ar", "az", "be", "bg", "bn", "bs", "ca", "cs", "da",
	"de", "dv", "dz", "el", "en", "es", "et", "fa", "fi", "fr",
	"he", "hi", "hr", "hu", "hy", "id", "is", "it", "ja", "ka",
	"kk", "km", "ko", "ky", "lt", "lv", "mk", "ml", "ms", "mt",
	"ne", "nl", "no", "pl", "ps", "pt", "ro", "ru", "rw", "si",
	"sk", "sl", "so", "sq", "sr", "sv", "sw", "te", "tg", "th",
	"tk", "tr", "uk", "ur", "uz", "vi", "zh",
}

// SeedKeywords contains English seed keywords for each category.
var SeedKeywords = map[string][]string{
	"politics": {
		"president", "election", "congress", "parliament", "government",
		"minister", "senator", "vote", "democracy", "political party",
	},
	"finance": {
		"stock", "nasdaq", "investment", "banking", "interest rate",
		"federal reserve", "bond", "forex", "trading", "IPO",
	},
	"tech": {
		"AI", "artificial intelligence", "software", "startup", "semiconductor",
		"apple", "google", "microsoft", "cloud", "5G",
	},
	"sports": {
		"football", "soccer", "basketball", "NBA", "MLB",
		"world cup", "olympics", "tennis", "golf", "championship",
	},
	"culture": {
		"movie", "film", "music", "concert", "netflix",
		"celebrity", "entertainment", "drama", "festival", "art",
	},
	"world": {
		"war", "ukraine", "russia", "conflict", "military",
		"diplomatic", "UN", "NATO", "refugee", "crisis",
	},
	"business": {
		"company", "corporate", "revenue", "merger", "acquisition",
		"CEO", "earnings", "profit", "export", "import",
	},
	"crypto": {
		"bitcoin", "ethereum", "cryptocurrency", "blockchain", "NFT",
		"defi", "token", "binance", "coinbase", "web3",
	},
	"science": {
		"NASA", "space", "research", "discovery", "scientist",
		"physics", "chemistry", "biology", "experiment", "nobel",
	},
	"health": {
		"hospital", "doctor", "medical", "vaccine", "disease",
		"cancer", "treatment", "surgery", "patient", "healthcare",
	},
	"security": {
		"hack", "cyber", "ransomware", "malware", "breach",
		"vulnerability", "phishing", "firewall", "encryption", "zero-day",
	},
	"climate": {
		"climate", "global warming", "carbon neutral", "renewable energy",
		"solar", "wind power", "electric vehicle", "ESG", "net zero",
		"greenhouse gas", "Paris agreement", "climate crisis",
	},
}

// LanguageNames maps language codes to full names.
var LanguageNames = map[string]string{
	"ko": "Korean", "ja": "Japanese", "zh": "Chinese",
	"de": "German", "fr": "French", "es": "Spanish",
	"pt": "Portuguese", "ru": "Russian", "ar": "Arabic",
	"it": "Italian", "nl": "Dutch", "pl": "Polish",
	"tr": "Turkish", "vi": "Vietnamese", "th": "Thai",
	"id": "Indonesian", "hi": "Hindi", "he": "Hebrew",
	"en": "English",
}

// KeywordFile represents the JSON structure for keyword files.
type KeywordFile struct {
	Version    string                  `json:"version"`
	Language   string                  `json:"language"`
	MatchType  string                  `json:"match_type"`
	Cleaned    bool                    `json:"cleaned"`
	Categories map[string]CategoryData `json:"categories"`
}

// CategoryData holds keywords for a single category.
type CategoryData struct {
	Keywords      []string `json:"keywords"`
	Count         int      `json:"count"`
	OriginalCount int      `json:"original_count"`
}

// ProcessResult contains the result of processing a language.
type ProcessResult struct {
	Lang       string         `json:"lang"`
	Categories map[string]int `json:"categories"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
}

// VerifyResult contains verification results for a language.
type VerifyResult struct {
	Lang       string                      `json:"lang"`
	Categories map[string]VerifyCategoryResult `json:"categories"`
}

// VerifyCategoryResult holds verification data for a single category.
type VerifyCategoryResult struct {
	Count   int            `json:"count"`
	Samples []SampleResult `json:"samples"`
}

// SampleResult represents a sample document.
type SampleResult struct {
	Title  string `json:"title"`
	Source string `json:"source"`
}

// KeywordSystem manages category keywords across multiple languages.
type KeywordSystem struct {
	keywordsDir string
	dbPath      string
	mu          sync.RWMutex
	cache       map[string]map[string][]string // lang -> category -> keywords
}

// NewKeywordSystem creates a new keyword system.
func NewKeywordSystem(keywordsDir, dbPath string) *KeywordSystem {
	return &KeywordSystem{
		keywordsDir: keywordsDir,
		dbPath:      dbPath,
		cache:       make(map[string]map[string][]string),
	}
}

// LoadKeywords loads keywords for a specific language.
func (ks *KeywordSystem) LoadKeywords(lang string) (map[string][]string, error) {
	ks.mu.RLock()
	if cached, ok := ks.cache[lang]; ok {
		ks.mu.RUnlock()
		return cached, nil
	}
	ks.mu.RUnlock()

	path := filepath.Join(ks.keywordsDir, lang+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]string), nil
		}
		return nil, err
	}

	var kf KeywordFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for cat, catData := range kf.Categories {
		result[cat] = catData.Keywords
	}

	ks.mu.Lock()
	ks.cache[lang] = result
	ks.mu.Unlock()

	return result, nil
}

// SaveKeywords saves keywords for a specific language.
func (ks *KeywordSystem) SaveKeywords(lang string, keywords map[string][]string) error {
	path := filepath.Join(ks.keywordsDir, lang+".json")

	var kf KeywordFile

	// Load existing file if present
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &kf)
	}

	// Initialize if needed
	if kf.Categories == nil {
		kf = KeywordFile{
			Version:    "2.0",
			Language:   lang,
			MatchType:  "partial",
			Cleaned:    true,
			Categories: make(map[string]CategoryData),
		}
	}

	// Update categories
	for cat, kwList := range keywords {
		kf.Categories[cat] = CategoryData{
			Keywords:      kwList,
			Count:         len(kwList),
			OriginalCount: len(kwList),
		}
	}

	data, err := json.MarshalIndent(kf, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Update cache
	ks.mu.Lock()
	ks.cache[lang] = keywords
	ks.mu.Unlock()

	return os.WriteFile(path, data, 0644)
}

// ExtractKeywordsFromDB extracts keywords from database documents.
func (ks *KeywordSystem) ExtractKeywordsFromDB(lang, category string, limit int) ([]string, error) {
	if ks.dbPath == "" {
		return nil, nil
	}

	db, err := sql.Open("sqlite3", ks.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT title
		FROM documents
		WHERE language = ? AND normalized_category = ?
		ORDER BY indexed_at DESC
		LIMIT 1000
	`, lang, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Count word frequencies
	wordFreq := make(map[string]int)
	for rows.Next() {
		var title sql.NullString
		if err := rows.Scan(&title); err != nil {
			continue
		}
		if !title.Valid {
			continue
		}

		words := strings.Fields(strings.ToLower(title.String))
		for _, word := range words {
			if len(word) >= 2 {
				wordFreq[word]++
			}
		}
	}

	// Sort by frequency
	type wordCount struct {
		word  string
		count int
	}
	sorted := make([]wordCount, 0, len(wordFreq))
	for w, c := range wordFreq {
		sorted = append(sorted, wordCount{w, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Return top keywords
	result := make([]string, 0, limit)
	for i, wc := range sorted {
		if i >= limit {
			break
		}
		result = append(result, wc.word)
	}

	return result, nil
}

// ProcessLanguage processes keywords for a single language.
func (ks *KeywordSystem) ProcessLanguage(lang string, translateFn func([]string, string) []string) (*ProcessResult, error) {
	result := &ProcessResult{
		Lang:       lang,
		Categories: make(map[string]int),
		Status:     "ok",
	}

	existing, err := ks.LoadKeywords(lang)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}

	updated := make(map[string][]string)

	for _, cat := range Categories {
		// 1. Load existing keywords
		existingKw := existing[cat]

		// 2. Extract from DB
		dbKw, _ := ks.ExtractKeywordsFromDB(lang, cat, 50)

		// 3. Translate seed keywords if needed
		var translatedKw []string
		if translateFn != nil && lang != "en" && len(existingKw) < 10 {
			if seed, ok := SeedKeywords[cat]; ok {
				translatedKw = translateFn(seed, lang)
			}
		}

		// 4. Merge and deduplicate
		allKw := mergeKeywords(existingKw, dbKw, translatedKw)

		// 5. Filter noise
		filtered := filterKeywords(allKw)

		// 6. Limit to 100
		if len(filtered) > 100 {
			filtered = filtered[:100]
		}

		updated[cat] = filtered
		result.Categories[cat] = len(filtered)
	}

	// Save
	if err := ks.SaveKeywords(lang, updated); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}

	return result, nil
}

// VerifyClassification verifies classification for a language.
func (ks *KeywordSystem) VerifyClassification(lang string, sampleLimit int) (*VerifyResult, error) {
	if ks.dbPath == "" {
		return nil, nil
	}

	db, err := sql.Open("sqlite3", ks.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := &VerifyResult{
		Lang:       lang,
		Categories: make(map[string]VerifyCategoryResult),
	}

	for _, cat := range Categories {
		rows, err := db.Query(`
			SELECT title, category_source
			FROM documents
			WHERE language = ? AND normalized_category = ?
			ORDER BY indexed_at DESC
			LIMIT ?
		`, lang, cat, sampleLimit)
		if err != nil {
			continue
		}

		var samples []SampleResult
		count := 0
		for rows.Next() {
			var title, source sql.NullString
			if err := rows.Scan(&title, &source); err != nil {
				continue
			}
			count++

			if len(samples) < 3 && title.Valid {
				t := title.String
				if len(t) > 50 {
					t = t[:50]
				}
				samples = append(samples, SampleResult{
					Title:  t,
					Source: source.String,
				})
			}
		}
		rows.Close()

		result.Categories[cat] = VerifyCategoryResult{
			Count:   count,
			Samples: samples,
		}
	}

	return result, nil
}

// GetKeywordsForCategory returns keywords for a specific language and category.
func (ks *KeywordSystem) GetKeywordsForCategory(lang, category string) ([]string, error) {
	keywords, err := ks.LoadKeywords(lang)
	if err != nil {
		return nil, err
	}
	return keywords[category], nil
}

// IsTopLanguage checks if a language is in the top 10.
func IsTopLanguage(lang string) bool {
	for _, l := range TopLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

// GetLanguageName returns the full name for a language code.
func GetLanguageName(code string) string {
	if name, ok := LanguageNames[code]; ok {
		return name
	}
	return code
}

// CategoryExists checks if a category is valid.
func CategoryExists(category string) bool {
	for _, c := range Categories {
		if c == category {
			return true
		}
	}
	return false
}

// mergeKeywords merges multiple keyword slices and removes duplicates.
func mergeKeywords(slices ...[]string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, slice := range slices {
		for _, kw := range slice {
			if !seen[kw] {
				seen[kw] = true
				result = append(result, kw)
			}
		}
	}

	return result
}

// filterKeywords removes noise from keywords.
func filterKeywords(keywords []string) []string {
	var result []string
	for _, kw := range keywords {
		// Skip too short
		if len(kw) < 2 {
			continue
		}
		// Skip pure digits
		if isDigitOnly(kw) {
			continue
		}
		result = append(result, kw)
	}
	return result
}

// isDigitOnly checks if a string contains only digits.
func isDigitOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
