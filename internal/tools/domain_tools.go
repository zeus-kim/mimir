package tools

import (
	"fmt"

	"github.com/user/mimir-mcp/internal/db"
	"github.com/user/mimir-mcp/internal/fetch"
	"github.com/user/mimir-mcp/internal/i18n"
)

// RegisterDomainTools registers domain-specific fetch tools
func (r *ToolRegistry) RegisterDomainTools() {
	// Language management
	r.Register(Tool{
		Name:        "set_language",
		Description: "Set the interface language (en, ko, ja, zh, es, fr, de)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"language": map[string]interface{}{"type": "string", "enum": []string{"en", "ko", "ja", "zh", "es", "fr", "de"}},
			},
			"required": []string{"language"},
		},
		Handler: r.handleSetLanguage,
	})

	r.Register(Tool{
		Name:        "get_language",
		Description: "Get the current interface language and available languages",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: r.handleGetLanguage,
	})
	// AI/ML Research
	r.Register(Tool{
		Name:        "fetch_ai_research",
		Description: "Fetch AI/ML papers and models from arXiv, Semantic Scholar, HuggingFace, Papers With Code (no API key required)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":  map[string]interface{}{"type": "string", "description": "Search query (e.g., 'large language model', 'transformer')"},
				"source": map[string]interface{}{"type": "string", "enum": []string{"all", "arxiv", "semantic_scholar", "huggingface", "papers_with_code"}, "default": "all"},
				"limit":  map[string]interface{}{"type": "integer", "default": 50},
			},
			"required": []string{"query"},
		},
		Handler: r.handleFetchAIResearch,
	})

	// Legal
	r.Register(Tool{
		Name:        "fetch_legal",
		Description: "Fetch legal data from Federal Register, CourtListener, Congress.gov (Federal Register = no key required)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":  map[string]interface{}{"type": "string", "description": "Search query (e.g., 'antitrust', 'privacy regulation')"},
				"source": map[string]interface{}{"type": "string", "enum": []string{"all", "federal_register", "court_listener", "congress"}, "default": "all"},
				"limit":  map[string]interface{}{"type": "integer", "default": 50},
			},
			"required": []string{"query"},
		},
		Handler: r.handleFetchLegal,
	})

	// Finance
	r.Register(Tool{
		Name:        "fetch_finance",
		Description: "Fetch finance data from Yahoo Finance, SEC, FRED (Yahoo Finance = no key required)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symbols": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Stock symbols (e.g., ['AAPL', 'GOOGL'])"},
				"source":  map[string]interface{}{"type": "string", "enum": []string{"all", "yahoo", "sec", "fred"}, "default": "all"},
			},
		},
		Handler: r.handleFetchFinance,
	})

	// Energy
	r.Register(Tool{
		Name:        "fetch_energy",
		Description: "Fetch energy data from ERCOT, EIA, ENTSO-E (ERCOT = no key required)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source":  map[string]interface{}{"type": "string", "enum": []string{"all", "ercot", "eia", "entsoe"}, "default": "all"},
				"country": map[string]interface{}{"type": "string", "description": "Country code for ENTSO-E (DE, FR, GB, etc.)"},
			},
		},
		Handler: r.handleFetchEnergy,
	})

	// Food
	r.Register(Tool{
		Name:        "fetch_food",
		Description: "Fetch food/nutrition data from Open Food Facts, TheMealDB, USDA (Open Food Facts, TheMealDB = no key required)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":  map[string]interface{}{"type": "string", "description": "Search query (e.g., 'pasta', 'chicken')"},
				"source": map[string]interface{}{"type": "string", "enum": []string{"all", "open_food_facts", "the_meal_db", "usda"}, "default": "all"},
				"limit":  map[string]interface{}{"type": "integer", "default": 50},
			},
			"required": []string{"query"},
		},
		Handler: r.handleFetchFood,
	})

	// Politics (all require keys)
	r.Register(Tool{
		Name:        "fetch_politics",
		Description: "Fetch political data from ProPublica, OpenSecrets, Korean Assembly (all require API keys)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{"type": "string", "enum": []string{"all", "propublica", "opensecrets", "korean_assembly"}, "default": "all"},
				"query":  map[string]interface{}{"type": "string", "description": "Search query"},
				"limit":  map[string]interface{}{"type": "integer", "default": 50},
			},
		},
		Handler: r.handleFetchPolitics,
	})

	// API Status
	r.Register(Tool{
		Name:        "api_status",
		Description: "Check availability of all data APIs (which require keys, which are free)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"domain": map[string]interface{}{"type": "string", "enum": []string{"all", "ai", "legal", "finance", "energy", "food", "politics", "pharma"}, "default": "all"},
			},
		},
		Handler: r.handleAPIStatus,
	})

	// Domain Presets
	r.Register(Tool{
		Name:        "list_domain_presets",
		Description: "List available domain presets with their default keywords and APIs",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: r.handleListDomainPresets,
	})
}

