// Package analyzer provides domain analysis for topic understanding and ontology generation.
//
// 도메인 분석기 - LLM으로 토픽 이해 및 온톨로지 생성
//
// Stage 1-2:
//   - 토픽 정의
//   - 하위 도메인 추출
//   - 관련 키워드 생성
//   - 권위 기관 식별
//   - FTS 검색 표현식 생성
package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// AnalysisDepth controls the level of detail in analysis
type AnalysisDepth string

const (
	DepthQuick    AnalysisDepth = "quick"
	DepthStandard AnalysisDepth = "standard"
	DepthThorough AnalysisDepth = "thorough"
)

// Keywords represents categorized keywords for a domain
type Keywords struct {
	Core       []string `json:"core"`       // 핵심 키워드 5-10개
	Extended   []string `json:"extended"`   // 확장 키워드 10-20개
	Brands     []string `json:"brands"`     // 주요 브랜드/기업 5-10개
	Techniques []string `json:"techniques"` // 기술/방법론 5-10개
}

// DomainAnalysis represents the complete analysis result for a topic
type DomainAnalysis struct {
	Definition    string   `json:"definition"`     // 토픽의 간결한 정의 (1-2문장)
	Subtopics     []string `json:"subtopics"`      // 하위 도메인 목록
	Keywords      []string `json:"keywords"`       // 모든 키워드 (평탄화된 목록)
	KeywordsFull  Keywords `json:"keywords_full"`  // 카테고리별 키워드
	Authorities   []string `json:"authorities"`    // 권위 있는 기관/사이트 도메인
	RelatedFields []string `json:"related_fields"` // 관련 분야
	FitExpr       string   `json:"fit_expr"`       // FTS5 검색용 OR 표현식
	SearchQueries []string `json:"search_queries"` // 피드 발굴용 검색 쿼리
}

// LLMProvider interface for external LLM integration
// Implement this interface to add LLM support (Claude, OpenAI, Gemini, etc.)
type LLMProvider interface {
	// Generate sends a prompt to the LLM and returns the response text
	Generate(prompt string) (string, error)
	// Name returns the provider name (e.g., "claude", "openai", "gemini")
	Name() string
}

// DomainAnalyzer analyzes topics to extract ontology and keywords
// LLM 기반 도메인 분석기
type DomainAnalyzer struct {
	// LLMProviders is a list of LLM providers to try in order (Claude first, then fallback)
	LLMProviders []LLMProvider
}

// NewDomainAnalyzer creates a new analyzer instance
func NewDomainAnalyzer(providers ...LLMProvider) *DomainAnalyzer {
	return &DomainAnalyzer{
		LLMProviders: providers,
	}
}

// Analyze performs domain analysis on a topic
// Returns structured analysis with keywords, subtopics, authorities, etc.
func (da *DomainAnalyzer) Analyze(topic string, languages []string, depth AnalysisDepth) (*DomainAnalysis, error) {
	if depth == "" {
		depth = DepthStandard
	}

	prompt := da.buildPrompt(topic, languages, depth)

	// Try each LLM provider in order
	for _, provider := range da.LLMProviders {
		response, err := provider.Generate(prompt)
		if err != nil {
			continue
		}

		result, err := da.parseJSON(response)
		if err == nil && len(result.Keywords) > 0 {
			return result, nil
		}
	}

	// LLM 없거나 실패하면 규칙 기반
	// (LLM 없음 - 규칙 기반 분석)
	return da.analyzeRules(topic, languages), nil
}

// buildPrompt creates the analysis prompt for LLM
func (da *DomainAnalyzer) buildPrompt(topic string, languages []string, depth AnalysisDepth) string {
	detailLevel := map[AnalysisDepth]string{
		DepthQuick:    "간략하게",
		DepthStandard: "적절한 수준으로",
		DepthThorough: "상세하게",
	}[depth]
	if detailLevel == "" {
		detailLevel = "적절한 수준으로"
	}

	return fmt.Sprintf(`당신은 도메인 전문가입니다. 다음 토픽에 대해 %s 분석해주세요.

토픽: %s
대상 언어: %s

다음 JSON 형식으로 응답하세요 (JSON만, 설명 없이):

{
  "definition": "토픽의 간결한 정의 (1-2문장)",
  "subtopics": ["하위 도메인 목록", "5-15개"],
  "keywords": {
    "core": ["핵심 키워드 5-10개"],
    "extended": ["확장 키워드 10-20개"],
    "brands": ["주요 브랜드/기업 5-10개"],
    "techniques": ["기술/방법론 5-10개"]
  },
  "authorities": ["권위 있는 기관/사이트 도메인 5-10개"],
  "related_fields": ["관련 분야 3-5개"],
  "fit_expr": "FTS5 검색용 OR 표현식 (도메인 특화 핵심 키워드, 한국어+영어 모두 포함, 일반 단어 제외. 예: 'clinical trial' OR FDA OR 임상시험 OR 신약 OR drug)",
  "search_queries": ["피드 발굴용 검색 쿼리 5-10개"]
}

중요:
- keywords는 대상 언어 모두 포함 (예: bread, 빵, パン)
- authorities는 .com, .org, .go.kr 등 도메인만
- fit_expr은 FTS5 MATCH 구문용 (OR로 연결, 핵심만)
- search_queries는 RSS/블로그 발굴용`, detailLevel, topic, strings.Join(languages, ", "))
}

