// Package classifier provides feed classification functionality.
// It classifies RSS feeds by type, language, quality tier, and domain/topic.
package classifier

import (
	"net/url"
	"strings"
)

// FeedType represents the type of feed source.
type FeedType string

const (
	FeedTypeNews          FeedType = "news"
	FeedTypeBlog          FeedType = "blog"
	FeedTypeTech          FeedType = "tech"
	FeedTypeFinance       FeedType = "finance"
	FeedTypeSports        FeedType = "sports"
	FeedTypeEntertainment FeedType = "entertainment"
	FeedTypeScience       FeedType = "science"
	FeedTypeHealth        FeedType = "health"
	FeedTypePolitics      FeedType = "politics"
	FeedTypeYouTube       FeedType = "youtube"
	FeedTypeOther         FeedType = "other"
)

// QualityTier represents the quality level of a feed.
type QualityTier string

const (
	QualityTierPremium  QualityTier = "premium"  // Major outlets, high signal
	QualityTierStandard QualityTier = "standard" // Established sources
	QualityTierBasic    QualityTier = "basic"    // Personal blogs, smaller sites
	QualityTierUnknown  QualityTier = "unknown"
)

// FeedClassification contains all classification metadata for a feed.
type FeedClassification struct {
	URL         string      `json:"url"`
	Domain      string      `json:"domain"`
	Country     string      `json:"country"`
	Language    string      `json:"lang"`
	Category    FeedType    `json:"category"`
	QualityTier QualityTier `json:"quality_tier"`
}

// tldToCountry maps TLDs to country codes.
var tldToCountry = map[string]string{
	// Asia
	"kr": "kr", "co.kr": "kr", "or.kr": "kr", "go.kr": "kr",
	"jp": "jp", "co.jp": "jp", "or.jp": "jp",
	"cn": "cn", "com.cn": "cn",
	"tw": "tw", "com.tw": "tw",
	"hk": "hk", "com.hk": "hk",
	"sg": "sg", "com.sg": "sg",
	"my": "my", "com.my": "my",
	"th": "th", "co.th": "th",
	"vn": "vn", "com.vn": "vn",
	"id": "id", "co.id": "id",
	"ph": "ph", "com.ph": "ph",
	"in": "in", "co.in": "in",
	"pk": "pk", "com.pk": "pk",
	"bd": "bd", "com.bd": "bd",

	// Europe
	"uk": "uk", "co.uk": "uk", "org.uk": "uk",
	"de": "de",
	"fr": "fr",
	"es": "es", "com.es": "es",
	"it": "it",
	"nl": "nl",
	"be": "be",
	"at": "at", "co.at": "at",
	"ch": "ch",
	"se": "se",
	"no": "no",
	"dk": "dk",
	"fi": "fi",
	"pl": "pl", "com.pl": "pl",
	"cz": "cz",
	"ru": "ru",
	"ua": "ua", "com.ua": "ua",
	"pt": "pt",
	"gr": "gr",
	"ro": "ro",
	"hu": "hu",
	"sk": "sk",
	"bg": "bg",
	"hr": "hr",
	"rs": "rs",
	"si": "si",

	// Americas
	"us": "us",
	"ca": "ca",
	"mx": "mx", "com.mx": "mx",
	"br": "br", "com.br": "br",
	"ar": "ar", "com.ar": "ar",
	"cl": "cl",
	"co": "co", "com.co": "co",
	"pe": "pe", "com.pe": "pe",
	"ve": "ve",
	"ec": "ec", "com.ec": "ec",

	// Middle East / Africa
	"ae": "ae",
	"sa": "sa", "com.sa": "sa",
	"il": "il", "co.il": "il",
	"tr": "tr", "com.tr": "tr",
	"eg": "eg", "com.eg": "eg",
	"za": "za", "co.za": "za",
	"ng": "ng", "com.ng": "ng",
	"ke": "ke", "co.ke": "ke",

	// Oceania
	"au": "au", "com.au": "au",
	"nz": "nz", "co.nz": "nz",
}

// countryToLang maps country codes to primary language codes.
var countryToLang = map[string]string{
	"kr": "ko", "jp": "ja", "cn": "zh", "tw": "zh", "hk": "zh",
	"vn": "vi", "th": "th", "id": "id", "my": "ms",
	"de": "de", "at": "de", "ch": "de",
	"fr": "fr", "be": "fr",
	"es": "es", "mx": "es", "ar": "es", "cl": "es", "co": "es", "pe": "es", "ve": "es", "ec": "es",
	"it": "it",
	"pt": "pt", "br": "pt",
	"nl": "nl",
	"ru": "ru", "ua": "uk",
	"pl": "pl", "cz": "cs", "sk": "sk",
	"se": "sv", "no": "no", "dk": "da", "fi": "fi",
	"gr": "el", "tr": "tr",
	"ro": "ro", "hu": "hu", "bg": "bg", "hr": "hr", "rs": "sr", "si": "sl",
	"in": "hi", "pk": "ur", "bd": "bn",
	"ae": "ar", "sa": "ar", "eg": "ar",
	"il": "he",
	// English-speaking countries
	"us": "en", "uk": "en", "ca": "en", "au": "en", "nz": "en",
	"sg": "en", "ph": "en", "ng": "en", "ke": "en", "za": "en",
}

