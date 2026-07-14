package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/zeus-kim/mimir/internal/config"
	"github.com/zeus-kim/mimir/internal/health"
	"github.com/zeus-kim/mimir/internal/i18n"
	"github.com/zeus-kim/mimir/internal/logger"
	"github.com/zeus-kim/mimir/internal/metrics"
	"github.com/zeus-kim/mimir/internal/vertical"
)

// CLI subcommands
type CLI struct {
	cfg      *config.Config
	log      *logger.Logger
	vertMgr  *vertical.Manager
}

// NewCLI creates a new CLI instance
func NewCLI(cfg *config.Config) *CLI {
	return &CLI{
		cfg:     cfg,
		log:     logger.Default(),
		vertMgr: vertical.NewManager(cfg.DataDir),
	}
}

// Run executes CLI commands
func (c *CLI) Run(args []string) error {
	if len(args) < 1 {
		return c.showHelp()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "version", "-v", "--version":
		return c.showVersion()
	case "help", "-h", "--help":
		return c.showHelp()
	case "vertical", "v":
		return c.verticalCmd(cmdArgs)
	case "health":
		return c.healthCmd()
	case "metrics":
		return c.metricsCmd()
	case "config":
		return c.configCmd(cmdArgs)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (c *CLI) showVersion() error {
	fmt.Printf("mimir-mcp %s\n", Version)
	fmt.Printf("  Build: %s\n", BuildTime)
	fmt.Printf("  Commit: %s\n", Commit)
	return nil
}

func (c *CLI) showHelp() error {
	help := `mimir-mcp - Vertical Search Engine MCP Server

Usage: mimir-mcp [command] [options]

Commands:
  serve              Start MCP server (default)
  vertical, v        Manage verticals
  health             Show health status
  metrics            Show metrics
  config             Show/manage configuration
  version            Show version
  help               Show this help

Vertical Commands:
  vertical list                  List all verticals
  vertical create <name> [opts]  Create a new vertical
  vertical show <name>           Show vertical details
  vertical delete <name>         Delete a vertical
  vertical stats <name>          Show vertical statistics

Create Options:
  --domain <domain>     Domain preset (pharma|ai|legal|finance|energy|food|politics|tech)
  --keywords <k1,k2>    Comma-separated keywords
  --languages <l1,l2>   Comma-separated language codes

Config Commands:
  config show           Show current configuration
  config path           Show config file path
  config init           Create default config file

Environment Variables:
  MIMIR_CONFIG          Config file path
  MIMIR_LANGUAGE        UI language (en|ko|ja|zh|es|fr|de)
  MIMIR_LOG_LEVEL       Log level (debug|info|warn|error)
  MIMIR_DATA_DIR        Data directory

Examples:
  mimir-mcp serve
  mimir-mcp vertical create pharma-v1 --domain pharma
  mimir-mcp vertical list
  mimir-mcp health
`
	fmt.Print(help)
	return nil
}

func (c *CLI) verticalCmd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vertical subcommand required: list|create|show|delete|stats")
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "list", "ls":
		return c.verticalList()
	case "create", "new":
		return c.verticalCreate(subArgs)
	case "show", "get":
		return c.verticalShow(subArgs)
	case "delete", "rm":
		return c.verticalDelete(subArgs)
	case "stats":
		return c.verticalStats(subArgs)
	default:
		return fmt.Errorf("unknown vertical subcommand: %s", subcmd)
	}
}

func (c *CLI) verticalList() error {
	verts, err := c.vertMgr.List()
	if err != nil {
		return err
	}

	if len(verts) == 0 {
		fmt.Println("No verticals found.")
		return nil
	}

	fmt.Printf("%-20s %-12s %-30s %s\n", "NAME", "DOMAIN", "KEYWORDS", "CREATED")
	fmt.Println(strings.Repeat("-", 80))

	for _, v := range verts {
		keywords := strings.Join(v.Keywords, ", ")
		if len(keywords) > 28 {
			keywords = keywords[:25] + "..."
		}
		fmt.Printf("%-20s %-12s %-30s %s\n",
			v.Name, v.Domain, keywords, v.Created.Format("2006-01-02"))
	}

	return nil
}

