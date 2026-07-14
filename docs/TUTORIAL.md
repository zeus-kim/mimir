# Tutorial: Building a Vertical Search Engine

This tutorial walks you through creating a domain-specific search engine using mimir.

## Prerequisites

```bash
pip install mimir-vertical
```

## Step 1: Create a Vertical

Start by creating a vertical for your domain. We'll create one for pharmaceutical research.

```bash
mimir-vertical vertical create pharma-research --domain pharma
```

Or use the MCP tool:
```json
{
  "name": "create_vertical",
  "arguments": {
    "name": "pharma-research",
    "domain": "pharma",
    "keywords": ["drug", "clinical trial", "FDA approval"]
  }
}
```

## Step 2: Fetch Initial Data

Populate your vertical with domain-specific data:

```
User: Fetch the latest clinical trials about immunotherapy
Claude: [Fetches from ClinicalTrials.gov]

User: Get recent PubMed articles about CAR-T therapy
Claude: [Fetches from PubMed]

User: Find FDA drug approvals from the last month
Claude: [Fetches from FDA]
```

## Step 3: Discover More Sources

Use DISCO-style discovery to find additional sources:

```
User: Discover sources about oncology research
Claude: [Runs discover_sources, finds RSS feeds, academic sources, etc.]
```

## Step 4: Check Fit Percentage

The "fit percentage" measures how relevant your content is to your domain:

```bash
mimir-vertical vertical stats pharma-research
```

Output:
```
Documents: 1,234
Feeds: 45
Fit Percent: 72.5%
```

**Goal**: Aim for 50%+ fit percentage. If lower:
- Add more domain-specific keywords
- Prune low-relevance feeds
- Focus on authoritative sources

## Step 5: Search Your Vertical

```
User: Search for CRISPR gene editing trials
Claude: [Searches your vertical's FTS5 index]
```

## Step 6: Generate Briefings

Create a daily briefing from your vertical:

```
User: Generate a briefing of today's pharma news
Claude: [Creates a summary of recent documents]
```

## Advanced: Multiple Verticals

You can create multiple verticals for different domains:

```bash
mimir-vertical vertical create ai-papers --domain ai
mimir-vertical vertical create energy-markets --domain energy
mimir-vertical vertical create legal-updates --domain legal
```

Switch between them as needed:

```
User: Show me verticals
Claude: pharma-research, ai-papers, energy-markets, legal-updates

User: Search ai-papers for transformer architecture
Claude: [Searches the ai-papers vertical]
```

## Tips

1. **Start focused**: Begin with a narrow topic, then expand
2. **Quality over quantity**: Prefer authoritative sources
3. **Regular pruning**: Remove low-relevance feeds
4. **Monitor fit%**: Keep above 50% for best results
5. **Use presets**: Domain presets have optimized keywords

## Next Steps

- Explore [API Reference](API.md) for all available tools
- Check [examples/](../examples/) for configuration samples
- Set up automated fetching with cron or scheduled tasks
