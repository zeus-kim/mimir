package tools

import (
	"context"
	"runtime"

	"github.com/zeus-kim/mimir/internal/health"
	"github.com/zeus-kim/mimir/internal/i18n"
	"github.com/zeus-kim/mimir/internal/metrics"
)

// RegisterSystemTools registers system-related tools
func (r *ToolRegistry) RegisterSystemTools() {
	r.Register(Tool{
		Name:        "health",
		Description: i18n.T("tool_health_desc"),
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			checker := health.Global()
			result := checker.Check(context.Background())
			return map[string]interface{}{
				"status":    result.Status,
				"timestamp": result.Timestamp,
				"version":   result.Version,
				"uptime":    result.Uptime,
				"checks":    result.Checks,
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "metrics",
		Description: i18n.T("tool_metrics_desc"),
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			m := metrics.Global()
			return m.Snapshot(), nil
		},
	})

	r.Register(Tool{
		Name:        "system_info",
		Description: i18n.T("tool_system_info_desc"),
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(args map[string]interface{}) (interface{}, error) {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			return map[string]interface{}{
				"go_version":    runtime.Version(),
				"os":            runtime.GOOS,
				"arch":          runtime.GOARCH,
				"num_cpu":       runtime.NumCPU(),
				"num_goroutine": runtime.NumGoroutine(),
				"memory": map[string]interface{}{
					"alloc_mb":       float64(memStats.Alloc) / 1024 / 1024,
					"total_alloc_mb": float64(memStats.TotalAlloc) / 1024 / 1024,
					"sys_mb":         float64(memStats.Sys) / 1024 / 1024,
					"num_gc":         memStats.NumGC,
				},
				"language": i18n.GetLanguage().String(),
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "api_status",
		Description: i18n.T("tool_api_status_desc"),
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: apiStatusHandler,
	})
}

func apiStatusHandler(args map[string]interface{}) (interface{}, error) {
	keyFree := map[string][]string{
		"pharma": {
			"ClinicalTrials.gov",
			"PubMed",
			"FDA Approvals",
			"FDA Adverse Events",
			"SEC EDGAR",
		},
		"ai_research": {
			"arXiv",
			"Semantic Scholar",
			"HuggingFace",
			"Papers With Code",
		},
		"legal": {
			"Federal Register",
			"CourtListener",
		},
		"finance": {
			"Yahoo Finance",
			"SEC EDGAR",
		},
		"food": {
			"Open Food Facts",
			"TheMealDB",
		},
		"energy": {
			"ERCOT",
		},
		"tech": {
			"GitHub Trending",
			"HackerNews",
			"DevTo",
		},
	}

	keyRequired := map[string]map[string]string{
		"finance": {
			"FRED":              "FRED_API_KEY",
			"Alpha Vantage":     "ALPHA_VANTAGE_KEY",
			"Financial Prep":    "FMP_API_KEY",
		},
		"energy": {
			"EIA":     "EIA_API_KEY",
			"ENTSO-E": "ENTSOE_API_KEY",
		},
		"politics": {
			"Congress.gov": "CONGRESS_API_KEY",
			"ProPublica":   "PROPUBLICA_API_KEY",
			"OpenSecrets":  "OPENSECRETS_API_KEY",
			"NewsAPI":      "NEWS_API_KEY",
		},
		"food": {
			"USDA":        "USDA_API_KEY",
			"Spoonacular": "SPOONACULAR_KEY",
			"Edamam":      "EDAMAM_APP_KEY",
		},
		"tts": {
			"OpenAI":     "OPENAI_API_KEY",
			"ElevenLabs": "ELEVEN_API_KEY",
		},
	}

	return map[string]interface{}{
		"key_free":     keyFree,
		"key_required": keyRequired,
		"note":         i18n.T("api_status_note"),
	}, nil
}
