package fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/user/mimir-mcp/internal/db"
)

type SECFetcher struct {
	SearchURL string
}

func NewSECFetcher() *SECFetcher {
	return &SECFetcher{
		SearchURL: "https://efts.sec.gov/LATEST/search-index",
	}
}

type secSearchResponse struct {
	Hits struct {
		Hits []struct {
			ID     string `json:"_id"`
			Source struct {
				CompanyName string `json:"display_names"`
				Tickers     string `json:"tickers"`
				FormType    string `json:"form"`
				FiledAt     string `json:"file_date"`
				Description string `json:"file_description"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func (f *SECFetcher) Fetch(d *db.DB, query string, formTypes []string, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}

	// SEC full-text search API
	searchURL := fmt.Sprintf("https://efts.sec.gov/LATEST/search-index?q=%s&dateRange=custom&startdt=2024-01-01&enddt=2026-12-31&forms=%s&from=0&size=%d",
		strings.ReplaceAll(query, " ", "%20"),
		strings.Join(formTypes, ","),
		limit,
	)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result secSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO sec_filings
		(accession_number, company_name, ticker, form_type, filed_date, description, url)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, hit := range result.Hits.Hits {
		s := hit.Source
		url := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s", hit.ID)

		_, err := stmt.Exec(
			hit.ID, s.CompanyName, s.Tickers,
			s.FormType, s.FiledAt, s.Description, url,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}