// categoryKeywords maps feed types to keywords that indicate that category.
var categoryKeywords = map[FeedType][]string{
	FeedTypeNews: {
		"news", "noticias", "nachrichten", "actualite", "nyheter", "nyhet",
		"nieuws", "wiadomosci", "zpravy", "noviny", "novosti",
		"뉴스", "ニュース", "新闻",
	},
	FeedTypeTech: {
		"tech", "technology", "digital", "cyber", "software", "hardware",
		"gadget", "geek", "nerd", "coding", "developer", "programming",
		"ai", "ml", "cloud", "startup", "テック", "科技", "테크", "tecnologia",
	},
	FeedTypeFinance: {
		"finance", "finanz", "financial", "money", "investment", "invest",
		"stock", "market", "economy", "economic", "business", "bank",
		"trading", "crypto", "bitcoin", "経済", "财经", "금융", "경제",
	},
	FeedTypeSports: {
		"sport", "sports", "football", "soccer", "basketball", "baseball",
		"tennis", "golf", "nfl", "nba", "mlb", "espn",
		"スポーツ", "体育", "스포츠",
	},
	FeedTypeEntertainment: {
		"entertainment", "movie", "film", "music", "celebrity", "tv",
		"television", "drama", "anime", "manga", "game", "gaming",
		"エンタメ", "娱乐", "연예",
	},
	FeedTypeScience: {
		"science", "research", "scientific", "nature", "biology", "physics",
		"chemistry", "space", "nasa", "科学", "과학",
	},
	FeedTypeHealth: {
		"health", "medical", "medicine", "doctor", "hospital", "wellness",
		"fitness", "diet", "nutrition", "健康", "건강",
	},
	FeedTypePolitics: {
		"politics", "political", "government", "election", "vote", "congress",
		"parliament", "minister", "president", "政治", "정치",
	},
	FeedTypeBlog: {
		"blog", "wordpress", "blogger", "medium", "substack", "tumblr",
		"tistory", "naver.com/blog", "note.com",
	},
	FeedTypeYouTube: {
		"youtube.com", "youtu.be",
	},
}

// premiumDomains lists domains that are considered high-quality sources.
var premiumDomains = map[string]bool{
	// Major news outlets
	"nytimes.com": true, "washingtonpost.com": true, "wsj.com": true,
	"reuters.com": true, "apnews.com": true, "bbc.com": true, "bbc.co.uk": true,
	"theguardian.com": true, "economist.com": true, "ft.com": true,
	"bloomberg.com": true, "cnbc.com": true,
	// Tech
	"techcrunch.com": true, "wired.com": true, "arstechnica.com": true,
	"theverge.com": true, "zdnet.com": true,
	// Science
	"nature.com": true, "sciencemag.org": true, "science.org": true,
	// Korean major outlets
	"chosun.com": true, "donga.com": true, "joongang.co.kr": true,
	"hani.co.kr": true, "khan.co.kr": true, "hankyung.com": true,
	// Japanese major outlets
	"asahi.com": true, "yomiuri.co.jp": true, "nikkei.com": true,
	"mainichi.jp": true,
}

// globalTLDs are TLDs that don't indicate a specific country.
var globalTLDs = map[string]bool{
	"com": true, "org": true, "net": true, "io": true, "ai": true, "co": true,
}

// Classifier provides feed classification functionality.
type Classifier struct{}

// NewClassifier creates a new feed classifier.
func NewClassifier() *Classifier {
	return &Classifier{}
}

// ClassifyFeed classifies a single feed URL.
func (c *Classifier) ClassifyFeed(feedURL string) (*FeedClassification, error) {
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return nil, err
	}

	domain := parsed.Host
	// Remove www. prefix for cleaner domain matching
	domain = strings.TrimPrefix(domain, "www.")

	country := c.GetCountryFromTLD(domain)
	lang := c.GetLanguageFromCountry(country)
	category := c.GuessCategory(feedURL, domain)
	quality := c.AssessQuality(domain)

	return &FeedClassification{
		URL:         feedURL,
		Domain:      domain,
		Country:     country,
		Language:    lang,
		Category:    category,
		QualityTier: quality,
	}, nil
}