func (c *CLI) verticalCreate(args []string) error {
	fs := flag.NewFlagSet("vertical create", flag.ExitOnError)
	domain := fs.String("domain", "", "Domain preset")
	keywords := fs.String("keywords", "", "Comma-separated keywords")
	languages := fs.String("languages", "en", "Comma-separated language codes")
	desc := fs.String("description", "", "Vertical description")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("vertical name required")
	}
	name := fs.Arg(0)

	var kws []string
	if *keywords != "" {
		kws = strings.Split(*keywords, ",")
		for i := range kws {
			kws[i] = strings.TrimSpace(kws[i])
		}
	}

	langs := strings.Split(*languages, ",")
	for i := range langs {
		langs[i] = strings.TrimSpace(langs[i])
	}

	var v *vertical.Vertical
	var err error

	if *domain != "" {
		v, err = c.vertMgr.CreateFromPreset(name, *domain, langs)
	} else {
		v, err = c.vertMgr.Create(name, *domain, *desc, kws, langs)
	}

	if err != nil {
		return err
	}

	fmt.Println(i18n.T("vertical_created", v.Name))
	fmt.Printf("  Domain: %s\n", v.Domain)
	fmt.Printf("  Keywords: %s\n", strings.Join(v.Keywords, ", "))
	fmt.Printf("  DB: %s\n", v.DBPath)

	return nil
}

func (c *CLI) verticalShow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vertical name required")
	}

	v, err := c.vertMgr.Get(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Name:        %s\n", v.Name)
	fmt.Printf("Domain:      %s\n", v.Domain)
	fmt.Printf("Description: %s\n", v.Description)
	fmt.Printf("Keywords:    %s\n", strings.Join(v.Keywords, ", "))
	fmt.Printf("Languages:   %s\n", strings.Join(v.Languages, ", "))
	fmt.Printf("DB Path:     %s\n", v.DBPath)
	fmt.Printf("Created:     %s\n", v.Created.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", v.Updated.Format("2006-01-02 15:04:05"))
	fmt.Printf("\nSettings:\n")
	fmt.Printf("  Min Fit:       %.1f%%\n", v.Settings.MinFitPercent)
	fmt.Printf("  Max Feeds:     %d\n", v.Settings.MaxFeeds)
	fmt.Printf("  Fetch Interval: %s\n", v.Settings.FetchInterval)
	fmt.Printf("  APIs:          %s\n", strings.Join(v.Settings.EnabledAPIs, ", "))

	return nil
}

func (c *CLI) verticalDelete(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vertical name required")
	}

	name := args[0]
	if err := c.vertMgr.Delete(name); err != nil {
		return err
	}

	fmt.Println(i18n.T("vertical_deleted", name))
	return nil
}

func (c *CLI) verticalStats(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("vertical name required")
	}

	stats, err := c.vertMgr.GetStats(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Vertical: %s\n", args[0])
	fmt.Printf("\nStatistics:\n")
	fmt.Printf("  Documents:    %d\n", stats.Documents)
	fmt.Printf("  Feeds:        %d\n", stats.Feeds)
	fmt.Printf("  Fit Percent:  %.1f%%\n", stats.FitPercent)
	fmt.Printf("  Last Fetch:   %s\n", stats.LastFetch.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Total Fetches: %d\n", stats.TotalFetches)
	fmt.Printf("  Total Pruned: %d\n", stats.TotalPruned)

	return nil
}

func (c *CLI) healthCmd() error {
	checker := health.NewChecker(Version)
	result := checker.Check(nil)

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func (c *CLI) metricsCmd() error {
	m := metrics.Global()
	data, _ := json.MarshalIndent(m.Snapshot(), "", "  ")
	fmt.Println(string(data))
	return nil
}

func (c *CLI) configCmd(args []string) error {
	if len(args) < 1 {
		return c.configShow()
	}

	switch args[0] {
	case "show":
		return c.configShow()
	case "path":
		fmt.Println(c.cfg.ConfigPath())
		return nil
	case "init":
		return c.configInit()
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func (c *CLI) configShow() error {
	data, _ := json.MarshalIndent(c.cfg, "", "  ")
	fmt.Println(string(data))
	return nil
}

func (c *CLI) configInit() error {
	cfg := config.Default()
	path := cfg.ConfigPath()

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	dir := strings.TrimSuffix(path, "/config.json")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	fmt.Printf("Config file created: %s\n", path)
	return nil
}
