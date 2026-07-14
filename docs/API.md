# API Reference

## MCP Tools

mimir exposes tools via the Model Context Protocol (MCP). Each tool can be called by Claude or any MCP-compatible client.

---

## Vertical Management

### create_vertical

Create a new vertical search engine.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| name | string | Yes | Unique name for the vertical |
| domain | string | No | Domain preset (pharma, ai, legal, finance, energy, food, politics, tech) |
| keywords | string[] | No | Custom keywords for relevance scoring |
| languages | string[] | No | Language codes (default: ["en"]) |
| description | string | No | Description of the vertical |

**Example:**
```json
{
  "name": "create_vertical",
  "arguments": {
    "name": "oncology-2024",
    "domain": "pharma",
    "keywords": ["cancer", "immunotherapy", "clinical trial"],
    "languages": ["en", "ko"]
  }
}
```

---

### list_verticals

List all vertical instances.

**Parameters:** None

**Response:**
```json
{
  "verticals": [
    {
      "name": "oncology-2024",
      "domain": "pharma",
      "documents": 1234,
      "feeds": 45,
      "fit_percent": 67.5
    }
  ]
}
```

---

### get_vertical

Get detailed information about a vertical.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| name | string | Yes | Vertical name |

---

### delete_vertical

Delete a vertical and all its data.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| name | string | Yes | Vertical name |

---

### vertical_stats

Get statistics for a vertical.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| name | string | Yes | Vertical name |

**Response:**
```json
{
  "documents": 1234,
  "feeds": 45,
  "fit_percent": 67.5,
  "last_fetch": "2024-01-15T10:30:00Z",
  "total_fetches": 120
}
```

---

## Data Fetching

### fetch_clinical_trials

Fetch clinical trials from ClinicalTrials.gov.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| query | string | Yes | Search query |
| limit | integer | No | Max results (default: 100) |

---

### fetch_pubmed

Fetch articles from PubMed.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| query | string | Yes | Search query |
| limit | integer | No | Max results (default: 50) |

---

### fetch_ai_research

Fetch AI/ML research from multiple sources.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| query | string | Yes | Search query |
| sources | string[] | No | Sources to use (arxiv, semantic_scholar, huggingface, papers_with_code) |
| limit | integer | No | Max results per source |

---

### fetch_legal

Fetch legal documents.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| query | string | Yes | Search query |
| sources | string[] | No | Sources (federal_register, court_listener) |

---

### fetch_finance

Fetch financial data.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| query | string | No | Search query or ticker |
| type | string | No | Data type (quotes, filings, economic) |

---

### fetch_energy

Fetch energy market data.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| source | string | No | Source (ercot, eia, entsoe) |
| zone | string | No | Grid zone (for ERCOT) |

---

## Discovery

### discover_sources

Discover sources for a topic using DISCO-style expansion.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| topic | string | Yes | Topic to discover |
| keywords | string[] | No | Seed keywords |
| limit | integer | No | Max sources |

---

### discover_rss

Find RSS feeds from a domain.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| domain | string | Yes | Domain to crawl |
| mode | string | No | fast or enhanced |

---

## Ranking

### score_relevance

Score text relevance using TF-IDF (ACHE style).

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| text | string | Yes | Text to score |
| keywords | string[] | Yes | Domain keywords |

**Response:**
```json
{
  "score": 0.85,
  "matched_keywords": ["cancer", "trial"]
}
```

---

## System

### health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h30m"
}
```

---

### metrics

Server metrics.

**Response:**
```json
{
  "total_requests": 1234,
  "total_documents": 5678,
  "api_calls": {"pubmed": 100, "arxiv": 50}
}
```

---

### api_status

Check available APIs.

**Response:**
```json
{
  "key_free": ["pubmed", "arxiv", "fda"],
  "key_required": {"fred": "FRED_API_KEY"}
}
```
