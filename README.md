# mimir

[English](README.md) | [한국어](README.ko.md)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-134%20passed-success)](https://github.com/zeus-kim/mimir)

**Vertical Search Engine Factory** — Create domain-specific search engines through natural language.

A standalone MCP (Model Context Protocol) server for building, managing, and operating vertical intelligence systems. Single binary, zero external dependencies.

## Features

- **8 Domain Fetchers**: Pharma, AI/ML, Legal, Finance, Energy, Food, Politics, Tech
- **Key-Free APIs**: Most APIs work without registration
- **ACHE/DISCO Algorithms**: Academic-grade TF-IDF relevance scoring and Bayesian ranking
- **Vertical Management**: Create, configure, and manage multiple search engines
- **i18n**: 7 languages (EN, KO, JA, ZH, ES, FR, DE)
- **Production Ready**: Health checks, metrics, structured logging, Docker support

## Installation

### pip (Recommended)

```bash
pip install mimir-vertical
```

### Go

```bash
go install github.com/zeus-kim/mimir/cmd/mimir-mcp@latest
```

### From Source

```bash
git clone https://github.com/zeus-kim/mimir.git
cd mimir
make build
```

### Docker

```bash
docker build -t mimir .
docker run -p 8080:8080 -v ~/.mimir:/data mimir
```

## Quick Start

```bash
# Build
make build

# Run MCP server
./bin/mimir-mcp

# CLI commands
./bin/mimir-mcp help
./bin/mimir-mcp vertical list
./bin/mimir-mcp vertical create my-pharma --domain pharma
./bin/mimir-mcp health
./bin/mimir-mcp metrics
```

## CLI Commands

```
mimir-mcp [command]

Commands:
  serve              Start MCP server (default)
  vertical, v        Manage verticals
    list             List all verticals
    create <name>    Create vertical (--domain, --keywords, --languages)
    show <name>      Show vertical details
    delete <name>    Delete vertical
    stats <name>     Show statistics
  health             Health check
  metrics            Show metrics
  config             Configuration management
  version            Show version
  help               Show help
```

## Domain APIs

### Key-Free (No Registration)

| Domain | APIs |
|--------|------|
| **Pharma** | ClinicalTrials.gov, PubMed, FDA, SEC EDGAR |
| **AI/ML** | arXiv, Semantic Scholar, HuggingFace, Papers With Code |
| **Legal** | Federal Register, CourtListener |
| **Finance** | Yahoo Finance, SEC EDGAR |
| **Food** | Open Food Facts, TheMealDB |
| **Energy** | ERCOT (Texas grid) |
| **Tech** | GitHub Trending, HackerNews, DevTo |

### Optional (Free API Key)

| Domain | APIs | Env Variable |
|--------|------|--------------|
| Finance | FRED | `FRED_API_KEY` |
| Energy | EIA, ENTSO-E | `EIA_API_KEY`, `ENTSOE_API_KEY` |
| Politics | Congress.gov, ProPublica | `CONGRESS_API_KEY`, `PROPUBLICA_API_KEY` |
| Food | USDA, Spoonacular | `USDA_API_KEY`, `SPOONACULAR_KEY` |

## MCP Tools

### Vertical Management
- `create_vertical` — Create from domain preset or custom
- `list_verticals` — List all instances
- `get_vertical` — Get config and stats
- `delete_vertical` — Delete vertical
- `vertical_stats` — Documents, feeds, fit%

### Domain Fetchers
- `fetch_ai_research` — arXiv, Semantic Scholar, HuggingFace
- `fetch_legal` — Federal Register, CourtListener
- `fetch_finance` — Yahoo Finance, SEC, FRED
- `fetch_energy` — ERCOT, EIA, ENTSO-E
- `fetch_food` — Open Food Facts, TheMealDB
- `fetch_politics` — Congress.gov, ProPublica
- `fetch_clinical_trials` — ClinicalTrials.gov
- `fetch_pubmed` — PubMed articles
- `fetch_fda_approvals` — FDA drug approvals

### System
- `health` — Health check
- `metrics` — Server metrics
- `api_status` — Available APIs

## Configuration

### Environment Variables

```bash
MIMIR_LANGUAGE=ko          # UI language
MIMIR_LOG_LEVEL=info       # debug|info|warn|error
MIMIR_DATA_DIR=~/.mimir    # Data directory
```

### Config File

```json
{
  "data_dir": "~/.mimir-verticals",
  "server": { "language": "en" },
  "logging": { "level": "info", "format": "json" },
  "verticals": { "min_fit_percent": 50.0, "max_feeds": 200 }
}
```

## Claude Desktop Integration

Add to `claude_desktop_config.json`:

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

## Architecture

```
mimir/
├── cmd/mimir-mcp/      # Entry point (CLI + MCP server)
├── internal/
│   ├── config/         # Configuration
│   ├── db/             # SQLite + FTS5
│   ├── fetch/          # Domain API fetchers
│   ├── health/         # Health checks
│   ├── httpclient/     # HTTP with retry/rate-limit
│   ├── i18n/           # Internationalization
│   ├── logger/         # Structured logging
│   ├── metrics/        # Metrics collection
│   ├── ranking/        # ACHE/DISCO algorithms
│   ├── tools/          # MCP tool registry
│   └── vertical/       # Vertical management
├── Dockerfile
├── Makefile
└── README.md
```

## Development

```bash
make test          # Run tests
make test-coverage # Coverage report
make lint          # Lint code
make build-all     # Build for all platforms
make docker        # Build Docker image
```

## References

- [ACHE](https://github.com/ViDA-NYU/ache) — Focused crawler with domain classifier
- [DISCO](https://github.com/ViDA-NYU/domain-discovery-crawler) — Domain discovery and seed expansion
- [MCP](https://modelcontextprotocol.io/) — Model Context Protocol

## License

MIT
