package tools

import (
	"fmt"

	"github.com/zeus-kim/mimir/internal/vertical"
)

// RegisterVerticalTools registers vertical management tools
func (r *ToolRegistry) RegisterVerticalTools() {
	r.Register(Tool{
		Name:        "create_vertical",
		Description: "Create a new vertical search engine instance",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]interface{}{"type": "string", "description": "Unique name for the vertical (e.g., 'pharma-2024')"},
				"domain":      map[string]interface{}{"type": "string", "description": "Domain preset (pharma, ai, legal, finance, politics, energy, food, tech) or 'custom'"},
				"description": map[string]interface{}{"type": "string", "description": "Description of this vertical"},
				"keywords":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Domain keywords for relevance scoring"},
				"languages":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "default": []string{"en"}},
			},
			"required": []string{"name", "domain"},
		},
		Handler: r.handleCreateVertical,
	})

	r.Register(Tool{
		Name:        "list_verticals",
		Description: "List all vertical search engine instances",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: r.handleListVerticals,
	})

	r.Register(Tool{
		Name:        "get_vertical",
		Description: "Get details about a specific vertical",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Vertical name"},
			},
			"required": []string{"name"},
		},
		Handler: r.handleGetVertical,
	})

	r.Register(Tool{
		Name:        "delete_vertical",
		Description: "Delete a vertical search engine instance",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string", "description": "Vertical name"},
				"confirm": map[string]interface{}{"type": "boolean", "description": "Must be true to confirm deletion"},
			},
			"required": []string{"name", "confirm"},
		},
		Handler: r.handleDeleteVertical,
	})

	r.Register(Tool{
		Name:        "vertical_stats",
		Description: "Get statistics for a vertical (documents, feeds, fit percent)",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Vertical name"},
			},
			"required": []string{"name"},
		},
		Handler: r.handleVerticalStats,
	})

	r.Register(Tool{
		Name:        "update_vertical_settings",
		Description: "Update settings for a vertical",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":            map[string]interface{}{"type": "string", "description": "Vertical name"},
				"min_fit_percent": map[string]interface{}{"type": "number", "description": "Minimum domain fit percentage (0-100)"},
				"max_feeds":       map[string]interface{}{"type": "integer", "description": "Maximum number of feeds"},
				"prune_threshold": map[string]interface{}{"type": "number", "description": "Prune feeds below this relevance (0-1)"},
				"fetch_interval":  map[string]interface{}{"type": "string", "description": "Fetch interval (e.g., '3h', '1d')"},
				"enabled_apis":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
			"required": []string{"name"},
		},
		Handler: r.handleUpdateVerticalSettings,
	})

	r.Register(Tool{
		Name:        "switch_vertical",
		Description: "Switch the current database to a different vertical",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string", "description": "Vertical name"},
			},
			"required": []string{"name"},
		},
		Handler: r.handleSwitchVertical,
	})
}

func (r *ToolRegistry) handleCreateVertical(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)
	domain, _ := args["domain"].(string)
	description, _ := args["description"].(string)

	var keywords []string
	if kw, ok := args["keywords"].([]interface{}); ok {
		for _, k := range kw {
			if s, ok := k.(string); ok {
				keywords = append(keywords, s)
			}
		}
	}

	var languages []string
	if langs, ok := args["languages"].([]interface{}); ok {
		for _, l := range langs {
			if s, ok := l.(string); ok {
				languages = append(languages, s)
			}
		}
	}
	if len(languages) == 0 {
		languages = []string{"en"}
	}

	manager := vertical.NewManager("")

	var v *vertical.Vertical
	var err error

	// Check if using preset or custom
	presets := vertical.GetDomainPresets()
	if _, isPreset := presets[domain]; isPreset {
		v, err = manager.CreateFromPreset(name, domain, languages)
		if err != nil {
			return nil, err
		}
		// Override description if provided
		if description != "" {
			v.Description = description
			manager.Update(v)
		}
	} else {
		// Custom domain
		if len(keywords) == 0 {
			return nil, fmt.Errorf("keywords required for custom domain")
		}
		v, err = manager.Create(name, domain, description, keywords, languages)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"success":     true,
		"name":        v.Name,
		"domain":      v.Domain,
		"description": v.Description,
		"keywords":    v.Keywords,
		"db_path":     v.DBPath,
		"created":     v.Created,
	}, nil
}