// GetCountryFromTLD infers country from domain TLD.
func (c *Classifier) GetCountryFromTLD(domain string) string {
	parts := strings.Split(strings.ToLower(domain), ".")
	if len(parts) < 2 {
		return "unknown"
	}

	// Check two-level TLD first (e.g., co.kr, com.br)
	if len(parts) >= 2 {
		twoLevel := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if country, ok := tldToCountry[twoLevel]; ok {
			return country
		}
	}

	// Check single-level TLD
	tld := parts[len(parts)-1]
	if country, ok := tldToCountry[tld]; ok {
		return country
	}

	// Global TLDs default to "global"
	if globalTLDs[tld] {
		return "global"
	}

	return "unknown"
}

// GetLanguageFromCountry returns the primary language for a country.
func (c *Classifier) GetLanguageFromCountry(country string) string {
	if lang, ok := countryToLang[country]; ok {
		return lang
	}
	if country == "global" {
		return "en" // Default to English for global domains
	}
	return "unknown"
}

// GuessCategory infers the feed category from URL and domain patterns.
func (c *Classifier) GuessCategory(feedURL, domain string) FeedType {
	urlLower := strings.ToLower(feedURL)
	domainLower := strings.ToLower(domain)

	for category, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(urlLower, kw) || strings.Contains(domainLower, kw) {
				return category
			}
		}
	}

	return FeedTypeOther
}

// AssessQuality determines the quality tier of a feed based on its domain.
func (c *Classifier) AssessQuality(domain string) QualityTier {
	domainLower := strings.ToLower(domain)

	// Check for exact match first
	if premiumDomains[domainLower] {
		return QualityTierPremium
	}

	// Check if it's a subdomain of a premium domain
	for premium := range premiumDomains {
		if strings.HasSuffix(domainLower, "."+premium) {
			return QualityTierPremium
		}
	}

	// Blog platforms are typically basic tier
	blogPlatforms := []string{
		"wordpress.com", "blogspot.com", "blogger.com", "medium.com",
		"substack.com", "tumblr.com", "tistory.com", "note.com",
	}
	for _, platform := range blogPlatforms {
		if strings.Contains(domainLower, platform) {
			return QualityTierBasic
		}
	}

	// Default to standard
	return QualityTierStandard
}

// ClassifyFeeds classifies multiple feed URLs.
func (c *Classifier) ClassifyFeeds(urls []string) ([]*FeedClassification, error) {
	results := make([]*FeedClassification, 0, len(urls))

	for _, feedURL := range urls {
		feedURL = strings.TrimSpace(feedURL)
		if feedURL == "" || strings.HasPrefix(feedURL, "#") {
			continue
		}

		classification, err := c.ClassifyFeed(feedURL)
		if err != nil {
			continue // Skip invalid URLs
		}
		results = append(results, classification)
	}

	return results, nil
}

// ClassificationStats holds aggregated statistics from feed classification.
type ClassificationStats struct {
	ByCountry  map[string]int  `json:"by_country"`
	ByLanguage map[string]int  `json:"by_language"`
	ByCategory map[FeedType]int `json:"by_category"`
	ByQuality  map[QualityTier]int `json:"by_quality"`
	Total      int             `json:"total"`
}

// ComputeStats aggregates statistics from a list of classifications.
func ComputeStats(classifications []*FeedClassification) *ClassificationStats {
	stats := &ClassificationStats{
		ByCountry:  make(map[string]int),
		ByLanguage: make(map[string]int),
		ByCategory: make(map[FeedType]int),
		ByQuality:  make(map[QualityTier]int),
		Total:      len(classifications),
	}

	for _, c := range classifications {
		stats.ByCountry[c.Country]++
		stats.ByLanguage[c.Language]++
		stats.ByCategory[c.Category]++
		stats.ByQuality[c.QualityTier]++
	}

	return stats
}

// DetectLanguageFromContent provides content-based language detection hints.
// This is a simple heuristic based on character ranges.
func DetectLanguageFromContent(text string) string {
	if len(text) == 0 {
		return "unknown"
	}

	var hasKorean, hasJapanese, hasChinese, hasCyrillic, hasArabic bool

	for _, r := range text {
		switch {
		case r >= 0xAC00 && r <= 0xD7AF: // Korean Hangul
			hasKorean = true
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF): // Hiragana/Katakana
			hasJapanese = true
		case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
			hasChinese = true
		case r >= 0x0400 && r <= 0x04FF: // Cyrillic
			hasCyrillic = true
		case r >= 0x0600 && r <= 0x06FF: // Arabic
			hasArabic = true
		}
	}

	// Priority: more specific scripts first
	switch {
	case hasKorean:
		return "ko"
	case hasJapanese:
		return "ja"
	case hasChinese:
		return "zh"
	case hasCyrillic:
		return "ru" // Could be other Cyrillic languages
	case hasArabic:
		return "ar"
	default:
		return "en" // Default to English for Latin scripts
	}
}
