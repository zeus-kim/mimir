# Examples

## Claude Desktop Integration

### Installation

```bash
pip install mimir-vertical
```

### Configuration

Add to your Claude Desktop config file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "mimir": {
      "command": "mimir-vertical"
    }
  }
}
```

### Usage Examples

Once connected, you can use natural language to interact with mimir:

#### Create a Vertical
```
User: Create a pharma vertical called "oncology-research"
Claude: [calls create_vertical tool]
```

#### Fetch Data
```
User: Get the latest clinical trials about immunotherapy
Claude: [calls fetch_clinical_trials tool]
```

#### Check Status
```
User: Show me all my verticals
Claude: [calls list_verticals tool]
```

#### Search
```
User: Search for CRISPR in my pharma vertical
Claude: [calls search tool]
```

## Domain Presets

Available domain presets for quick vertical creation:

| Domain | Keywords | APIs |
|--------|----------|------|
| `pharma` | drug, clinical trial, FDA | ClinicalTrials.gov, PubMed, FDA |
| `ai` | machine learning, LLM, neural network | arXiv, Semantic Scholar, HuggingFace |
| `legal` | court, regulation, law | Federal Register, CourtListener |
| `finance` | stock, SEC, economics | Yahoo Finance, SEC EDGAR, FRED |
| `energy` | electricity, grid, renewable | ERCOT, EIA |
| `food` | nutrition, recipe, ingredient | Open Food Facts, TheMealDB |
| `politics` | congress, policy, election | Congress.gov, ProPublica |
| `tech` | startup, open source, API | GitHub, HackerNews, DevTo |

## Environment Variables

```bash
# Language (en, ko, ja, zh, es, fr, de)
export MIMIR_LANGUAGE=ko

# Log level
export MIMIR_LOG_LEVEL=debug

# Optional API keys for enhanced features
export FRED_API_KEY=your_key
export CONGRESS_API_KEY=your_key
```