// parseJSON extracts and parses JSON from LLM response
func (da *DomainAnalyzer) parseJSON(text string) (*DomainAnalysis, error) {
	// JSON 블록 추출
	jsonPattern := regexp.MustCompile(`\{[\s\S]*\}`)
	match := jsonPattern.FindString(text)
	if match == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	// Parse the intermediate structure with nested keywords
	var raw struct {
		Definition    string          `json:"definition"`
		Subtopics     []string        `json:"subtopics"`
		Keywords      json.RawMessage `json:"keywords"`
		Authorities   []string        `json:"authorities"`
		RelatedFields []string        `json:"related_fields"`
		FitExpr       string          `json:"fit_expr"`
		SearchQueries []string        `json:"search_queries"`
	}

	if err := json.Unmarshal([]byte(match), &raw); err != nil {
		return nil, err
	}

	result := &DomainAnalysis{
		Definition:    raw.Definition,
		Subtopics:     raw.Subtopics,
		Authorities:   raw.Authorities,
		RelatedFields: raw.RelatedFields,
		FitExpr:       raw.FitExpr,
		SearchQueries: raw.SearchQueries,
	}

	// 키워드 평탄화 - handle both nested object and array formats
	if len(raw.Keywords) > 0 {
		// Try parsing as Keywords object first
		var kw Keywords
		if err := json.Unmarshal(raw.Keywords, &kw); err == nil {
			result.KeywordsFull = kw
			// Flatten keywords
			seen := make(map[string]bool)
			for _, k := range kw.Core {
				if !seen[k] {
					result.Keywords = append(result.Keywords, k)
					seen[k] = true
				}
			}
			for _, k := range kw.Extended {
				if !seen[k] {
					result.Keywords = append(result.Keywords, k)
					seen[k] = true
				}
			}
			for _, k := range kw.Brands {
				if !seen[k] {
					result.Keywords = append(result.Keywords, k)
					seen[k] = true
				}
			}
			for _, k := range kw.Techniques {
				if !seen[k] {
					result.Keywords = append(result.Keywords, k)
					seen[k] = true
				}
			}
		} else {
			// Try parsing as flat array
			var flat []string
			if err := json.Unmarshal(raw.Keywords, &flat); err == nil {
				result.Keywords = flat
			}
		}
	}

	return result, nil
}

// DomainExpansion contains predefined expansions for known domains
type DomainExpansion struct {
	Keywords    []string
	Authorities []string
	Subtopics   []string
}

