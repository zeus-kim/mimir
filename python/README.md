# mimir-mcp

[![PyPI version](https://badge.fury.io/py/mimir-mcp.svg)](https://pypi.org/project/mimir-mcp/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Vertical Search Engine MCP Server** — Create domain-specific search engines through natural language.

Python wrapper for [mimir](https://github.com/zeus-kim/mimir) - automatically downloads and manages the Go binary.

## Installation

```bash
pip install mimir-mcp
```

## Usage

```bash
# Run MCP server
mimir-mcp

# CLI commands
mimir-mcp help
mimir-mcp version
mimir-mcp vertical list
mimir-mcp vertical create my-pharma --domain pharma
mimir-mcp health
mimir-mcp metrics

# Upgrade binary
mimir-mcp --upgrade

# Show binary path
mimir-mcp --path
```

## Claude Desktop Integration

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mimir": {
      "command": "mimir-mcp"
    }
  }
}
```

## Features

- **8 Domain Fetchers**: Pharma, AI/ML, Legal, Finance, Energy, Food, Politics, Tech
- **Key-Free APIs**: Most APIs work without registration
- **ACHE/DISCO Algorithms**: TF-IDF relevance scoring and Bayesian ranking
- **Vertical Management**: Create and manage multiple search engines
- **i18n**: 7 languages (EN, KO, JA, ZH, ES, FR, DE)

## Links

- [GitHub Repository](https://github.com/zeus-kim/mimir)
- [Documentation](https://github.com/zeus-kim/mimir#readme)

## License

MIT