func (r *ToolRegistry) handleListVerticals(args map[string]interface{}) (interface{}, error) {
	manager := vertical.NewManager("")

	verticals, err := manager.List()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, len(verticals))
	for i, v := range verticals {
		stats, _ := manager.GetStats(v.Name)

		result[i] = map[string]interface{}{
			"name":        v.Name,
			"domain":      v.Domain,
			"description": v.Description,
			"languages":   v.Languages,
			"created":     v.Created,
			"updated":     v.Updated,
		}

		if stats != nil {
			result[i]["documents"] = stats.Documents
			result[i]["feeds"] = stats.Feeds
			result[i]["fit_percent"] = stats.FitPercent
		}
	}

	return map[string]interface{}{
		"verticals": result,
		"count":     len(result),
	}, nil
}

func (r *ToolRegistry) handleGetVertical(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)

	manager := vertical.NewManager("")

	v, err := manager.Get(name)
	if err != nil {
		return nil, err
	}

	stats, _ := manager.GetStats(name)

	result := map[string]interface{}{
		"name":        v.Name,
		"domain":      v.Domain,
		"description": v.Description,
		"keywords":    v.Keywords,
		"languages":   v.Languages,
		"db_path":     v.DBPath,
		"created":     v.Created,
		"updated":     v.Updated,
		"settings":    v.Settings,
	}

	if stats != nil {
		result["stats"] = stats
	}

	return result, nil
}

func (r *ToolRegistry) handleDeleteVertical(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)
	confirm, _ := args["confirm"].(bool)

	if !confirm {
		return nil, fmt.Errorf("deletion not confirmed: set confirm=true")
	}

	manager := vertical.NewManager("")

	if err := manager.Delete(name); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Vertical '%s' deleted", name),
	}, nil
}

func (r *ToolRegistry) handleVerticalStats(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)

	manager := vertical.NewManager("")

	v, err := manager.Get(name)
	if err != nil {
		return nil, err
	}

	stats, err := manager.GetStats(name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":        v.Name,
		"domain":      v.Domain,
		"documents":   stats.Documents,
		"feeds":       stats.Feeds,
		"fit_percent": stats.FitPercent,
		"last_fetch":  stats.LastFetch,
		"last_prune":  stats.LastPrune,
		"keywords":    v.Keywords,
	}, nil
}

func (r *ToolRegistry) handleUpdateVerticalSettings(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)

	manager := vertical.NewManager("")

	v, err := manager.Get(name)
	if err != nil {
		return nil, err
	}

	// Update settings
	if minFit, ok := args["min_fit_percent"].(float64); ok {
		v.Settings.MinFitPercent = minFit
	}
	if maxFeeds, ok := args["max_feeds"].(float64); ok {
		v.Settings.MaxFeeds = int(maxFeeds)
	}
	if pruneThresh, ok := args["prune_threshold"].(float64); ok {
		v.Settings.PruneThreshold = pruneThresh
	}
	if fetchInt, ok := args["fetch_interval"].(string); ok {
		v.Settings.FetchInterval = fetchInt
	}
	if apis, ok := args["enabled_apis"].([]interface{}); ok {
		v.Settings.EnabledAPIs = nil
		for _, a := range apis {
			if s, ok := a.(string); ok {
				v.Settings.EnabledAPIs = append(v.Settings.EnabledAPIs, s)
			}
		}
	}

	if err := manager.Update(v); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success":  true,
		"name":     v.Name,
		"settings": v.Settings,
	}, nil
}

func (r *ToolRegistry) handleSwitchVertical(args map[string]interface{}) (interface{}, error) {
	name, _ := args["name"].(string)

	manager := vertical.NewManager("")

	db, err := manager.OpenDB(name)
	if err != nil {
		return nil, err
	}

	// Close current DB and switch
	if r.db != nil {
		r.db.Close()
	}
	r.db = db

	return map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Switched to vertical '%s'", name),
		"db_path": db.Path,
	}, nil
}
