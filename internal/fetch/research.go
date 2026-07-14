package fetch

import (
	"fmt"

	"github.com/zeus-kim/mimir/internal/db"
)

// QueryConfig defines a single fetch query with its parameters
type QueryConfig struct {
	Query string
	Max   int
}

// DomainConfig defines the research queries for a specific domain
type DomainConfig struct {
	Name           string
	ClinicalTrials []QueryConfig
	PubMed         []QueryConfig
}

// ResearchQueries contains predefined query sets for each research domain
var ResearchQueries = map[string]DomainConfig{
	"pharma": {
		Name: "Pharma/Clinical Trials",
		ClinicalTrials: []QueryConfig{
			{Query: "cancer immunotherapy", Max: 100},
			{Query: "phase 3 drug approval", Max: 100},
			{Query: "GLP-1 obesity diabetes", Max: 50},
			{Query: "alzheimer disease treatment", Max: 50},
			{Query: "gene therapy", Max: 50},
			{Query: "CAR-T cell therapy", Max: 50},
			{Query: "mRNA vaccine", Max: 50},
			{Query: "CRISPR gene editing", Max: 30},
		},
		PubMed: []QueryConfig{
			{Query: "clinical trial drug approval FDA", Max: 100},
			{Query: "immunotherapy cancer checkpoint inhibitor", Max: 100},
			{Query: "GLP-1 agonist obesity weight loss", Max: 50},
			{Query: "CAR-T cell therapy efficacy", Max: 50},
			{Query: "mRNA vaccine technology", Max: 50},
			{Query: "drug discovery machine learning AI", Max: 50},
			{Query: "biosimilar interchangeability", Max: 30},
		},
	},

	"biotech": {
		Name: "Biotech",
		ClinicalTrials: []QueryConfig{
			{Query: "gene therapy rare disease", Max: 50},
			{Query: "CRISPR cas9 clinical", Max: 50},
			{Query: "cell therapy regenerative", Max: 50},
			{Query: "biologic drug", Max: 50},
		},
		PubMed: []QueryConfig{
			{Query: "gene editing CRISPR therapy", Max: 50},
			{Query: "stem cell therapy clinical", Max: 50},
			{Query: "synthetic biology applications", Max: 30},
		},
	},

	"oncology": {
		Name: "Oncology",
		ClinicalTrials: []QueryConfig{
			{Query: "cancer immunotherapy checkpoint", Max: 100},
			{Query: "CAR-T therapy lymphoma leukemia", Max: 50},
			{Query: "targeted therapy oncology", Max: 50},
			{Query: "PD-1 PD-L1 inhibitor", Max: 50},
			{Query: "tumor microenvironment", Max: 30},
		},
		PubMed: []QueryConfig{
			{Query: "cancer immunotherapy review", Max: 100},
			{Query: "checkpoint inhibitor resistance", Max: 50},
			{Query: "liquid biopsy cancer detection", Max: 50},
			{Query: "tumor heterogeneity therapy", Max: 30},
		},
	},

	"neurology": {
		Name: "Neurology",
		ClinicalTrials: []QueryConfig{
			{Query: "alzheimer disease treatment", Max: 50},
			{Query: "parkinson disease therapy", Max: 50},
			{Query: "multiple sclerosis drug", Max: 30},
			{Query: "depression anxiety clinical trial", Max: 30},
		},
		PubMed: []QueryConfig{
			{Query: "alzheimer amyloid therapy", Max: 50},
			{Query: "neurodegeneration treatment", Max: 50},
			{Query: "psychedelic therapy depression", Max: 30},
		},
	},

	"metabolism": {
		Name: "Metabolism",
		ClinicalTrials: []QueryConfig{
			{Query: "GLP-1 diabetes obesity", Max: 50},
			{Query: "SGLT2 inhibitor", Max: 30},
			{Query: "NASH NAFLD liver", Max: 30},
			{Query: "weight loss drug obesity", Max: 30},
		},
		PubMed: []QueryConfig{
			{Query: "GLP-1 agonist weight loss mechanism", Max: 50},
			{Query: "metabolic syndrome treatment", Max: 30},
			{Query: "diabetes remission therapy", Max: 30},
		},
	},
}