func (r *ToolRegistry) handleSetLanguage(args map[string]interface{}) (interface{}, error) {
	langStr, _ := args["language"].(string)
	lang := i18n.ParseLanguage(langStr)
	i18n.SetLanguage(lang)

	return map[string]interface{}{
		"success":  true,
		"language": string(lang),
		"message":  i18n.Get().AppName + " - " + i18n.Get().AppDescription,
	}, nil
}

func (r *ToolRegistry) handleGetLanguage(args map[string]interface{}) (interface{}, error) {
	current := i18n.GetLanguage()
	supported := i18n.SupportedLanguages()

	langs := make([]string, len(supported))
	for i, l := range supported {
		langs[i] = string(l)
	}

	return map[string]interface{}{
		"current":   string(current),
		"supported": langs,
		"messages": map[string]string{
			"app_name":        i18n.Get().AppName,
			"app_description": i18n.Get().AppDescription,
			"domain_pharma":   i18n.Get().DomainPharma,
			"domain_ai":       i18n.Get().DomainAI,
			"domain_legal":    i18n.Get().DomainLegal,
			"domain_finance":  i18n.Get().DomainFinance,
		},
	}, nil
}

func (r *ToolRegistry) handleFetchAIResearch(args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	fetcher := fetch.NewAIResearchFetcher()
	result := map[string]interface{}{}

	if source == "all" {
		res, err := fetcher.FetchAll(r.db, query, limit)
		if err != nil {
			return nil, err
		}
		result["arxiv"] = res.Arxiv
		result["semantic_scholar"] = res.SemanticScholar
		result["huggingface"] = res.HuggingFace
		result["papers_with_code"] = res.PapersWithCode
		result["total"] = res.Arxiv + res.SemanticScholar + res.HuggingFace + res.PapersWithCode
	} else {
		var count int
		var err error
		switch source {
		case "arxiv":
			count, err = fetcher.FetchArxiv(r.db, query, limit)
		case "semantic_scholar":
			count, err = fetcher.FetchSemanticScholar(r.db, query, limit)
		case "huggingface":
			count, err = fetcher.FetchHuggingFaceModels(r.db, query, limit)
		case "papers_with_code":
			count, err = fetcher.FetchPapersWithCode(r.db, query, limit)
		}
		if err != nil {
			return nil, err
		}
		result[source] = count
	}

	result["message"] = i18n.T("fetch_completed", result["total"], "AI Research")
	return result, nil
}

func (r *ToolRegistry) handleFetchLegal(args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	fetcher := fetch.NewLegalFetcher()
	result := map[string]interface{}{}

	if source == "all" {
		res, err := fetcher.FetchAll(r.db, query, limit)
		if err != nil {
			return nil, err
		}
		total := 0
		for k, v := range res {
			result[k] = v
			total += v
		}
		result["total"] = total
	} else {
		var count int
		var err error
		switch source {
		case "federal_register":
			count, err = fetcher.FederalRegister.FetchFederalRegister(r.db, query, limit)
		case "court_listener":
			count, err = fetcher.CourtListener.FetchCourtCases(r.db, query, limit)
		case "congress":
			count, err = fetcher.Congress.FetchCongressBills(r.db, query, limit)
		}
		if err != nil {
			return nil, err
		}
		result[source] = count
	}

	result["available_sources"] = fetcher.AvailableSources()
	return result, nil
}

func (r *ToolRegistry) handleFetchFinance(args map[string]interface{}) (interface{}, error) {
	var symbols []string
	if s, ok := args["symbols"].([]interface{}); ok {
		for _, sym := range s {
			if str, ok := sym.(string); ok {
				symbols = append(symbols, str)
			}
		}
	}
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}

	fetcher := fetch.NewFinanceFetcher()
	result := map[string]interface{}{}

	if source == "all" {
		res, err := fetcher.FetchAll(r.db, symbols)
		if err != nil {
			return nil, err
		}
		result["stock_quotes"] = res.StockQuotes
		result["economic_data"] = res.EconomicData
		result["korean_disclosures"] = res.KoreanDisclosures
	} else {
		switch source {
		case "yahoo":
			count, err := fetcher.FetchStockQuotes(r.db, symbols)
			if err != nil {
				return nil, err
			}
			result["stock_quotes"] = count.StockQuotes
		}
	}

	result["available_sources"] = fetcher.AvailableSources()
	return result, nil
}

