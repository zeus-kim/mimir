package sources

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed authoritative_sources.jsonl
var embeddedSourcesData []byte

// AuthoritySource represents a curated tier-1 source
type AuthoritySource struct {
	Topic       string  `json:"topic"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Language    string  `json:"language"`
	Tier        int     `json:"tier"`
	FeedType    string  `json:"feed_type"`
	Source      string  `json:"source,omitempty"`
	TrustScore  float64 `json:"trust_score,omitempty"`
}

// DomainKeywords maps topics to their associated keywords for matching
// Keys correspond to topics in the JSONL data file
var DomainKeywords = map[string][]string{
	// health covers pharma/clinical trial keywords
	"health":    {"제약", "임상", "신약", "FDA", "clinical trial", "pharmaceutical", "drug", "medicine", "health", "medical", "healthcare", "NIH", "CDC"},
	"legal":     {"법률", "법원", "판례", "소송", "legal", "law", "court", "attorney", "scotus"},
	"bakery":    {"빵", "베이커리", "제빵", "bread", "baking", "pastry", "sourdough"},
	"politics":  {"정치", "국회", "대통령", "선거", "politics", "government", "election", "congress", "senate"},
	"ai":        {"AI", "인공지능", "머신러닝", "딥러닝", "machine learning", "deep learning", "LLM", "artificial intelligence", "neural"},
	"crypto":    {"비트코인", "암호화폐", "블록체인", "bitcoin", "crypto", "blockchain", "ethereum", "cryptocurrency"},
	"tech":      {"technology", "software", "hardware", "programming", "developer", "tech", "startup", "venture"},
	"science":   {"science", "research", "physics", "biology"},
	"business":  {"business", "finance", "economy", "market", "investment", "벤처", "투자", "스타트업"},
	"security":  {"security", "cybersecurity", "hacking", "vulnerability", "infosec"},
	"space":     {"space", "astronomy", "nasa", "rocket", "satellite"},
	"energy":    {"energy", "renewable", "solar", "wind", "nuclear", "electricity"},
	"dev":       {"developer", "programming", "coding", "github", "opensource"},
	"economics": {"economics", "macroeconomics", "monetary", "fiscal", "federal reserve"},
	"math":      {"mathematics", "math", "algebra", "calculus", "statistics"},
	"chemistry": {"chemistry", "chemical", "molecule", "reaction", "compound"},
}

// AuthorityRegistry holds all loaded authority sources
type AuthorityRegistry struct {
	sources   []AuthoritySource
	byTopic   map[string][]AuthoritySource
}

// NewAuthorityRegistry creates a new registry with embedded data
func NewAuthorityRegistry() (*AuthorityRegistry, error) {
	return LoadAuthorityRegistry(embeddedSourcesData)
}

// LoadAuthorityRegistry loads sources from JSONL data
func LoadAuthorityRegistry(data []byte) (*AuthorityRegistry, error) {
	reg := &AuthorityRegistry{
		sources: make([]AuthoritySource, 0),
		byTopic: make(map[string][]AuthoritySource),
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var src AuthoritySource
		if err := json.Unmarshal([]byte(line), &src); err != nil {
			continue // skip malformed lines
		}

		// Set defaults
		src.Source = "authority"
		if src.TrustScore == 0 {
			src.TrustScore = 0.95
		}

		reg.sources = append(reg.sources, src)
		reg.byTopic[src.Topic] = append(reg.byTopic[src.Topic], src)
	}

	return reg, scanner.Err()
}

// GetByTopic returns all sources for a specific topic
func (r *AuthorityRegistry) GetByTopic(topic string) []AuthoritySource {
	topic = strings.ToLower(topic)
	return r.byTopic[topic]
}

// GetByTier returns all sources at or above a tier level (1 is highest)
func (r *AuthorityRegistry) GetByTier(maxTier int) []AuthoritySource {
	var result []AuthoritySource
	for _, src := range r.sources {
		if src.Tier <= maxTier {
			result = append(result, src)
		}
	}
	return result
}

// All returns all authority sources
func (r *AuthorityRegistry) All() []AuthoritySource {
	return r.sources
}

// GetAuthorityFeeds returns authority feeds matching the topic and keywords
// This is the main entry point, matching the Python get_authority_feeds function
func GetAuthorityFeeds(topic string, keywords []string) ([]AuthoritySource, error) {
	reg, err := NewAuthorityRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Match(topic, keywords), nil
}

// Match finds sources that match the given topic or keywords
func (r *AuthorityRegistry) Match(topic string, keywords []string) []AuthoritySource {
	topicLower := strings.ToLower(topic)
	keywordsLower := make([]string, len(keywords))
	for i, kw := range keywords {
		keywordsLower[i] = strings.ToLower(kw)
	}

	seen := make(map[string]bool)
	var matched []AuthoritySource

	// Check each domain's keywords against the input topic/keywords
	for domain, domainKeywords := range DomainKeywords {
		domainKeywordsLower := make([]string, len(domainKeywords))
		for i, dk := range domainKeywords {
			domainKeywordsLower[i] = strings.ToLower(dk)
		}

		shouldInclude := false

		// Check if any domain keyword is in the topic
		for _, dk := range domainKeywordsLower {
			if strings.Contains(topicLower, dk) {
				shouldInclude = true
				break
			}
		}

		// Check if any domain keyword matches input keywords
		if !shouldInclude {
			for _, dk := range domainKeywordsLower {
				for _, kw := range keywordsLower {
					if strings.Contains(kw, dk) || strings.Contains(dk, kw) {
						shouldInclude = true
						break
					}
				}
				if shouldInclude {
					break
				}
			}
		}

		// If matched, add all sources for this domain
		if shouldInclude {
			for _, src := range r.byTopic[domain] {
				if !seen[src.URL] {
					seen[src.URL] = true
					matched = append(matched, src)
				}
			}
		}
	}

	return matched
}

// Topics returns all available topic names
func (r *AuthorityRegistry) Topics() []string {
	topics := make([]string, 0, len(r.byTopic))
	for topic := range r.byTopic {
		topics = append(topics, topic)
	}
	return topics
}

// Count returns the total number of sources
func (r *AuthorityRegistry) Count() int {
	return len(r.sources)
}

// CountByTopic returns source count per topic
func (r *AuthorityRegistry) CountByTopic() map[string]int {
	counts := make(map[string]int)
	for topic, sources := range r.byTopic {
		counts[topic] = len(sources)
	}
	return counts
}