// 알려진 도메인별 확장
var domainExpansions = map[string]DomainExpansion{
	"bakery": {
		Keywords:    []string{"bread", "baking", "pastry", "sourdough", "croissant", "cake", "빵", "베이커리", "제빵"},
		Authorities: []string{"kingarthurbaking.com", "theperfectloaf.com", "seriouseats.com"},
		Subtopics:   []string{"sourdough", "pastry", "cake", "bread", "dessert"},
	},
	"coffee": {
		Keywords:    []string{"coffee", "espresso", "barista", "brewing", "roasting", "커피", "에스프레소", "바리스타"},
		Authorities: []string{"dailycoffeenews.com", "perfectdailygrind.com", "sprudge.com"},
		Subtopics:   []string{"espresso", "brewing", "roasting", "latte art", "coffee beans"},
	},
	"wine": {
		Keywords:    []string{"wine", "vineyard", "winery", "sommelier", "grape", "vintage", "와인", "포도주"},
		Authorities: []string{"winespectator.com", "decanter.com", "wine-searcher.com"},
		Subtopics:   []string{"red wine", "white wine", "champagne", "vineyard", "tasting"},
	},
	"pharma": {
		Keywords:    []string{"pharmaceutical", "drug", "FDA", "clinical", "trial", "biotech", "제약", "신약", "임상"},
		Authorities: []string{"fda.gov", "clinicaltrials.gov", "nih.gov", "nature.com"},
		Subtopics:   []string{"clinical trials", "drug approval", "biotech", "vaccines", "therapy"},
	},
	"legal": {
		Keywords:    []string{"law", "legal", "attorney", "court", "lawsuit", "litigation", "법률", "변호사", "소송"},
		Authorities: []string{"law.cornell.edu", "scotusblog.com", "lawfare.blog"},
		Subtopics:   []string{"corporate law", "litigation", "contracts", "intellectual property"},
	},
	"politics": {
		Keywords:    []string{"politics", "election", "government", "policy", "congress", "정치", "선거", "정부", "국회"},
		Authorities: []string{"politico.com", "thehill.com", "fivethirtyeight.com", "ballotpedia.org"},
		Subtopics:   []string{"elections", "legislation", "foreign policy", "domestic policy", "campaigns"},
	},
	"technology": {
		Keywords:    []string{"tech", "software", "AI", "machine learning", "startup", "기술", "인공지능", "스타트업"},
		Authorities: []string{"techcrunch.com", "wired.com", "arstechnica.com", "theverge.com"},
		Subtopics:   []string{"AI/ML", "cloud computing", "cybersecurity", "startups", "hardware"},
	},
	"finance": {
		Keywords:    []string{"finance", "investing", "stocks", "market", "banking", "금융", "투자", "주식"},
		Authorities: []string{"bloomberg.com", "wsj.com", "ft.com", "reuters.com"},
		Subtopics:   []string{"stock market", "cryptocurrency", "banking", "fintech", "trading"},
	},
}

// analyzeRules performs rule-based analysis when LLM is unavailable
// 규칙 기반 분석 (LLM 없을 때)
func (da *DomainAnalyzer) analyzeRules(topic string, languages []string) *DomainAnalysis {
	topicLower := strings.ToLower(topic)
	words := extractWords(topicLower)

	// Find matching domain expansion
	var expansion DomainExpansion
	var found bool
	for key, data := range domainExpansions {
		if strings.Contains(topicLower, key) {
			expansion = data
			found = true
			break
		}
		// Check if any of the first 3 keywords match
		for i, kw := range data.Keywords {
			if i >= 3 {
				break
			}
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				expansion = data
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	var keywords []string
	if found {
		keywords = expansion.Keywords
	} else {
		// 기본 확장
		keywords = words
		for _, w := range words {
			if len(w) > 3 {
				keywords = append(keywords, w+"s")
				keywords = append(keywords, w+"ing")
			}
		}
	}

	// Deduplicate keywords
	keywords = uniqueStrings(keywords)

	// Build FTS expression from first 10 keywords
	fitExprKeywords := keywords
	if len(fitExprKeywords) > 10 {
		fitExprKeywords = fitExprKeywords[:10]
	}

	subtopics := expansion.Subtopics
	if len(subtopics) == 0 {
		subtopics = words
	}

	return &DomainAnalysis{
		Definition:    fmt.Sprintf("%s 관련 콘텐츠", topic),
		Subtopics:     subtopics,
		Keywords:      keywords,
		Authorities:   expansion.Authorities,
		RelatedFields: []string{},
		FitExpr:       strings.Join(fitExprKeywords, " OR "),
		SearchQueries: []string{
			topic + " RSS feed",
			topic + " blog",
			"best " + topic + " sites",
			topic + " news",
		},
	}
}

// extractWords extracts alphanumeric words from text
func extractWords(text string) []string {
	wordPattern := regexp.MustCompile(`\w+`)
	return wordPattern.FindAllString(text, -1)
}

// uniqueStrings returns deduplicated slice preserving order
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// AddDomainExpansion adds or updates a domain expansion rule
// Useful for dynamically adding domain knowledge
func AddDomainExpansion(domain string, expansion DomainExpansion) {
	domainExpansions[strings.ToLower(domain)] = expansion
}

// GetDomainExpansion returns the expansion for a known domain, if any
func GetDomainExpansion(domain string) (DomainExpansion, bool) {
	exp, ok := domainExpansions[strings.ToLower(domain)]
	return exp, ok
}

// ListKnownDomains returns all pre-defined domain names
func ListKnownDomains() []string {
	domains := make([]string, 0, len(domainExpansions))
	for k := range domainExpansions {
		domains = append(domains, k)
	}
	return domains
}