func (r *ToolRegistry) handleFetchEnergy(args map[string]interface{}) (interface{}, error) {
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}
	country, _ := args["country"].(string)

	fetcher := fetch.NewEnergyFetcher()
	result := map[string]interface{}{}

	if source == "all" {
		res, err := fetcher.FetchAll(r.db)
		if err != nil {
			return nil, err
		}
		result["ercot"] = res.ERCOT
		result["eia"] = res.EIA
		result["entsoe"] = res.ENTSOE
	} else {
		switch source {
		case "ercot":
			ercot := fetch.NewERCOTFetcher()
			count, err := ercot.FetchERCOTPrices(r.db)
			if err != nil {
				return nil, err
			}
			result["ercot"] = count
		case "entsoe":
			if country == "" {
				country = "DE"
			}
			entsoe := fetch.NewENTSOEFetcher()
			count, err := entsoe.FetchEuropeanGrid(r.db, country)
			if err != nil {
				return nil, err
			}
			result["entsoe"] = count
		}
	}

	result["available_sources"] = fetcher.AvailableSources()
	return result, nil
}

func (r *ToolRegistry) handleFetchFood(args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	fetcher := fetch.NewFoodFetcher()
	result := map[string]interface{}{}

	if source == "all" {
		res, err := fetcher.FetchAll(r.db, query, limit)
		if err != nil {
			return nil, err
		}
		result["open_food_facts"] = res.OpenFoodFacts
		result["the_meal_db"] = res.TheMealDB
		result["usda"] = res.USDA
		result["spoonacular"] = res.Spoonacular
		result["total"] = res.OpenFoodFacts + res.TheMealDB + res.USDA + res.Spoonacular
	} else {
		var count int
		var err error
		switch source {
		case "open_food_facts":
			count, err = fetcher.FetchOpenFoodProducts(r.db, query, limit)
		case "the_meal_db":
			count, err = fetcher.FetchRecipes(r.db, query, limit)
		case "usda":
			count, err = fetcher.FetchFoodNutrition(r.db, query, limit)
		}
		if err != nil {
			return nil, err
		}
		result[source] = count
	}

	result["available_sources"] = fetcher.AvailableSources()
	return result, nil
}

