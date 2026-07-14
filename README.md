# mimir-mcp

**Vertical Search Engine Factory** - Create domain-specific search engines through natural language.

A standalone MCP (Model Context Protocol) server for building, managing, and operating vertical intelligence systems. Single binary, no external dependencies.

## Features

- **Domain-Specific Data Collection**: 16+ API fetchers across 8 domains
- **Vertical Management**: Create, configure, and manage multiple search verticals
- **ACHE/DISCO Algorithms**: Academic-grade relevance scoring and domain bootstrapping
- **Multi-Language Support**: EN, KO, JA, ZH, ES, FR, DE
- **Key-Free APIs**: Most core APIs work without API keys
- **39 MCP Tools**: Comprehensive toolset for vertical operations

## Quick Start

```bash
# Build
CGO_ENABLED=1 go build -tags "fts5" -o mimir-mcp ./cmd/mimir-mcp

# Run
./mimir-mcp

# With custom database
./mimir-mcp -db ~/.mine-pharma/lite.db

# With config file
./mimir-mcp -config config.json
```

## Architecture

```
mimir-mcp/
├── cmd/mimir-mcp/          # MCP server entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── db/                 # SQLite + FTS5
│   ├── delivery/           # Telegram, Slack, Discord, Email, ntfy
│   ├── discovery/          # DISCO-style source discovery
│   ├── fetch/              # Domain API fetchers (16 modules)
│   ├── hints/              # Domain exploration hints
│   ├── httpclient/         # HTTP client with retry/rate limiting
│   ├── i18n/               # Multi-language support
│   ├── logger/             # Structured logging
│   ├── ranking/            # ACHE-style TF-IDF + Bayesian ranking
│   ├── tools/              # MCP tool registry (39 tools)
│   ├── tts/                # Text-to-speech (edge-tts, macOS say)
│   ├── validator/          # Source validation
│   └── vertical/           # Vertical management
└── go.mod                  # Pure Go dependencies
```

## Domain APIs

### Key-Free APIs (No Registration Required)

| Domain | APIs | Data |
|--------|------|------|
| **Pharma** | ClinicalTrials.gov, PubMed, FDA, SEC | Clinical trials, papers, approvals, filings |
| **AI/ML** | arXiv, Semantic Scholar, HuggingFace, Papers With Code | Papers, models, code |
| **Legal** | Federal Register, CourtListener | Regulations, court cases |
| **Finance** | Yahoo Finance, SEC EDGAR | Stock quotes, company filings |
| **Food** | Open Food Facts, TheMealDB | Nutrition, recipes |
| **Energy** | ERCOT | Texas grid prices |

### Optional APIs (Free Key Required)

| Domain | APIs | Environment Variable |
|--------|------|---------------------|
| Finance | FRED, DART | `FRED_API_KEY`, `DART_API_KEY` |
| Legal | Congress.gov | `CONGRESS_API_KEY` |
| Energy | EIA, ENTSO-E | `EIA_API_KEY`, `ENTSOE_API_TOKEN` |
| Politics | ProPublica, OpenSecrets, 국회API | Various |
| Food | USDA, Spoonacular | `USDA_API_KEY`, `SPOONACULAR_API_KEY` |

## MCP Tools

### Vertical Management
| Tool | Description |
|------|-------------|
| `create_vertical` | Create a new vertical from domain preset or custom |
| `list_verticals` | List all vertical instances |
| `get_vertical` | Get vertical configuration and stats |
| `delete_vertical` | Delete a vertical |
| `vertical_stats` | Get documents, feeds, fit% |
| `update_vertical_settings` | Configure min_fit, max_feeds, etc. |
| `switch_vertical` | Switch active database |

### Domain Fetchers
| Tool | Description |
|------|-------------|
| `fetch_ai_research` | arXiv, Semantic Scholar, HuggingFace, Papers With Code |
| `fetch_legal` | Federal Register, CourtListener, Congress.gov |
| `fetch_finance` | Yahoo Finance, SEC, FRED |
| `fetch_energy` | ERCOT, EIA, ENTSO-E |
| `fetch_food` | Open Food Facts, TheMealDB, USDA |
| `fetch_politics` | ProPublica, OpenSecrets, Korean Assembly |
| `fetch_clinical_trials` | ClinicalTrials.gov |
| `fetch_pubmed` | PubMed articles |
| `fetch_fda_approvals` | FDA drug approvals |
| `fetch_sec_filings` | SEC EDGAR filings |

### Discovery & Ranking
| Tool | Description |
|------|-------------|
| `discover_sources` | DISCO-style seed expansion |
| `discover_rss` | Find RSS feeds from domain |
| `authority_sources` | Curated authoritative sources |
| `score_relevance` | ACHE-style TF-IDF scoring |
| `auto_curate` | Automated curation pipeline |
| `bootstrap_vertical` | Full vertical creation pipeline |

### Utilities
| Tool | Description |
|------|-------------|
| `api_status` | Check which APIs are available |
| `list_domain_presets` | Show domain templates |
| `domain_hints` | Get exploration strategies |
| `search` | Full-text search |
| `generate_briefing` | Create text/audio briefing |
| `send_briefing` | Send via Telegram/Slack/etc. |

## Domain Presets

```
pharma   - Clinical trials, drug approvals, biotech research
ai       - Machine learning, LLMs, AI research papers
legal    - Court cases, legislation, regulations
finance  - Markets, economics, company filings
politics - Political news, policy, elections
energy   - Energy markets, grid data, renewables
food     - Nutrition, recipes, food industry
tech     - Technology, startups, open source
```

## Configuration

### JSON Config File
```json
{
  "server": {
    "name": "mimir-mcp",
    "version": "1.0.0"
  },
  "database": {
    "path": "~/.mine-pharma/lite.db",
    "wal_mode": true
  },
  "tts": {
    "engine": "edge-tts",
    "voice": "en-US-AriaNeural"
  },
  "delivery": {
    "default": "telegram",
    "telegram": {
      "bot_token": "YOUR_TOKEN",
      "chat_id": "YOUR_CHAT_ID"
    }
  },
  "api_keys": {
    "fred": "",
    "congress": ""
  },
  "verticals": {
    "min_fit_percent": 50.0,
    "max_feeds": 200
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

### Environment Variables
```bash
# Database
export MIMIR_DB_PATH=~/.mine-pharma/lite.db

# API Keys
export FRED_API_KEY=xxx
export CONGRESS_API_KEY=xxx
export DART_API_KEY=xxx

# Delivery
export TELEGRAM_BOT_TOKEN=xxx
export TELEGRAM_CHAT_ID=xxx
```

## Claude Desktop Integration

```json
{
  "mcpServers": {
    "mimir": {
      "command": "/path/to/mimir-mcp",
      "args": ["-config", "/path/to/config.json"]
    }
  }
}
```

## Example Usage

### Create a Vertical
```
User: Create a pharma vertical called "oncology-2024"
Claude: [calls create_vertical with domain="pharma", name="oncology-2024"]
```

### Fetch Domain Data
```
User: Get the latest AI research on transformers
Claude: [calls fetch_ai_research with query="transformer"]
```

### Check API Status
```
User: Which APIs can I use without keys?
Claude: [calls api_status]
```

## References

- **ACHE**: Focused crawler with domain classifier (ViDA-NYU/ache)
- **DISCO**: Domain discovery and seed expansion (ViDA-NYU/domain-discovery-crawler)

## License

MIT