// FetchResult holds the results from a fetch operation
type FetchResult struct {
	Domain         string   `json:"domain,omitempty"`
	ClinicalTrials int      `json:"clinical_trials"`
	PubMed         int      `json:"pubmed"`
	Domains        []string `json:"domains,omitempty"`
}

// ResearchFetcher combines multiple API sources for unified research data collection
type ResearchFetcher struct {
	trialsFetcher *ClinicalTrialsFetcher
	pubmedFetcher *PubMedFetcher
}

// NewResearchFetcher creates a new unified research fetcher
func NewResearchFetcher() *ResearchFetcher {
	return &ResearchFetcher{
		trialsFetcher: NewClinicalTrialsFetcher(),
		pubmedFetcher: NewPubMedFetcher(),
	}
}

// AvailableDomains returns a list of all predefined research domains
func (f *ResearchFetcher) AvailableDomains() []string {
	domains := make([]string, 0, len(ResearchQueries))
	for k := range ResearchQueries {
		domains = append(domains, k)
	}
	return domains
}

// FetchDomain fetches research data for a specific domain
func (f *ResearchFetcher) FetchDomain(d *db.DB, domain string) (*FetchResult, error) {
	config, ok := ResearchQueries[domain]
	if !ok {
		return nil, fmt.Errorf("unknown domain: %s (available: %v)", domain, f.AvailableDomains())
	}

	result := &FetchResult{
		Domain:         domain,
		ClinicalTrials: 0,
		PubMed:         0,
	}

	// Fetch clinical trials
	for _, q := range config.ClinicalTrials {
		count, err := f.trialsFetcher.Fetch(d, q.Query, q.Max)
		if err != nil {
			// Log error but continue with other queries
			fmt.Printf("clinical trials fetch error (%s): %v\n", q.Query, err)
			continue
		}
		result.ClinicalTrials += count
	}

	// Fetch PubMed articles
	for _, q := range config.PubMed {
		count, err := f.pubmedFetcher.Fetch(d, q.Query, q.Max)
		if err != nil {
			fmt.Printf("pubmed fetch error (%s): %v\n", q.Query, err)
			continue
		}
		result.PubMed += count
	}

	return result, nil
}

// FetchAll fetches research data for multiple domains
// If domains is nil or empty, fetches all available domains
func (f *ResearchFetcher) FetchAll(d *db.DB, domains []string) (*FetchResult, error) {
	if len(domains) == 0 {
		domains = f.AvailableDomains()
	}

	total := &FetchResult{
		ClinicalTrials: 0,
		PubMed:         0,
		Domains:        []string{},
	}

	for _, domain := range domains {
		result, err := f.FetchDomain(d, domain)
		if err != nil {
			fmt.Printf("domain fetch error (%s): %v\n", domain, err)
			continue
		}
		total.ClinicalTrials += result.ClinicalTrials
		total.PubMed += result.PubMed
		total.Domains = append(total.Domains, domain)
	}

	return total, nil
}

// CustomQuery defines a custom fetch query
type CustomQuery struct {
	Source string `json:"source"` // "clinical_trials" or "pubmed"
	Query  string `json:"query"`
	Max    int    `json:"max"`
}