func (r *ToolRegistry) handleFetchPolitics(args map[string]interface{}) (interface{}, error) {
	source, _ := args["source"].(string)
	if source == "" {
		source = "all"
	}

	fetcher := fetch.NewPoliticsFetcher()
	available := fetcher.AvailableSources()

	// Check if any sources are available
	hasAny := false
	for _, v := range available {
		if v {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return map[string]interface{}{
			"error":             "All politics APIs require API keys",
			"available_sources": available,
			"required_keys": []string{
				"PROPUBLICA_API_KEY",
				"OPENSECRETS_API_KEY",
				"KOREAN_ASSEMBLY_API_KEY",
				"VOTESMART_API_KEY",
			},
		}, nil
	}

	result := map[string]interface{}{}
	if source == "all" {
		res, err := fetcher.FetchAll(r.db)
		if err != nil {
			return nil, err
		}
		result["congress_members"] = res.CongressMembers
		result["congress_bills"] = res.CongressBills
		result["congress_votes"] = res.CongressVotes
		result["korean_members"] = res.KoreanMembers
		result["korean_bills"] = res.KoreanBills
	}

	result["available_sources"] = available
	return result, nil
}

func (r *ToolRegistry) handleAPIStatus(args map[string]interface{}) (interface{}, error) {
	domain, _ := args["domain"].(string)
	if domain == "" {
		domain = "all"
	}

	status := map[string]interface{}{}

	// Key-free APIs
	keyFree := map[string][]string{
		"pharma":  {"clinical_trials", "pubmed", "fda", "sec"},
		"ai":      {"arxiv", "semantic_scholar", "huggingface", "papers_with_code"},
		"legal":   {"federal_register", "court_listener"},
		"finance": {"yahoo_finance", "sec"},
		"energy":  {"ercot"},
		"food":    {"open_food_facts", "the_meal_db"},
	}

	// Key-required APIs
	keyRequired := map[string]map[string]string{
		"pharma":   {},
		"ai":       {},
		"legal":    {"congress": "CONGRESS_API_KEY", "open_states": "OPENSTATES_API_KEY"},
		"finance":  {"fred": "FRED_API_KEY", "dart": "DART_API_KEY", "alpha_vantage": "ALPHAVANTAGE_API_KEY"},
		"energy":   {"eia": "EIA_API_KEY", "entsoe": "ENTSOE_API_TOKEN", "kpx": "KPX_API_KEY"},
		"food":     {"usda": "USDA_API_KEY", "spoonacular": "SPOONACULAR_API_KEY"},
		"politics": {"propublica": "PROPUBLICA_API_KEY", "opensecrets": "OPENSECRETS_API_KEY", "korean_assembly": "KOREAN_ASSEMBLY_API_KEY", "votesmart": "VOTESMART_API_KEY"},
	}

	domains := []string{"pharma", "ai", "legal", "finance", "energy", "food", "politics"}
	if domain != "all" {
		domains = []string{domain}
	}

	for _, d := range domains {
		domainStatus := map[string]interface{}{
			"key_free":     keyFree[d],
			"key_required": map[string]interface{}{},
		}

		for api, envVar := range keyRequired[d] {
			hasKey := getEnv(envVar) != ""
			domainStatus["key_required"].(map[string]interface{})[api] = map[string]interface{}{
				"env_var":   envVar,
				"available": hasKey,
			}
		}

		status[d] = domainStatus
	}

	return status, nil
}

func (r *ToolRegistry) handleListDomainPresets(args map[string]interface{}) (interface{}, error) {
	presets := map[string]interface{}{}

	domainPresets := map[string]map[string]interface{}{
		"pharma": {
			"name":        i18n.Get().DomainPharma,
			"description": "Clinical trials, drug approvals, biotech research",
			"keywords":    []string{"clinical trial", "FDA", "drug", "pharmaceutical", "biotech", "therapy", "approval"},
			"apis":        []string{"clinical_trials", "pubmed", "fda", "sec"},
			"key_free":    true,
		},
		"ai": {
			"name":        i18n.Get().DomainAI,
			"description": "Machine learning, deep learning, LLMs, AI research",
			"keywords":    []string{"machine learning", "deep learning", "neural network", "LLM", "transformer", "AI"},
			"apis":        []string{"arxiv", "semantic_scholar", "huggingface", "papers_with_code"},
			"key_free":    true,
		},
		"legal": {
			"name":        i18n.Get().DomainLegal,
			"description": "Court cases, legislation, regulations, legal news",
			"keywords":    []string{"court", "law", "regulation", "legislation", "ruling", "legal", "compliance"},
			"apis":        []string{"federal_register", "court_listener", "congress"},
			"key_free":    "partial",
		},
		"finance": {
			"name":        i18n.Get().DomainFinance,
			"description": "Markets, economics, company filings, financial analysis",
			"keywords":    []string{"market", "stock", "finance", "economy", "investment", "SEC", "earnings"},
			"apis":        []string{"yahoo_finance", "sec", "fred"},
			"key_free":    "partial",
		},
		"politics": {
			"name":        i18n.Get().DomainPolitics,
			"description": "Political news, policy, elections, government",
			"keywords":    []string{"politics", "election", "congress", "policy", "government", "legislation"},
			"apis":        []string{"propublica", "opensecrets"},
			"key_free":    false,
		},
		"energy": {
			"name":        i18n.Get().DomainEnergy,
			"description": "Energy markets, renewable energy, utilities, grid data",
			"keywords":    []string{"energy", "power", "renewable", "solar", "wind", "electricity", "grid"},
			"apis":        []string{"ercot", "eia", "entsoe"},
			"key_free":    "partial",
		},
		"food": {
			"name":        i18n.Get().DomainFood,
			"description": "Nutrition, recipes, food industry, restaurants",
			"keywords":    []string{"food", "nutrition", "recipe", "restaurant", "cooking", "diet"},
			"apis":        []string{"open_food_facts", "the_meal_db", "usda"},
			"key_free":    "partial",
		},
		"tech": {
			"name":        i18n.Get().DomainTech,
			"description": "Technology news, startups, open source, development",
			"keywords":    []string{"technology", "startup", "software", "open source", "programming", "cloud"},
			"apis":        []string{"arxiv", "huggingface"},
			"key_free":    true,
		},
	}

	for k, v := range domainPresets {
		presets[k] = v
	}

	return presets, nil
}

// Helper function
func getEnv(key string) string {
	return getEnvDefault(key, "")
}

func getEnvDefault(key, defaultVal string) string {
	if v := getEnvRaw(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvRaw(key string) string {
	// This is a simple implementation; in production, use os.Getenv
	// but we want to avoid import cycles
	return ""
}

// InitDomainTools ensures domain tables exist
func InitDomainTools(d *db.DB) error {
	// Ensure AI tables
	if err := fetch.EnsureAISchema(d); err != nil {
		return fmt.Errorf("AI schema: %w", err)
	}

	// Ensure Politics tables
	if err := fetch.EnsurePoliticsSchema(d); err != nil {
		return fmt.Errorf("Politics schema: %w", err)
	}

	// Ensure Energy tables
	if err := fetch.EnsureEnergySchema(d); err != nil {
		return fmt.Errorf("Energy schema: %w", err)
	}

	return nil
}
