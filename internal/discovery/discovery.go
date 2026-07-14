package discovery

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/ranking"
)

// Source represents a discovered source
type Source struct {
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Type        string  `json:"type"` // rss, webpage, youtube, github, academic, api
	Language    string  `json:"language"`
	Category    string  `json:"category,omitempty"`
	Score       float64 `json:"score"`
	Validated   bool    `json:"validated"`
	Stars       int     `json:"stars,omitempty"` // For GitHub repos
	Tier        int     `json:"tier,omitempty"`  // Trust tier (1=highest)
}

// Discoverer interface for different source types
type Discoverer interface {
	Discover(topic string, keywords []string, limit int) ([]Source, error)
	Name() string
}

// AllDiscoverers returns all available discoverers
func AllDiscoverers() []Discoverer {
	return []Discoverer{
		NewAcademicDiscoverer(),
		NewBlogDiscoverer(),
		NewGitHubDiscoverer(),
		NewGovDiscoverer(),
		NewRSSDiscoverer(),
		NewYouTubeDiscoverer(),
	}
}

// DomainBootstrapper orchestrates the discovery process
// Implements DISCO-style seed expansion
type DomainBootstrapper struct {
	Topic      string
	Keywords   []string
	Languages  []string
	Classifier *ranking.RelevanceClassifier
	Ranker     *ranking.BayesianRanker
	Discovered []Source
}

func NewDomainBootstrapper(topic string, keywords, languages []string) *DomainBootstrapper {
	return &DomainBootstrapper{
		Topic:      topic,
		Keywords:   keywords,
		Languages:  languages,
		Classifier: ranking.NewRelevanceClassifier(keywords, topic),
		Ranker:     ranking.NewBayesianRanker(2.0),
	}
}

// Bootstrap runs the full discovery pipeline
func (db *DomainBootstrapper) Bootstrap(discoverers []Discoverer, limit int) ([]Source, error) {
	var allSources []Source

	// Stage 1: Discover from all sources
	for _, d := range discoverers {
		sources, err := d.Discover(db.Topic, db.Keywords, limit)
		if err != nil {
			continue
		}
		allSources = append(allSources, sources...)
	}

	// Stage 2: Score and filter using relevance classifier
	var relevant []Source
	for _, s := range allSources {
		text := s.Title + " " + s.Description
		score := db.Classifier.Score(text)
		s.Score = score
		if score >= 0.3 {
			relevant = append(relevant, s)
		}
	}

	// Stage 3: Add top sources to ranker as positive examples
	for i, s := range relevant {
		if i < 10 && s.Score > 0.6 {
			db.Ranker.AddPositive(s.Title + " " + s.Description)
		}
	}

	// Stage 4: Re-rank using Bayesian ranker
	if len(db.Ranker.PositiveDocs) > 0 {
		candidates := make([]map[string]interface{}, len(relevant))
		for i, s := range relevant {
			candidates[i] = map[string]interface{}{
				"text":  s.Title + " " + s.Description,
				"index": i,
			}
		}

		ranked := db.Ranker.Rank(candidates)
		reordered := make([]Source, len(ranked))
		for i, r := range ranked {
			idx := r.Data.(map[string]interface{})["index"].(int)
			reordered[i] = relevant[idx]
			reordered[i].Score = r.BayesianScore
		}
		relevant = reordered
	}

	// Stage 5: Validate sources
	for i := range relevant {
		relevant[i].Validated = validateSource(relevant[i].URL)
	}

	db.Discovered = relevant
	return relevant, nil
}

// Helper functions

// resolveURL resolves a potentially relative URL against a base URL
func resolveURL(baseURL, ref string) string {
	if ref == "" {
		return ""
	}

	// Already absolute
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}

	return base.ResolveReference(refURL).String()
}

func validateSource(u string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(u)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

var rssPattern = regexp.MustCompile(`(?i)<rss|<feed|<atom`)

func detectRSSFeed(body []byte) bool {
	return rssPattern.Match(body[:min(1000, len(body))])
}
