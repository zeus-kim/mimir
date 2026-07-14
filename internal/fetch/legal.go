package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/db"
)

// ===========================================================================
// CourtListener API - Federal court opinions and cases
// https://www.courtlistener.com/api/rest/v3/
// ===========================================================================

type CourtListenerFetcher struct {
	BaseURL string
	Token   string // Optional API token for higher rate limits
}

func NewCourtListenerFetcher() *CourtListenerFetcher {
	return &CourtListenerFetcher{
		BaseURL: "https://www.courtlistener.com/api/rest/v3",
		Token:   os.Getenv("COURTLISTENER_API_TOKEN"),
	}
}

type courtListenerOpinionResponse struct {
	Count   int `json:"count"`
	Results []struct {
		ID               int    `json:"id"`
		AbsoluteURL      string `json:"absolute_url"`
		CaseName         string `json:"caseName"`
		CaseNameFull     string `json:"caseNameFull"`
		Court            string `json:"court"`
		CourtID          string `json:"court_id"`
		DateFiled        string `json:"date_filed"`
		Citation         []struct {
			Volume   int    `json:"volume"`
			Reporter string `json:"reporter"`
			Page     string `json:"page"`
		} `json:"citation"`
		Snippet string `json:"snippet"`
		Status  string `json:"status"`
	} `json:"results"`
}