// FetchCustom fetches data using custom query definitions
func (f *ResearchFetcher) FetchCustom(d *db.DB, queries []CustomQuery) (*FetchResult, error) {
	result := &FetchResult{
		ClinicalTrials: 0,
		PubMed:         0,
	}

	for _, q := range queries {
		maxResults := q.Max
		if maxResults == 0 {
			maxResults = 50
		}

		switch q.Source {
		case "clinical_trials":
			count, err := f.trialsFetcher.Fetch(d, q.Query, maxResults)
			if err != nil {
				fmt.Printf("clinical trials fetch error (%s): %v\n", q.Query, err)
				continue
			}
			result.ClinicalTrials += count

		case "pubmed":
			count, err := f.pubmedFetcher.Fetch(d, q.Query, maxResults)
			if err != nil {
				fmt.Printf("pubmed fetch error (%s): %v\n", q.Query, err)
				continue
			}
			result.PubMed += count
		}
	}

	return result, nil
}

// SourceStats holds statistics for a single data source
type SourceStats struct {
	Count       int    `json:"count"`
	LastUpdated string `json:"last_updated,omitempty"`
}

// ResearchStats holds combined statistics from all sources
type ResearchStats struct {
	ClinicalTrials SourceStats `json:"clinical_trials"`
	PubMed         SourceStats `json:"pubmed"`
}

// GetStats returns combined statistics from all research sources
func (f *ResearchFetcher) GetStats(d *db.DB) (*ResearchStats, error) {
	stats := &ResearchStats{}

	// Clinical trials stats
	var ctCount int
	row := d.QueryRow("SELECT COUNT(*) FROM clinical_trials")
	if err := row.Scan(&ctCount); err == nil {
		stats.ClinicalTrials.Count = ctCount
	}

	// PubMed stats
	var pmCount int
	row = d.QueryRow("SELECT COUNT(*) FROM pubmed_articles")
	if err := row.Scan(&pmCount); err == nil {
		stats.PubMed.Count = pmCount
	}

	return stats, nil
}

// ResearchSearchResult holds results from a unified research search
type ResearchSearchResult struct {
	ClinicalTrials []ClinicalTrialResult `json:"clinical_trials"`
	PubMed         []PubMedResult        `json:"pubmed"`
}

// ClinicalTrialResult represents a clinical trial search result
type ClinicalTrialResult struct {
	NCTID   string `json:"nct_id"`
	Title   string `json:"title"`
	Sponsor string `json:"sponsor"`
	Phase   string `json:"phase"`
	Status  string `json:"status"`
}

// PubMedResult represents a PubMed article search result
type PubMedResult struct {
	PMID     string `json:"pmid"`
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Journal  string `json:"journal"`
	Abstract string `json:"abstract,omitempty"`
}

// Search performs a unified search across all research sources
func (f *ResearchFetcher) Search(d *db.DB, query string, limit int) (*ResearchSearchResult, error) {
	if limit == 0 {
		limit = 20
	}

	result := &ResearchSearchResult{
		ClinicalTrials: []ClinicalTrialResult{},
		PubMed:         []PubMedResult{},
	}

	// Search clinical trials
	ctRows, err := d.Query(`
		SELECT nct_id, title, sponsor, phase, status
		FROM clinical_trials_fts
		WHERE clinical_trials_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer ctRows.Close()
		for ctRows.Next() {
			var ct ClinicalTrialResult
			if err := ctRows.Scan(&ct.NCTID, &ct.Title, &ct.Sponsor, &ct.Phase, &ct.Status); err == nil {
				result.ClinicalTrials = append(result.ClinicalTrials, ct)
			}
		}
	}

	// Search PubMed
	pmRows, err := d.Query(`
		SELECT pmid, title, authors, journal, abstract
		FROM pubmed_fts
		WHERE pubmed_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer pmRows.Close()
		for pmRows.Next() {
			var pm PubMedResult
			if err := pmRows.Scan(&pm.PMID, &pm.Title, &pm.Authors, &pm.Journal, &pm.Abstract); err == nil {
				result.PubMed = append(result.PubMed, pm)
			}
		}
	}

	return result, nil
}

// GetDomainConfig returns the configuration for a specific domain
func GetDomainConfig(domain string) (DomainConfig, bool) {
	config, ok := ResearchQueries[domain]
	return config, ok
}
