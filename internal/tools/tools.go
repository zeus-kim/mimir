package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/bootstrap"
	"github.com/zeus-kim/mimir/internal/curator"
	"github.com/zeus-kim/mimir/internal/db"
	"github.com/zeus-kim/mimir/internal/delivery"
	"github.com/zeus-kim/mimir/internal/discovery"
	"github.com/zeus-kim/mimir/internal/fetch"
	"github.com/zeus-kim/mimir/internal/hints"
	"github.com/zeus-kim/mimir/internal/lang"
	"github.com/zeus-kim/mimir/internal/ranking"
	"github.com/zeus-kim/mimir/internal/search"
	"github.com/zeus-kim/mimir/internal/sources"
	"github.com/zeus-kim/mimir/internal/tts"
	"github.com/zeus-kim/mimir/internal/validator"
)

type ToolHandler func(args map[string]interface{}) (interface{}, error)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     ToolHandler            `json:"-"`
}

type ToolRegistry struct {
	db       *db.DB
	TTS      tts.Engine
	Delivery delivery.Channel
	tools    map[string]Tool
}

func NewRegistry(d *db.DB) *ToolRegistry {
	r := &ToolRegistry{
		db:    d,
		TTS:   tts.GetEngine(""),
		tools: make(map[string]Tool),
	}
	r.registerAll()
	r.RegisterDomainTools()
	r.RegisterVerticalTools()
	r.RegisterSystemTools()
	return r
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name] = t
}

// Backwards compatibility
func (r *ToolRegistry) DB() *db.DB {
	return r.db
}