func (f *CourtListenerFetcher) FetchCourtCases(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("order_by", "-date_filed")
	params.Set("page_size", fmt.Sprintf("%d", min(limit, 100)))

	req, err := http.NewRequest("GET", f.BaseURL+"/search/?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")
	if f.Token != "" {
		req.Header.Set("Authorization", "Token "+f.Token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("courtlistener API error: %s", resp.Status)
	}

	var result courtListenerOpinionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO legal_cases
		(case_id, case_name, case_name_full, court, court_id, date_filed, citation, snippet, status, source, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'courtlistener', ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO legal_cases_fts
		(case_id, case_name, court, snippet)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, c := range result.Results {
		var citations []string
		for _, cite := range c.Citation {
			citations = append(citations, fmt.Sprintf("%d %s %s", cite.Volume, cite.Reporter, cite.Page))
		}
		citationStr := strings.Join(citations, "; ")

		caseURL := "https://www.courtlistener.com" + c.AbsoluteURL
		caseID := fmt.Sprintf("cl_%d", c.ID)

		_, err := stmt.Exec(
			caseID, c.CaseName, c.CaseNameFull,
			c.Court, c.CourtID, c.DateFiled,
			citationStr, c.Snippet, c.Status, caseURL,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(caseID, c.CaseName, c.Court, c.Snippet)
		count++
	}

	return count, nil
}

// ===========================================================================
// Congress.gov API - Federal legislation
// https://api.congress.gov/v3/
// Requires API key from api.congress.gov
// ===========================================================================

type CongressFetcher struct {
	BaseURL string
	APIKey  string
}

func NewCongressFetcher() *CongressFetcher {
	return &CongressFetcher{
		BaseURL: "https://api.congress.gov/v3",
		APIKey:  os.Getenv("CONGRESS_API_KEY"),
	}
}

type congressBillResponse struct {
	Bills []struct {
		Congress       int    `json:"congress"`
		Type           string `json:"type"`
		Number         int    `json:"number"`
		Title          string `json:"title"`
		OriginChamber  string `json:"originChamber"`
		OriginChamberCode string `json:"originChamberCode"`
		LatestAction   struct {
			ActionDate string `json:"actionDate"`
			Text       string `json:"text"`
		} `json:"latestAction"`
		UpdateDate     string `json:"updateDate"`
		URL            string `json:"url"`
		PolicyArea     *struct {
			Name string `json:"name"`
		} `json:"policyArea"`
	} `json:"bills"`
	Pagination struct {
		Count int `json:"count"`
	} `json:"pagination"`
}

type congressBillDetailResponse struct {
	Bill struct {
		Congress      int    `json:"congress"`
		Type          string `json:"type"`
		Number        int    `json:"number"`
		Title         string `json:"title"`
		IntroducedDate string `json:"introducedDate"`
		OriginChamber string `json:"originChamber"`
		Sponsors      []struct {
			BioGuideID string `json:"bioguideId"`
			FullName   string `json:"fullName"`
			Party      string `json:"party"`
			State      string `json:"state"`
		} `json:"sponsors"`
		LatestAction struct {
			ActionDate string `json:"actionDate"`
			Text       string `json:"text"`
		} `json:"latestAction"`
		Cosponsors *struct {
			Count int `json:"count"`
		} `json:"cosponsors"`
		PolicyArea *struct {
			Name string `json:"name"`
		} `json:"policyArea"`
		Subjects *struct {
			Count int `json:"count"`
		} `json:"subjects"`
	} `json:"bill"`
}

func (f *CongressFetcher) FetchCongressBills(d *db.DB, query string, limit int) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("CONGRESS_API_KEY environment variable not set")
	}
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("api_key", f.APIKey)
	params.Set("limit", fmt.Sprintf("%d", min(limit, 250)))
	params.Set("sort", "updateDate desc")
	params.Set("format", "json")
	if query != "" {
		// Congress API uses 'q' for search - searches title, summary, etc.
		params.Set("q", query)
	}

	resp, err := http.Get(f.BaseURL + "/bill?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("congress.gov API error: %s", resp.Status)
	}

	var result congressBillResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO legislation
		(bill_id, bill_number, title, congress, chamber, sponsor, sponsor_party, sponsor_state,
		 policy_area, latest_action, latest_action_date, introduced_date, cosponsors_count,
		 jurisdiction, source, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'federal', 'congress.gov', ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO legislation_fts
		(bill_id, bill_number, title, sponsor, policy_area, latest_action)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, bill := range result.Bills {
		billNumber := fmt.Sprintf("%s %d", bill.Type, bill.Number)
		billID := fmt.Sprintf("us_%d_%s_%d", bill.Congress, strings.ToLower(bill.Type), bill.Number)
		billURL := fmt.Sprintf("https://www.congress.gov/bill/%dth-congress/%s-bill/%d",
			bill.Congress, strings.ToLower(bill.OriginChamber), bill.Number)

		var policyArea string
		if bill.PolicyArea != nil {
			policyArea = bill.PolicyArea.Name
		}

		// For full details, we'd need to fetch each bill individually
		// For now, use the list response data
		_, err := stmt.Exec(
			billID, billNumber, bill.Title, bill.Congress, bill.OriginChamber,
			"", "", "", // Sponsor details not in list response
			policyArea, bill.LatestAction.Text, bill.LatestAction.ActionDate,
			"", 0, billURL,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(billID, billNumber, bill.Title, "", policyArea, bill.LatestAction.Text)
		count++
	}

	return count, nil
}

// FetchBillDetails fetches full details for a specific bill
func (f *CongressFetcher) FetchBillDetails(d *db.DB, congress int, billType string, billNumber int) error {
	if f.APIKey == "" {
		return fmt.Errorf("CONGRESS_API_KEY environment variable not set")
	}

	params := url.Values{}
	params.Set("api_key", f.APIKey)
	params.Set("format", "json")

	url := fmt.Sprintf("%s/bill/%d/%s/%d?%s", f.BaseURL, congress, strings.ToLower(billType), billNumber, params.Encode())
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("congress.gov API error: %s", resp.Status)
	}

	var result congressBillDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	bill := result.Bill
	billID := fmt.Sprintf("us_%d_%s_%d", congress, strings.ToLower(billType), billNumber)
	billNumStr := fmt.Sprintf("%s %d", strings.ToUpper(billType), billNumber)
	billURL := fmt.Sprintf("https://www.congress.gov/bill/%dth-congress/%s-bill/%d",
		congress, strings.ToLower(bill.OriginChamber), billNumber)

	var sponsor, sponsorParty, sponsorState string
	if len(bill.Sponsors) > 0 {
		sponsor = bill.Sponsors[0].FullName
		sponsorParty = bill.Sponsors[0].Party
		sponsorState = bill.Sponsors[0].State
	}

	var policyArea string
	if bill.PolicyArea != nil {
		policyArea = bill.PolicyArea.Name
	}

	var cosponsorsCount int
	if bill.Cosponsors != nil {
		cosponsorsCount = bill.Cosponsors.Count
	}

	_, err = d.Exec(`INSERT OR REPLACE INTO legislation
		(bill_id, bill_number, title, congress, chamber, sponsor, sponsor_party, sponsor_state,
		 policy_area, latest_action, latest_action_date, introduced_date, cosponsors_count,
		 jurisdiction, source, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'federal', 'congress.gov', ?)`,
		billID, billNumStr, bill.Title, congress, bill.OriginChamber,
		sponsor, sponsorParty, sponsorState,
		policyArea, bill.LatestAction.Text, bill.LatestAction.ActionDate,
		bill.IntroducedDate, cosponsorsCount, billURL,
	)

	return err
}

// ===========================================================================
// Federal Register API - Federal rules, notices, and executive orders
// https://www.federalregister.gov/api/v1/
// No API key required
// ===========================================================================

type FederalRegisterFetcher struct {
	BaseURL string
}

func NewFederalRegisterFetcher() *FederalRegisterFetcher {
	return &FederalRegisterFetcher{
		BaseURL: "https://www.federalregister.gov/api/v1",
	}
}

type federalRegisterResponse struct {
	Count       int `json:"count"`
	TotalPages  int `json:"total_pages"`
	Results     []struct {
		DocumentNumber   string   `json:"document_number"`
		Title            string   `json:"title"`
		Abstract         string   `json:"abstract"`
		DocumentType     string   `json:"type"`
		Subtype          string   `json:"subtype"`
		Agencies         []struct {
			RawName string `json:"raw_name"`
			Name    string `json:"name"`
			ID      int    `json:"id"`
		} `json:"agencies"`
		PublicationDate  string   `json:"publication_date"`
		SigningDate      string   `json:"signing_date"`
		EffectiveOn      string   `json:"effective_on"`
		ExcerptFrom      string   `json:"body_html_url"`
		HTMLUrl          string   `json:"html_url"`
		PDFUrl           string   `json:"pdf_url"`
		Citation         string   `json:"citation"`
		StartPage        int      `json:"start_page"`
		EndPage          int      `json:"end_page"`
		Topics           []string `json:"topics"`
		Action           string   `json:"action"`
		Dates            string   `json:"dates"`
		SignificantStr   string   `json:"significant"`
		CFRReferences    []struct {
			Title   int    `json:"title"`
			Part    int    `json:"part"`
		} `json:"cfr_references"`
	} `json:"results"`
}

func (f *FederalRegisterFetcher) FetchFederalRegister(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	if query != "" {
		params.Set("conditions[term]", query)
	}
	params.Set("per_page", fmt.Sprintf("%d", min(limit, 1000)))
	params.Set("order", "newest")
	// Include all document types by default
	params.Set("conditions[type][]", "RULE")
	params.Add("conditions[type][]", "PRORULE")
	params.Add("conditions[type][]", "NOTICE")
	params.Add("conditions[type][]", "PRESDOCU")

	req, err := http.NewRequest("GET", f.BaseURL+"/documents.json?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("federal register API error: %s", resp.Status)
	}

	var result federalRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO federal_register
		(document_number, title, abstract, document_type, agencies, publication_date,
		 effective_date, citation, topics, cfr_references, url, pdf_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO federal_register_fts
		(document_number, title, abstract, agencies, topics)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, doc := range result.Results {
		var agencies []string
		for _, agency := range doc.Agencies {
			name := agency.Name
			if name == "" {
				name = agency.RawName
			}
			agencies = append(agencies, name)
		}
		agenciesStr := strings.Join(agencies, "; ")

		topicsStr := strings.Join(doc.Topics, "; ")

		var cfrRefs []string
		for _, ref := range doc.CFRReferences {
			cfrRefs = append(cfrRefs, fmt.Sprintf("%d CFR %d", ref.Title, ref.Part))
		}
		cfrStr := strings.Join(cfrRefs, "; ")

		_, err := stmt.Exec(
			doc.DocumentNumber, doc.Title, doc.Abstract, doc.DocumentType,
			agenciesStr, doc.PublicationDate, doc.EffectiveOn,
			doc.Citation, topicsStr, cfrStr, doc.HTMLUrl, doc.PDFUrl,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(doc.DocumentNumber, doc.Title, doc.Abstract, agenciesStr, topicsStr)
		count++
	}

	return count, nil
}

// FetchByDocumentType fetches specific document types (RULE, PRORULE, NOTICE, PRESDOCU)
func (f *FederalRegisterFetcher) FetchByDocumentType(d *db.DB, docType string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	params := url.Values{}
	params.Set("conditions[type][]", docType)
	params.Set("per_page", fmt.Sprintf("%d", min(limit, 1000)))
	params.Set("order", "newest")

	req, err := http.NewRequest("GET", f.BaseURL+"/documents.json?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("federal register API error: %s", resp.Status)
	}

	var result federalRegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO federal_register
		(document_number, title, abstract, document_type, agencies, publication_date,
		 effective_date, citation, topics, cfr_references, url, pdf_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO federal_register_fts
		(document_number, title, abstract, agencies, topics)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, doc := range result.Results {
		var agencies []string
		for _, agency := range doc.Agencies {
			name := agency.Name
			if name == "" {
				name = agency.RawName
			}
			agencies = append(agencies, name)
		}
		agenciesStr := strings.Join(agencies, "; ")

		topicsStr := strings.Join(doc.Topics, "; ")

		var cfrRefs []string
		for _, ref := range doc.CFRReferences {
			cfrRefs = append(cfrRefs, fmt.Sprintf("%d CFR %d", ref.Title, ref.Part))
		}
		cfrStr := strings.Join(cfrRefs, "; ")

		_, err := stmt.Exec(
			doc.DocumentNumber, doc.Title, doc.Abstract, doc.DocumentType,
			agenciesStr, doc.PublicationDate, doc.EffectiveOn,
			doc.Citation, topicsStr, cfrStr, doc.HTMLUrl, doc.PDFUrl,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(doc.DocumentNumber, doc.Title, doc.Abstract, agenciesStr, topicsStr)
		count++
	}

	return count, nil
}

// ===========================================================================
// Open States API - State legislation via GraphQL
// https://v3.openstates.org/graphql
// Requires API key from openstates.org
// ===========================================================================

type OpenStatesFetcher struct {
	GraphQLURL string
	APIKey     string
}

func NewOpenStatesFetcher() *OpenStatesFetcher {
	return &OpenStatesFetcher{
		GraphQLURL: "https://v3.openstates.org/graphql",
		APIKey:     os.Getenv("OPENSTATES_API_KEY"),
	}
}

type openStatesGraphQLResponse struct {
	Data struct {
		Bills struct {
			Edges []struct {
				Node struct {
					ID             string `json:"id"`
					Identifier     string `json:"identifier"`
					Title          string `json:"title"`
					Classification []string `json:"classification"`
					Subject        []string `json:"subject"`
					LatestAction   *struct {
						Description string `json:"description"`
						Date        string `json:"date"`
					} `json:"latestAction"`
					FromOrganization *struct {
						Name string `json:"name"`
					} `json:"fromOrganization"`
					LegislativeSession struct {
						Identifier string `json:"identifier"`
						Jurisdiction struct {
							Name string `json:"name"`
							ID   string `json:"id"`
						} `json:"jurisdiction"`
					} `json:"legislativeSession"`
					Sponsorships []struct {
						Name       string `json:"name"`
						EntityType string `json:"entityType"`
						Primary    bool   `json:"primary"`
					} `json:"sponsorships"`
					OpenStatesURL string `json:"openstatesUrl"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"bills"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (f *OpenStatesFetcher) FetchStateBills(d *db.DB, state string, query string, limit int) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("OPENSTATES_API_KEY environment variable not set")
	}
	if limit == 0 {
		limit = 50
	}

	// Construct GraphQL query
	graphqlQuery := `
query SearchBills($jurisdiction: String!, $query: String, $first: Int) {
  bills(jurisdiction: $jurisdiction, searchQuery: $query, first: $first) {
    edges {
      node {
        id
        identifier
        title
        classification
        subject
        latestAction {
          description
          date
        }
        fromOrganization {
          name
        }
        legislativeSession {
          identifier
          jurisdiction {
            name
            id
          }
        }
        sponsorships(first: 5) {
          name
          entityType
          primary
        }
        openstatesUrl
      }
    }
  }
}
`

	variables := map[string]interface{}{
		"jurisdiction": state,
		"first":        min(limit, 100),
	}
	if query != "" {
		variables["query"] = query
	}

	requestBody := map[string]interface{}{
		"query":     graphqlQuery,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", f.GraphQLURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", f.APIKey)
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("open states API error: %s", resp.Status)
	}

	var result openStatesGraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Errors) > 0 {
		return 0, fmt.Errorf("open states GraphQL error: %s", result.Errors[0].Message)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO legislation
		(bill_id, bill_number, title, congress, chamber, sponsor, sponsor_party, sponsor_state,
		 policy_area, latest_action, latest_action_date, introduced_date, cosponsors_count,
		 jurisdiction, source, url)
		VALUES (?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, '', 0, ?, 'openstates', ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO legislation_fts
		(bill_id, bill_number, title, sponsor, policy_area, latest_action)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, edge := range result.Data.Bills.Edges {
		bill := edge.Node

		var chamber string
		if bill.FromOrganization != nil {
			chamber = bill.FromOrganization.Name
		}

		var latestAction, latestActionDate string
		if bill.LatestAction != nil {
			latestAction = bill.LatestAction.Description
			latestActionDate = bill.LatestAction.Date
		}

		var primarySponsor string
		for _, sponsor := range bill.Sponsorships {
			if sponsor.Primary {
				primarySponsor = sponsor.Name
				break
			}
		}
		if primarySponsor == "" && len(bill.Sponsorships) > 0 {
			primarySponsor = bill.Sponsorships[0].Name
		}

		subjectsStr := strings.Join(bill.Subject, "; ")
		jurisdiction := bill.LegislativeSession.Jurisdiction.Name
		session := bill.LegislativeSession.Identifier

		_, err := stmt.Exec(
			bill.ID, bill.Identifier, bill.Title,
			session, chamber, primarySponsor, state,
			subjectsStr, latestAction, latestActionDate,
			jurisdiction, bill.OpenStatesURL,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(bill.ID, bill.Identifier, bill.Title, primarySponsor, subjectsStr, latestAction)
		count++
	}

	return count, nil
}

// FetchRecentStateBills fetches recent bills from a state without a search query
func (f *OpenStatesFetcher) FetchRecentStateBills(d *db.DB, state string, limit int) (int, error) {
	return f.FetchStateBills(d, state, "", limit)
}

// ===========================================================================
// LegalFetcher - Unified interface for all legal data fetchers
// ===========================================================================

type LegalFetcher struct {
	CourtListener   *CourtListenerFetcher
	Congress        *CongressFetcher
	FederalRegister *FederalRegisterFetcher
	OpenStates      *OpenStatesFetcher
}

func NewLegalFetcher() *LegalFetcher {
	return &LegalFetcher{
		CourtListener:   NewCourtListenerFetcher(),
		Congress:        NewCongressFetcher(),
		FederalRegister: NewFederalRegisterFetcher(),
		OpenStates:      NewOpenStatesFetcher(),
	}
}

// FetchAll fetches from all available legal sources
// Key-free: Federal Register, CourtListener (rate-limited without token)
// Key-required: Congress.gov, Open States (skipped if no key)
func (f *LegalFetcher) FetchAll(d *db.DB, query string, limit int) (map[string]int, error) {
	results := make(map[string]int)

	// Federal Register - NO KEY REQUIRED (priority)
	if count, err := f.FederalRegister.FetchFederalRegister(d, query, limit); err == nil {
		results["federal_register"] = count
	}

	// CourtListener - no key required but token increases rate limit
	if count, err := f.CourtListener.FetchCourtCases(d, query, limit); err == nil {
		results["court_cases"] = count
	}

	// Congress - requires API key (skip if not set)
	if f.Congress.APIKey != "" {
		if count, err := f.Congress.FetchCongressBills(d, query, limit); err == nil {
			results["congress_bills"] = count
		}
	}

	// Open States requires API key (skip if not set)
	// Note: Can't search all states at once, would need specific state

	return results, nil
}

// AvailableSources returns which legal sources are available
func (f *LegalFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"federal_register": true,                     // Always available
		"court_listener":   true,                     // Always available (rate-limited without token)
		"congress_gov":     f.Congress.APIKey != "",  // Requires CONGRESS_API_KEY
		"open_states":      f.OpenStates.APIKey != "",// Requires OPENSTATES_API_KEY
	}
}
