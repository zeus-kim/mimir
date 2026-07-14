package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/zeus-kim/mimir/internal/config"
	"github.com/zeus-kim/mimir/internal/db"
	"github.com/zeus-kim/mimir/internal/delivery"
	"github.com/zeus-kim/mimir/internal/i18n"
	"github.com/zeus-kim/mimir/internal/logger"
	"github.com/zeus-kim/mimir/internal/tools"
	"github.com/zeus-kim/mimir/internal/tts"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	Commit    = "unknown"
)

type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	registry *tools.ToolRegistry
	config   *config.Config
	log      *logger.Logger
}

func main() {
	// Handle CLI commands first
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if cmd != "" && cmd[0] != '-' && cmd != "serve" {
			cfg, _ := config.Load("")
			cli := NewCLI(cfg)
			if err := cli.Run(os.Args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	dbPath := flag.String("db", "", "Database path")
	language := flag.String("lang", "", "Language (en, ko, ja, zh, es, fr, de)")
	ttsEngine := flag.String("tts", "", "TTS engine (edge-tts, say, none)")
	configPath := flag.String("config", "", "Config file path")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mimir-mcp %s (built %s, commit %s)\n", Version, BuildTime, Commit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override with command-line flags
	if *dbPath != "" {
		cfg.Database.Path = *dbPath
	}
	if *language != "" {
		cfg.Server.Language = *language
	}
	if *ttsEngine != "" {
		cfg.TTS.Engine = *ttsEngine
	}

	// Initialize i18n
	lang := i18n.ParseLanguage(cfg.Server.Language)
	i18n.SetLanguage(lang)

	// Initialize logger
	log := logger.New(
		"mimir",
		logger.ParseLevel(cfg.Logging.Level),
		cfg.Logging.Format,
		os.Stderr,
	)
	logger.SetDefault(log)

	log.Info("Starting mimir-mcp v%s (lang=%s)", Version, lang)

	// Open database
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		log.Error("Failed to open database: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	// Ensure schema
	if err := database.EnsureSchema(); err != nil {
		log.Error("Failed to ensure schema: %v", err)
		os.Exit(1)
	}

	// Create registry
	registry := tools.NewRegistry(database)
	registry.TTS = tts.GetEngine(cfg.TTS.Engine)

	// Configure delivery channel
	registry.Delivery = configureDelivery(cfg)

	server := &Server{
		registry: registry,
		config:   cfg,
		log:      log,
	}

	server.run()
}

func configureDelivery(cfg *config.Config) delivery.Channel {
	dc := cfg.Delivery

	switch dc.Default {
	case "telegram":
		if dc.Telegram.BotToken != "" && dc.Telegram.ChatID != "" {
			return &delivery.TelegramChannel{
				BotToken: dc.Telegram.BotToken,
				ChatID:   dc.Telegram.ChatID,
			}
		}
	case "slack":
		if dc.Slack.WebhookURL != "" {
			return &delivery.SlackChannel{
				WebhookURL: dc.Slack.WebhookURL,
			}
		}
	case "discord":
		if dc.Discord.WebhookURL != "" {
			return &delivery.DiscordChannel{
				WebhookURL: dc.Discord.WebhookURL,
			}
		}
	case "ntfy":
		server := dc.Ntfy.Server
		if server == "" {
			server = "https://ntfy.sh"
		}
		if dc.Ntfy.Topic != "" {
			return &delivery.NtfyChannel{
				ServerURL: server,
				Topic:     dc.Ntfy.Topic,
			}
		}
	}

	return nil
}

func (s *Server) run() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg MCPMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleMessage(&msg)
	}
}

func (s *Server) handleMessage(msg *MCPMessage) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "tools/list":
		s.handleToolsList(msg)
	case "tools/call":
		s.handleToolsCall(msg)
	default:
		if msg.ID != nil {
			s.sendError(msg.ID, -32601, "Method not found")
		}
	}
}

func (s *Server) handleInitialize(msg *MCPMessage) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{"listChanged": true},
		},
		"serverInfo": map[string]string{
			"name":    s.config.Server.Name,
			"version": Version,
		},
	}
	s.sendResult(msg.ID, result)
}

func (s *Server) handleToolsList(msg *MCPMessage) {
	toolList := s.registry.ListTools()

	var mcpTools []map[string]interface{}
	for _, t := range toolList {
		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}

	s.sendResult(msg.ID, map[string]interface{}{"tools": mcpTools})
}

func (s *Server) handleToolsCall(msg *MCPMessage) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.sendError(msg.ID, -32602, "Invalid params")
		return
	}

	s.log.Debug("Tool call: %s", params.Name)

	result, err := s.registry.Execute(params.Name, params.Arguments)
	if err != nil {
		s.log.Error("Tool error: %s - %v", params.Name, err)
		s.sendResult(msg.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
		return
	}

	resultJSON, _ := json.Marshal(result)
	s.sendResult(msg.ID, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(resultJSON)},
		},
	})
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(&msg)
}

func (s *Server) sendError(id interface{}, code int, message string) {
	msg := MCPMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPError{Code: code, Message: message},
	}
	s.send(&msg)
}

func (s *Server) send(msg *MCPMessage) {
	data, _ := json.Marshal(msg)
	fmt.Println(string(data))
}