func (r *ToolRegistry) registerAll() {
	// === Data Fetching ===
	r.tools["fetch_clinical_trials"] = Tool{
		Name:        "fetch_clinical_trials",
		Description: "Fetch clinical trials from ClinicalTrials.gov",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]string{"type": "string", "description": "Search query (e.g., 'cancer immunotherapy')"},
				"limit": map[string]interface{}{"type": "integer", "default": 100},
			},
			"required": []string{"query"},
		},
	}

	r.tools["fetch_pubmed"] = Tool{
		Name:        "fetch_pubmed",
		Description: "Fetch articles from PubMed",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]string{"type": "string", "description": "Search query"},
				"limit": map[string]interface{}{"type": "integer", "default": 50},
			},
			"required": []string{"query"},
		},
	}

	r.tools["fetch_fda_approvals"] = Tool{
		Name:        "fetch_fda_approvals",
		Description: "Fetch FDA drug approvals",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]string{"type": "string", "description": "Drug name filter (optional)"},
				"limit": map[string]interface{}{"type": "integer", "default": 100},
			},
		},
	}

	r.tools["fetch_fda_adverse_events"] = Tool{
		Name:        "fetch_fda_adverse_events",
		Description: "Fetch FDA adverse event reports",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]string{"type": "string", "description": "Drug name filter (optional)"},
				"limit": map[string]interface{}{"type": "integer", "default": 100},
			},
		},
	}

	r.tools["fetch_sec_filings"] = Tool{
		Name:        "fetch_sec_filings",
		Description: "Fetch SEC filings (8-K, 10-K, etc.)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":      map[string]string{"type": "string", "description": "Search query"},
				"form_types": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "default": []string{"8-K"}},
				"limit":      map[string]interface{}{"type": "integer", "default": 100},
			},
			"required": []string{"query"},
		},
	}

	r.tools["fetch_research"] = Tool{
		Name:        "fetch_research",
		Description: "Fetch research data for a domain (pharma, biotech, oncology, neurology, metabolism)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]string{"type": "string", "description": "Research domain"},
			},
			"required": []string{"domain"},
		},
	}

	// === Discovery ===
	r.tools["discover_sources"] = Tool{
		Name:        "discover_sources",
		Description: "Discover sources for a topic using DISCO-style seed expansion (RSS, GitHub, Academic, Gov, YouTube, Blogs)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"topic":    map[string]string{"type": "string", "description": "Topic to discover sources for"},
				"keywords": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
				"limit":    map[string]interface{}{"type": "integer", "default": 20},
			},
			"required": []string{"topic"},
		},
	}

	r.tools["discover_rss"] = Tool{
		Name:        "discover_rss",
		Description: "Discover RSS feeds from a domain",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]string{"type": "string", "description": "Domain to crawl (e.g., 'techcrunch.com')"},
				"mode":   map[string]string{"type": "string", "description": "fast or enhanced (default: fast)"},
			},
			"required": []string{"domain"},
		},
	}

	r.tools["authority_sources"] = Tool{
		Name:        "authority_sources",
		Description: "Get curated authoritative sources for a topic",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"topic": map[string]string{"type": "string", "description": "Topic (ai, tech, science, pharma, etc.)"},
			},
			"required": []string{"topic"},
		},
	}

	// === Validation ===
	r.tools["validate_feeds"] = Tool{
		Name:        "validate_feeds",
		Description: "Validate RSS/Atom feed URLs",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"urls": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "URLs to validate"},
			},
			"required": []string{"urls"},
		},
	}

	r.tools["validate_sources"] = Tool{
		Name:        "validate_sources",
		Description: "Validate and score sources for quality and relevance",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"urls":     map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
				"language": map[string]string{"type": "string", "default": "en"},
			},
			"required": []string{"urls"},
		},
	}

	// === Ranking & Classification ===
	r.tools["score_relevance"] = Tool{
		Name:        "score_relevance",
		Description: "Score text relevance to a domain using TF-IDF classifier (ACHE style)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text":     map[string]string{"type": "string", "description": "Text to score"},
				"keywords": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
			},
			"required": []string{"text", "keywords"},
		},
	}

	r.tools["detect_language"] = Tool{
		Name:        "detect_language",
		Description: "Detect language of text",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]string{"type": "string"},
			},
			"required": []string{"text"},
		},
	}

	// === Curation ===
	r.tools["auto_curate"] = Tool{
		Name:        "auto_curate",
		Description: "Run automatic feed curation for a vertical (discover, validate, prune, index)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"vertical":  map[string]string{"type": "string", "description": "Vertical name"},
				"topic":     map[string]string{"type": "string", "description": "Topic keywords"},
				"languages": map[string]string{"type": "string", "default": "en,ko"},
				"limit":     map[string]interface{}{"type": "integer", "default": 50},
			},
			"required": []string{"vertical", "topic"},
		},
	}

	// === Bootstrap ===
	r.tools["bootstrap_vertical"] = Tool{
		Name:        "bootstrap_vertical",
		Description: "Create a new vertical search engine from a topic (full pipeline: analyze, discover, validate, curate, index)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"topic":     map[string]string{"type": "string", "description": "Topic to create vertical for"},
				"languages": map[string]string{"type": "string", "default": "en,ko"},
				"depth":     map[string]string{"type": "string", "description": "quick, standard, or thorough"},
			},
			"required": []string{"topic"},
		},
	}

	// === Search ===
	r.tools["search"] = Tool{
		Name:        "search",
		Description: "Search across all data sources",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":  map[string]string{"type": "string"},
				"source": map[string]string{"type": "string", "description": "clinical_trials, pubmed, fda_approvals, fda_adverse_events, sec_filings, or all"},
				"limit":  map[string]interface{}{"type": "integer", "default": 20},
			},
			"required": []string{"query"},
		},
	}

	r.tools["expand_query"] = Tool{
		Name:        "expand_query",
		Description: "Expand search query with synonyms and related terms",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":    map[string]string{"type": "string"},
				"keywords": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
			},
			"required": []string{"query"},
		},
	}

	// === Briefing ===
	r.tools["generate_briefing"] = Tool{
		Name:        "generate_briefing",
		Description: "Generate a briefing from collected data with optional TTS",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain":   map[string]string{"type": "string", "description": "Domain (pharma, legal, etc.)"},
				"language": map[string]string{"type": "string", "default": "ko"},
				"tts":      map[string]interface{}{"type": "boolean", "default": true},
			},
		},
	}

	r.tools["send_briefing"] = Tool{
		Name:        "send_briefing",
		Description: "Send a briefing via configured delivery channel (telegram, slack, discord, email, ntfy)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title":   map[string]string{"type": "string"},
				"body":    map[string]string{"type": "string"},
				"channel": map[string]string{"type": "string"},
			},
			"required": []string{"title", "body"},
		},
	}

	// === Domain Intelligence ===
	r.tools["domain_hints"] = Tool{
		Name:        "domain_hints",
		Description: "Get search hints, sources, and APIs for a domain (pharma, ai, legal, finance, tech, etc.)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]string{"type": "string"},
			},
			"required": []string{"domain"},
		},
	}

	r.tools["domain_strategy"] = Tool{
		Name:        "domain_strategy",
		Description: "Get exploration strategy for deep-diving into a domain (steps, questions, patterns)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]string{"type": "string"},
			},
			"required": []string{"domain"},
		},
	}

	r.tools["architecture_refs"] = Tool{
		Name:        "architecture_refs",
		Description: "Get reference architectures (ACHE, DISCO) that inspired mimir's vertical building approach",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	r.tools["list_domains"] = Tool{
		Name:        "list_domains",
		Description: "List all available domains with hints",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}

	// === Statistics ===
	r.tools["stats"] = Tool{
		Name:        "stats",
		Description: "Get database statistics",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func (r *ToolRegistry) Execute(name string, args map[string]interface{}) (interface{}, error) {
	// Check for handler-based tools first
	if tool, ok := r.tools[name]; ok && tool.Handler != nil {
		return tool.Handler(args)
	}

	switch name {
	// === Data Fetching ===
	case "fetch_clinical_trials":
		query, _ := args["query"].(string)
		limit, _ := args["limit"].(float64)
		f := fetch.NewClinicalTrialsFetcher()
		count, err := f.Fetch(r.db, query, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"fetched": count, "source": "ClinicalTrials.gov"}, nil

	case "fetch_pubmed":
		query, _ := args["query"].(string)
		limit, _ := args["limit"].(float64)
		f := fetch.NewPubMedFetcher()
		count, err := f.Fetch(r.db, query, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"fetched": count, "source": "PubMed"}, nil

	case "fetch_fda_approvals":
		query, _ := args["query"].(string)
		limit, _ := args["limit"].(float64)
		f := fetch.NewFDAFetcher()
		count, err := f.FetchApprovals(r.db, query, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"fetched": count, "source": "FDA Approvals"}, nil

	case "fetch_fda_adverse_events":
		query, _ := args["query"].(string)
		limit, _ := args["limit"].(float64)
		f := fetch.NewFDAFetcher()
		count, err := f.FetchAdverseEvents(r.db, query, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"fetched": count, "source": "FDA FAERS"}, nil

	case "fetch_sec_filings":
		query, _ := args["query"].(string)
		limit, _ := args["limit"].(float64)
		formTypes := []string{"8-K"}
		if ft, ok := args["form_types"].([]interface{}); ok {
			formTypes = nil
			for _, t := range ft {
				if s, ok := t.(string); ok {
					formTypes = append(formTypes, s)
				}
			}
		}
		f := fetch.NewSECFetcher()
		count, err := f.Fetch(r.db, query, formTypes, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"fetched": count, "source": "SEC EDGAR"}, nil

	case "fetch_research":
		domain, _ := args["domain"].(string)
		fetcher := fetch.NewResearchFetcher()
		result, err := fetcher.FetchDomain(r.db, domain)
		if err != nil {
			return nil, err
		}
		return result, nil

	// === Discovery ===
	case "discover_sources":
		topic, _ := args["topic"].(string)
		var keywords []string
		if kws, ok := args["keywords"].([]interface{}); ok {
			for _, k := range kws {
				if s, ok := k.(string); ok {
					keywords = append(keywords, s)
				}
			}
		}
		limit, _ := args["limit"].(float64)
		if limit == 0 {
			limit = 20
		}

		bootstrapper := discovery.NewDomainBootstrapper(topic, keywords, []string{"en", "ko"})
		discoverers := discovery.AllDiscoverers()
		srcs, err := bootstrapper.Bootstrap(discoverers, int(limit))
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"topic":   topic,
			"sources": srcs,
			"count":   len(srcs),
		}, nil

	case "discover_rss":
		domain, _ := args["domain"].(string)
		mode, _ := args["mode"].(string)
		finderMode := discovery.ModeFast
		if mode == "enhanced" {
			finderMode = discovery.ModeEnhanced
		}
		finder := discovery.NewRSSFinder(finderMode)
		result := finder.DiscoverDomain(context.Background(), domain)
		return result, nil

	case "authority_sources":
		topic, _ := args["topic"].(string)
		registry, err := sources.NewAuthorityRegistry()
		if err != nil {
			return nil, err
		}
		feeds := registry.GetByTopic(topic)
		return map[string]interface{}{
			"topic":   topic,
			"sources": feeds,
			"count":   len(feeds),
		}, nil

	// === Validation ===
	case "validate_feeds":
		var urls []string
		if u, ok := args["urls"].([]interface{}); ok {
			for _, url := range u {
				if s, ok := url.(string); ok {
					urls = append(urls, s)
				}
			}
		}
		v := validator.NewFastValidator(500)
		results := v.ValidateBatch(context.Background(), urls)
		return results, nil

	case "validate_sources":
		var urls []string
		if u, ok := args["urls"].([]interface{}); ok {
			for _, url := range u {
				if s, ok := url.(string); ok {
					urls = append(urls, s)
				}
			}
		}
		language, _ := args["language"].(string)
		if language == "" {
			language = "en"
		}
		v := validator.NewValidator()
		srcs := make([]validator.Source, len(urls))
		for i, url := range urls {
			srcs[i] = validator.Source{URL: url}
		}
		validated := v.ValidateBatch(srcs, []string{language}, 0.5)
		return validated, nil

	// === Ranking & Classification ===
	case "score_relevance":
		text, _ := args["text"].(string)
		var keywords []string
		if kws, ok := args["keywords"].([]interface{}); ok {
			for _, k := range kws {
				if s, ok := k.(string); ok {
					keywords = append(keywords, s)
				}
			}
		}
		classifier := ranking.NewRelevanceClassifier(keywords, "")
		score := classifier.Score(text)
		return map[string]interface{}{
			"score":    score,
			"relevant": score >= 0.5,
		}, nil

	case "detect_language":
		text, _ := args["text"].(string)
		result := lang.DetectWithConfidence(text)
		return map[string]interface{}{
			"language":   result.Lang,
			"confidence": result.Confidence,
			"script":     result.Script,
		}, nil

	// === Curation ===
	case "auto_curate":
		vertical, _ := args["vertical"].(string)
		topic, _ := args["topic"].(string)
		languages, _ := args["languages"].(string)
		if languages == "" {
			languages = "en,ko"
		}
		limit, _ := args["limit"].(float64)
		if limit == 0 {
			limit = 50
		}
		langList := []string{"en", "ko"}
		if languages != "" {
			langList = splitLanguages(languages)
		}
		ac, err := curator.NewAutoCurator(vertical, topic, langList, int(limit))
		if err != nil {
			return nil, err
		}
		result, err := ac.AutoCurate()
		if err != nil {
			return nil, err
		}
		return result, nil

	// === Bootstrap ===
	case "bootstrap_vertical":
		topic, _ := args["topic"].(string)
		languages, _ := args["languages"].(string)
		if languages == "" {
			languages = "en,ko"
		}
		depth, _ := args["depth"].(string)
		if depth == "" {
			depth = "standard"
		}
		config := &bootstrap.BootstrapConfig{
			Topic:     topic,
			Languages: []string{"en", "ko"},
			Depth:     depth,
		}
		bootstrapper := bootstrap.NewVerticalBootstrapper(config)
		result, err := bootstrapper.Run(context.Background())
		if err != nil {
			return nil, err
		}
		return result, nil

	// === Search ===
	case "search":
		query, _ := args["query"].(string)
		source, _ := args["source"].(string)
		limit, _ := args["limit"].(float64)
		if limit == 0 {
			limit = 20
		}
		return r.search(query, source, int(limit))

	case "expand_query":
		query, _ := args["query"].(string)
		var keywords []string
		if kws, ok := args["keywords"].([]interface{}); ok {
			for _, k := range kws {
				if s, ok := k.(string); ok {
					keywords = append(keywords, s)
				}
			}
		}
		expander := search.NewQueryExpander(keywords, query)
		expanded := expander.GetSearchStrings()
		return map[string]interface{}{
			"original": query,
			"expanded": expanded,
		}, nil

	// === Briefing ===
	case "generate_briefing":
		domain, _ := args["domain"].(string)
		if domain == "" {
			domain = "pharma"
		}
		language, _ := args["language"].(string)
		if language == "" {
			language = "ko"
		}
		useTTS, _ := args["tts"].(bool)
		return r.generateBriefing(domain, language, useTTS)

	case "send_briefing":
		title, _ := args["title"].(string)
		body, _ := args["body"].(string)
		if r.Delivery != nil {
			return nil, r.Delivery.Send(title, body, nil)
		}
		return map[string]string{"status": "no delivery channel configured"}, nil

	// === Domain Intelligence ===
	case "domain_hints":
		domain, _ := args["domain"].(string)
		guide, ok := hints.Get(domain)
		if !ok {
			return map[string]interface{}{
				"error":   "unknown domain",
				"domains": hints.List(),
			}, nil
		}
		return guide, nil

	case "domain_strategy":
		domain, _ := args["domain"].(string)
		strategy, ok := hints.GetStrategy(domain)
		if !ok {
			return map[string]interface{}{
				"error":   "no strategy for domain",
				"domains": hints.List(),
			}, nil
		}
		return strategy, nil

	case "architecture_refs":
		return map[string]interface{}{
			"references":     hints.GetArchitectureRefs(),
			"building_guide": hints.GetBuildingProcess(),
			"uniqueness":     hints.GetMimirUniqueness(),
		}, nil

	case "list_domains":
		domains := hints.List()
		var result []map[string]string
		for _, d := range domains {
			if guide, ok := hints.Get(d); ok {
				result = append(result, map[string]string{
					"domain":      d,
					"description": guide.Description,
				})
			}
		}
		return result, nil

	// === Statistics ===
	case "stats":
		stats, err := r.db.Stats()
		if err != nil {
			return nil, err
		}
		return stats, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (r *ToolRegistry) search(query, source string, limit int) (interface{}, error) {
	results := make(map[string]interface{})

	if source == "" || source == "all" || source == "clinical_trials" {
		rows, err := r.db.Query(`SELECT nct_id, title, phase, status FROM clinical_trials_fts WHERE clinical_trials_fts MATCH ? LIMIT ?`, query, limit)
		if err == nil {
			var trials []map[string]string
			for rows.Next() {
				var id, title, phase, status string
				rows.Scan(&id, &title, &phase, &status)
				trials = append(trials, map[string]string{"nct_id": id, "title": title, "phase": phase, "status": status})
			}
			rows.Close()
			results["clinical_trials"] = trials
		}
	}

	if source == "" || source == "all" || source == "pubmed" {
		rows, err := r.db.Query(`SELECT pmid, title, journal FROM pubmed_fts WHERE pubmed_fts MATCH ? LIMIT ?`, query, limit)
		if err == nil {
			var articles []map[string]string
			for rows.Next() {
				var pmid, title, journal string
				rows.Scan(&pmid, &title, &journal)
				articles = append(articles, map[string]string{"pmid": pmid, "title": title, "journal": journal})
			}
			rows.Close()
			results["pubmed"] = articles
		}
	}

	return results, nil
}

func (r *ToolRegistry) generateBriefing(domain, language string, useTTS bool) (interface{}, error) {
	stats, _ := r.db.Stats()

	var text string
	if language == "ko" {
		text = fmt.Sprintf("제약 인텔리전스 브리핑 - %s\n\n", time.Now().Format("2006-01-02"))
		text += fmt.Sprintf("현재 추적 중: 임상시험 %d건, 논문 %d건, FDA 승인 %d건\n\n",
			stats["clinical_trials"], stats["pubmed_articles"], stats["fda_approvals"])
	} else {
		text = fmt.Sprintf("Pharma Intelligence Briefing - %s\n\n", time.Now().Format("2006-01-02"))
		text += fmt.Sprintf("Currently tracking: %d clinical trials, %d papers, %d FDA approvals\n\n",
			stats["clinical_trials"], stats["pubmed_articles"], stats["fda_approvals"])
	}

	rows, err := r.db.Query(`SELECT title, sponsor, status FROM clinical_trials WHERE phase LIKE '%3%' ORDER BY created_at DESC LIMIT 5`)
	if err == nil {
		if language == "ko" {
			text += "주요 Phase 3 임상시험:\n"
		} else {
			text += "Key Phase 3 Trials:\n"
		}
		for rows.Next() {
			var title, sponsor, status string
			rows.Scan(&title, &sponsor, &status)
			text += fmt.Sprintf("- %s (%s) [%s]\n", title, sponsor, status)
		}
		rows.Close()
	}

	result := map[string]interface{}{
		"text":   text,
		"domain": domain,
		"lang":   language,
	}

	if useTTS && r.TTS != nil {
		voice := "ko-KR-SunHiNeural"
		if language == "en" {
			voice = "en-US-JennyNeural"
		}
		audio, err := r.TTS.Synthesize(text, voice)
		if err == nil && len(audio) > 0 {
			result["audio_size"] = len(audio)
			result["audio_engine"] = r.TTS.Name()
		}
	}

	return result, nil
}

func splitLanguages(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (r *ToolRegistry) ListTools() []Tool {
	var result []Tool
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *ToolRegistry) GetToolsJSON() ([]byte, error) {
	return json.MarshalIndent(r.ListTools(), "", "  ")
}
