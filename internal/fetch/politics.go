package fetch

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/zeus-kim/mimir/internal/db"
)

// =====================
// Schema for Politics Domain
// =====================

// PoliticsSchemas returns the SQL statements to create politics-related tables
func PoliticsSchemas() []string {
	return []string{
		// US Congress Members (ProPublica)
		`CREATE TABLE IF NOT EXISTS congress_members (
			member_id TEXT PRIMARY KEY,
			first_name TEXT,
			last_name TEXT,
			party TEXT,
			state TEXT,
			chamber TEXT,
			title TEXT,
			twitter_account TEXT,
			facebook_account TEXT,
			url TEXT,
			next_election TEXT,
			total_votes INTEGER,
			missed_votes_pct REAL,
			votes_with_party_pct REAL,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS congress_members_fts USING fts5(
			member_id, first_name, last_name, party, state, chamber
		)`,

		// Congress Bills (ProPublica)
		`CREATE TABLE IF NOT EXISTS congress_bills (
			bill_id TEXT PRIMARY KEY,
			bill_slug TEXT,
			bill_type TEXT,
			number TEXT,
			title TEXT,
			short_title TEXT,
			sponsor_id TEXT,
			sponsor_name TEXT,
			sponsor_party TEXT,
			sponsor_state TEXT,
			committees TEXT,
			primary_subject TEXT,
			introduced_date TEXT,
			latest_major_action TEXT,
			latest_major_action_date TEXT,
			active INTEGER,
			house_passage TEXT,
			senate_passage TEXT,
			enacted TEXT,
			vetoed TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS congress_bills_fts USING fts5(
			bill_id, title, short_title, sponsor_name, primary_subject, latest_major_action
		)`,

		// Congress Votes (ProPublica)
		`CREATE TABLE IF NOT EXISTS congress_votes (
			vote_id TEXT PRIMARY KEY,
			congress INTEGER,
			session INTEGER,
			chamber TEXT,
			roll_call INTEGER,
			bill_id TEXT,
			question TEXT,
			description TEXT,
			vote_type TEXT,
			date TEXT,
			time TEXT,
			result TEXT,
			democratic_yes INTEGER,
			democratic_no INTEGER,
			republican_yes INTEGER,
			republican_no INTEGER,
			independent_yes INTEGER,
			independent_no INTEGER,
			total_yes INTEGER,
			total_no INTEGER,
			total_not_voting INTEGER,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS congress_votes_fts USING fts5(
			vote_id, bill_id, question, description, result
		)`,

		// Campaign Finance (OpenSecrets)
		`CREATE TABLE IF NOT EXISTS campaign_finance (
			cid TEXT,
			candidate_name TEXT,
			cycle TEXT,
			party TEXT,
			state TEXT,
			chamber TEXT,
			incumbent INTEGER,
			total_receipts REAL,
			total_disbursements REAL,
			cash_on_hand REAL,
			debt REAL,
			individual_contributions REAL,
			pac_contributions REAL,
			self_financing REAL,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (cid, cycle)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS campaign_finance_fts USING fts5(
			cid, candidate_name, party, state, chamber
		)`,

		// PAC Contributions (OpenSecrets)
		`CREATE TABLE IF NOT EXISTS pac_contributions (
			pac_id TEXT,
			pac_name TEXT,
			candidate_id TEXT,
			candidate_name TEXT,
			cycle TEXT,
			amount REAL,
			party TEXT,
			state TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (pac_id, candidate_id, cycle)
		)`,

		// Korean National Assembly Members (열린국회정보)
		`CREATE TABLE IF NOT EXISTS korean_assembly_members (
			mona_cd TEXT PRIMARY KEY,
			hg_nm TEXT,
			hj_nm TEXT,
			eng_nm TEXT,
			bth_date TEXT,
			poly_nm TEXT,
			orig_nm TEXT,
			elect_gbn_nm TEXT,
			reele_gbn_nm TEXT,
			units TEXT,
			cmits TEXT,
			tel_no TEXT,
			email TEXT,
			mem_title TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS korean_assembly_members_fts USING fts5(
			mona_cd, hg_nm, poly_nm, orig_nm, cmits
		)`,

		// Korean Bills (열린국회정보 의안정보)
		`CREATE TABLE IF NOT EXISTS korean_bills (
			bill_id TEXT PRIMARY KEY,
			bill_no TEXT,
			bill_name TEXT,
			proposer TEXT,
			proposer_kind TEXT,
			propose_dt TEXT,
			committee TEXT,
			proc_result TEXT,
			proc_dt TEXT,
			link_url TEXT,
			detail_link TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS korean_bills_fts USING fts5(
			bill_id, bill_name, proposer, committee, proc_result
		)`,

		// Vote Smart - Politician Ratings
		`CREATE TABLE IF NOT EXISTS politician_ratings (
			candidate_id TEXT,
			rating_id TEXT,
			rating_name TEXT,
			sig_id TEXT,
			sig_name TEXT,
			rating TEXT,
			rating_text TEXT,
			timespan TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (candidate_id, rating_id)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS politician_ratings_fts USING fts5(
			candidate_id, rating_name, sig_name, rating_text
		)`,

		// Vote Smart - Candidate Bio
		`CREATE TABLE IF NOT EXISTS candidate_bios (
			candidate_id TEXT PRIMARY KEY,
			first_name TEXT,
			middle_name TEXT,
			last_name TEXT,
			suffix TEXT,
			nick_name TEXT,
			birth_date TEXT,
			birth_place TEXT,
			pronunciation TEXT,
			gender TEXT,
			family TEXT,
			home_city TEXT,
			home_state TEXT,
			education TEXT,
			profession TEXT,
			political TEXT,
			religion TEXT,
			congress_office TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS candidate_bios_fts USING fts5(
			candidate_id, first_name, last_name, home_state, profession, political
		)`,
	}
}

// EnsurePoliticsSchema creates the politics tables if they don't exist
func EnsurePoliticsSchema(d *db.DB) error {
	for _, schema := range PoliticsSchemas() {
		if _, err := d.Exec(schema); err != nil {
			return fmt.Errorf("politics schema error: %w", err)
		}
	}
	return nil
}

// =====================
// ProPublica Congress API
// =====================

type ProPublicaFetcher struct {
	BaseURL string
	APIKey  string
}

func NewProPublicaFetcher() *ProPublicaFetcher {
	apiKey := os.Getenv("PROPUBLICA_API_KEY")
	return &ProPublicaFetcher{
		BaseURL: "https://api.propublica.org/congress/v1",
		APIKey:  apiKey,
	}
}

// ProPublica API response structures
type ppMembersResponse struct {
	Status  string `json:"status"`
	Results []struct {
		Congress string `json:"congress"`
		Chamber  string `json:"chamber"`
		Members  []struct {
			ID                string  `json:"id"`
			FirstName         string  `json:"first_name"`
			LastName          string  `json:"last_name"`
			Party             string  `json:"party"`
			State             string  `json:"state"`
			Title             string  `json:"title"`
			TwitterAccount    string  `json:"twitter_account"`
			FacebookAccount   string  `json:"facebook_account"`
			URL               string  `json:"url"`
			NextElection      string  `json:"next_election"`
			TotalVotes        int     `json:"total_votes"`
			MissedVotesPct    float64 `json:"missed_votes_pct"`
			VotesWithPartyPct float64 `json:"votes_with_party_pct"`
		} `json:"members"`
	} `json:"results"`
}

type ppBillsResponse struct {
	Status  string `json:"status"`
	Results []struct {
		Bills []struct {
			BillID                string `json:"bill_id"`
			BillSlug              string `json:"bill_slug"`
			BillType              string `json:"bill_type"`
			Number                string `json:"number"`
			Title                 string `json:"title"`
			ShortTitle            string `json:"short_title"`
			SponsorID             string `json:"sponsor_id"`
			SponsorName           string `json:"sponsor_name"`
			SponsorParty          string `json:"sponsor_party"`
			SponsorState          string `json:"sponsor_state"`
			Committees            string `json:"committees"`
			PrimarySubject        string `json:"primary_subject"`
			IntroducedDate        string `json:"introduced_date"`
			LatestMajorAction     string `json:"latest_major_action"`
			LatestMajorActionDate string `json:"latest_major_action_date"`
			Active                bool   `json:"active"`
			HousePassage          string `json:"house_passage"`
			SenatePassage         string `json:"senate_passage"`
			Enacted               string `json:"enacted"`
			Vetoed                string `json:"vetoed"`
		} `json:"bills"`
	} `json:"results"`
}

type ppVotesResponse struct {
	Status  string `json:"status"`
	Results struct {
		Votes []struct {
			Congress int    `json:"congress"`
			Session  int    `json:"session"`
			Chamber  string `json:"chamber"`
			RollCall int    `json:"roll_call"`
			Bill     struct {
				BillID string `json:"bill_id"`
			} `json:"bill"`
			Question    string `json:"question"`
			Description string `json:"description"`
			VoteType    string `json:"vote_type"`
			Date        string `json:"date"`
			Time        string `json:"time"`
			Result      string `json:"result"`
			Democratic  struct {
				Yes int `json:"yes"`
				No  int `json:"no"`
			} `json:"democratic"`
			Republican struct {
				Yes int `json:"yes"`
				No  int `json:"no"`
			} `json:"republican"`
			Independent struct {
				Yes int `json:"yes"`
				No  int `json:"no"`
			} `json:"independent"`
			Total struct {
				Yes       int `json:"yes"`
				No        int `json:"no"`
				NotVoting int `json:"not_voting"`
			} `json:"total"`
		} `json:"votes"`
	} `json:"results"`
}

func (f *ProPublicaFetcher) makeRequest(endpoint string) (*http.Response, error) {
	if f.APIKey == "" {
		return nil, fmt.Errorf("PROPUBLICA_API_KEY environment variable not set")
	}

	req, err := http.NewRequest("GET", f.BaseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", f.APIKey)

	client := &http.Client{}
	return client.Do(req)
}

// FetchCongressMembers fetches members of the specified chamber (house/senate)
// congress: e.g., "118" for the 118th Congress
func (f *ProPublicaFetcher) FetchCongressMembers(d *db.DB, congress string, chamber string) (int, error) {
	if congress == "" {
		congress = "118" // Current Congress
	}
	if chamber == "" {
		chamber = "senate"
	}

	endpoint := fmt.Sprintf("/%s/%s/members.json", congress, chamber)
	resp, err := f.makeRequest(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result ppMembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Results) == 0 {
		return 0, nil
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_members
		(member_id, first_name, last_name, party, state, chamber, title,
		twitter_account, facebook_account, url, next_election,
		total_votes, missed_votes_pct, votes_with_party_pct)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_members_fts
		(member_id, first_name, last_name, party, state, chamber)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, res := range result.Results {
		for _, m := range res.Members {
			_, err := stmt.Exec(
				m.ID, m.FirstName, m.LastName, m.Party, m.State, chamber,
				m.Title, m.TwitterAccount, m.FacebookAccount, m.URL,
				m.NextElection, m.TotalVotes, m.MissedVotesPct, m.VotesWithPartyPct,
			)
			if err != nil {
				continue
			}
			ftsStmt.Exec(m.ID, m.FirstName, m.LastName, m.Party, m.State, chamber)
			count++
		}
	}

	return count, nil
}

// FetchRecentBills fetches recent bills from the specified chamber
func (f *ProPublicaFetcher) FetchRecentBills(d *db.DB, congress string, chamber string, billType string, limit int) (int, error) {
	if congress == "" {
		congress = "118"
	}
	if chamber == "" {
		chamber = "both"
	}
	if billType == "" {
		billType = "introduced"
	}

	endpoint := fmt.Sprintf("/%s/%s/bills/%s.json", congress, chamber, billType)
	resp, err := f.makeRequest(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result ppBillsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Results) == 0 {
		return 0, nil
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_bills
		(bill_id, bill_slug, bill_type, number, title, short_title,
		sponsor_id, sponsor_name, sponsor_party, sponsor_state,
		committees, primary_subject, introduced_date,
		latest_major_action, latest_major_action_date, active,
		house_passage, senate_passage, enacted, vetoed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_bills_fts
		(bill_id, title, short_title, sponsor_name, primary_subject, latest_major_action)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, res := range result.Results {
		for i, b := range res.Bills {
			if limit > 0 && i >= limit {
				break
			}

			active := 0
			if b.Active {
				active = 1
			}

			_, err := stmt.Exec(
				b.BillID, b.BillSlug, b.BillType, b.Number, b.Title, b.ShortTitle,
				b.SponsorID, b.SponsorName, b.SponsorParty, b.SponsorState,
				b.Committees, b.PrimarySubject, b.IntroducedDate,
				b.LatestMajorAction, b.LatestMajorActionDate, active,
				b.HousePassage, b.SenatePassage, b.Enacted, b.Vetoed,
			)
			if err != nil {
				continue
			}
			ftsStmt.Exec(b.BillID, b.Title, b.ShortTitle, b.SponsorName, b.PrimarySubject, b.LatestMajorAction)
			count++
		}
	}

	return count, nil
}

// FetchCongressVotes fetches recent votes from the specified chamber
func (f *ProPublicaFetcher) FetchCongressVotes(d *db.DB, chamber string, limit int) (int, error) {
	if chamber == "" {
		chamber = "senate"
	}

	endpoint := fmt.Sprintf("/%s/votes/recent.json", chamber)
	resp, err := f.makeRequest(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result ppVotesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_votes
		(vote_id, congress, session, chamber, roll_call, bill_id, question, description,
		vote_type, date, time, result,
		democratic_yes, democratic_no, republican_yes, republican_no,
		independent_yes, independent_no, total_yes, total_no, total_not_voting)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO congress_votes_fts
		(vote_id, bill_id, question, description, result)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for i, v := range result.Results.Votes {
		if limit > 0 && i >= limit {
			break
		}

		voteID := fmt.Sprintf("%d-%d-%s-%d", v.Congress, v.Session, v.Chamber, v.RollCall)

		_, err := stmt.Exec(
			voteID, v.Congress, v.Session, v.Chamber, v.RollCall,
			v.Bill.BillID, v.Question, v.Description, v.VoteType,
			v.Date, v.Time, v.Result,
			v.Democratic.Yes, v.Democratic.No, v.Republican.Yes, v.Republican.No,
			v.Independent.Yes, v.Independent.No, v.Total.Yes, v.Total.No, v.Total.NotVoting,
		)
		if err != nil {
			continue
		}
		ftsStmt.Exec(voteID, v.Bill.BillID, v.Question, v.Description, v.Result)
		count++
	}

	return count, nil
}

// =====================
// OpenSecrets API
// =====================

type OpenSecretsFetcher struct {
	BaseURL string
	APIKey  string
}

func NewOpenSecretsFetcher() *OpenSecretsFetcher {
	apiKey := os.Getenv("OPENSECRETS_API_KEY")
	return &OpenSecretsFetcher{
		BaseURL: "https://www.opensecrets.org/api/",
		APIKey:  apiKey,
	}
}

// OpenSecrets API response structures
type osCandSummaryResponse struct {
	Response struct {
		Summary struct {
			CID          string `json:"@attributes>cid,omitempty" xml:"cid,attr"`
			CandName     string `json:"cand_name,omitempty" xml:"cand_name,attr"`
			Cycle        string `json:"cycle,omitempty" xml:"cycle,attr"`
			Party        string `json:"party,omitempty" xml:"party,attr"`
			State        string `json:"state,omitempty" xml:"state,attr"`
			Chamber      string `json:"chamber,omitempty" xml:"chamber,attr"`
			FirstElected string `json:"first_elected,omitempty" xml:"first_elected,attr"`
			NextElection string `json:"next_election,omitempty" xml:"next_election,attr"`
			Total        string `json:"total,omitempty" xml:"total,attr"`
			Spent        string `json:"spent,omitempty" xml:"spent,attr"`
			CashOnHand   string `json:"cash_on_hand,omitempty" xml:"cash_on_hand,attr"`
			Debt         string `json:"debt,omitempty" xml:"debt,attr"`
		} `json:"summary" xml:"summary"`
	} `json:"response" xml:"response"`
}

type osXMLCandSummaryResponse struct {
	XMLName xml.Name `xml:"response"`
	Summary struct {
		CID        string `xml:"cid,attr"`
		CandName   string `xml:"cand_name,attr"`
		Cycle      string `xml:"cycle,attr"`
		Party      string `xml:"party,attr"`
		State      string `xml:"state,attr"`
		Chamber    string `xml:"chamber,attr"`
		Total      string `xml:"total,attr"`
		Spent      string `xml:"spent,attr"`
		CashOnHand string `xml:"cash_on_hand,attr"`
		Debt       string `xml:"debt,attr"`
	} `xml:"summary"`
}

type osXMLCandContribResponse struct {
	XMLName      xml.Name `xml:"response"`
	Contributors struct {
		Contributor []struct {
			OrgName string `xml:"org_name,attr"`
			Total   string `xml:"total,attr"`
			PACs    string `xml:"pacs,attr"`
			Indivs  string `xml:"indivs,attr"`
		} `xml:"contributor"`
	} `xml:"contributors"`
}

func (f *OpenSecretsFetcher) makeRequest(method string, params url.Values) (*http.Response, error) {
	if f.APIKey == "" {
		return nil, fmt.Errorf("OPENSECRETS_API_KEY environment variable not set")
	}

	params.Set("apikey", f.APIKey)
	params.Set("method", method)
	params.Set("output", "xml")

	req, err := http.NewRequest("GET", f.BaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{}
	return client.Do(req)
}

// FetchCampaignFinance fetches campaign finance summary for a candidate
// cid: Candidate ID (e.g., "N00007360" for Nancy Pelosi)
// cycle: Election cycle (e.g., "2024")
func (f *OpenSecretsFetcher) FetchCampaignFinance(d *db.DB, cid string, cycle string) (int, error) {
	if cid == "" {
		return 0, fmt.Errorf("candidate ID (cid) is required")
	}
	if cycle == "" {
		cycle = "2024"
	}

	params := url.Values{}
	params.Set("cid", cid)
	params.Set("cycle", cycle)

	resp, err := f.makeRequest("candSummary", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result osXMLCandSummaryResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	s := result.Summary
	if s.CID == "" {
		return 0, nil
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO campaign_finance
		(cid, candidate_name, cycle, party, state, chamber, total_receipts,
		total_disbursements, cash_on_hand, debt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO campaign_finance_fts
		(cid, candidate_name, party, state, chamber)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	_, err = stmt.Exec(
		s.CID, s.CandName, s.Cycle, s.Party, s.State, s.Chamber,
		s.Total, s.Spent, s.CashOnHand, s.Debt,
	)
	if err != nil {
		return 0, err
	}

	ftsStmt.Exec(s.CID, s.CandName, s.Party, s.State, s.Chamber)
	return 1, nil
}

// FetchCandidateContributors fetches top contributors for a candidate
func (f *OpenSecretsFetcher) FetchCandidateContributors(d *db.DB, cid string, cycle string) (int, error) {
	if cid == "" {
		return 0, fmt.Errorf("candidate ID (cid) is required")
	}
	if cycle == "" {
		cycle = "2024"
	}

	params := url.Values{}
	params.Set("cid", cid)
	params.Set("cycle", cycle)

	resp, err := f.makeRequest("candContrib", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result osXMLCandContribResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO pac_contributions
		(pac_id, pac_name, candidate_id, candidate_name, cycle, amount)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, c := range result.Contributors.Contributor {
		// Generate a pac_id from the org name (OpenSecrets doesn't provide one in this endpoint)
		pacID := strings.ReplaceAll(strings.ToLower(c.OrgName), " ", "_")

		_, err := stmt.Exec(
			pacID, c.OrgName, cid, "", cycle, c.Total,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// =====================
// Korean National Assembly API (열린국회정보)
// =====================

type KoreanAssemblyFetcher struct {
	BaseURL string
	APIKey  string
}

func NewKoreanAssemblyFetcher() *KoreanAssemblyFetcher {
	apiKey := os.Getenv("KOREAN_ASSEMBLY_API_KEY")
	return &KoreanAssemblyFetcher{
		BaseURL: "https://open.assembly.go.kr/portal/openapi",
		APIKey:  apiKey,
	}
}

// Korean Assembly API response structures
type kaXMLMembersResponse struct {
	XMLName xml.Name `xml:"response"`
	Header  struct {
		ResultCode    string `xml:"resultCode"`
		ResultMessage string `xml:"resultMessage"`
	} `xml:"header"`
	Body struct {
		Items struct {
			Item []struct {
				MonaCd     string `xml:"MONA_CD"`
				HgNm       string `xml:"HG_NM"`
				HjNm       string `xml:"HJ_NM"`
				EngNm      string `xml:"ENG_NM"`
				BthDate    string `xml:"BTH_DATE"`
				PolyNm     string `xml:"POLY_NM"`
				OrigNm     string `xml:"ORIG_NM"`
				ElectGbnNm string `xml:"ELECT_GBN_NM"`
				ReeleGbnNm string `xml:"REELE_GBN_NM"`
				Units      string `xml:"UNITS"`
				Cmits      string `xml:"CMITS"`
				TelNo      string `xml:"TEL_NO"`
				Email      string `xml:"E_MAIL"`
				MemTitle   string `xml:"MEM_TITLE"`
			} `xml:"item"`
		} `xml:"items"`
	} `xml:"body"`
}

type kaJSONMembersResponse struct {
	Response struct {
		Head []struct {
			ListTotalCount int `json:"list_total_count,omitempty"`
			Result         struct {
				Code    string `json:"CODE"`
				Message string `json:"MESSAGE"`
			} `json:"RESULT,omitempty"`
		} `json:"head"`
		Row []struct {
			MonaCd     string `json:"MONA_CD"`
			HgNm       string `json:"HG_NM"`
			HjNm       string `json:"HJ_NM"`
			EngNm      string `json:"ENG_NM"`
			BthDate    string `json:"BTH_DATE"`
			PolyNm     string `json:"POLY_NM"`
			OrigNm     string `json:"ORIG_NM"`
			ElectGbnNm string `json:"ELECT_GBN_NM"`
			ReeleGbnNm string `json:"REELE_GBN_NM"`
			Units      string `json:"UNITS"`
			Cmits      string `json:"CMITS"`
			TelNo      string `json:"TEL_NO"`
			Email      string `json:"E_MAIL"`
			MemTitle   string `json:"MEM_TITLE"`
		} `json:"row"`
	} `json:"ALLNAMEMBER"`
}

type kaJSONBillsResponse struct {
	Response struct {
		Head []struct {
			ListTotalCount int `json:"list_total_count,omitempty"`
			Result         struct {
				Code    string `json:"CODE"`
				Message string `json:"MESSAGE"`
			} `json:"RESULT,omitempty"`
		} `json:"head"`
		Row []struct {
			BillID       string `json:"BILL_ID"`
			BillNo       string `json:"BILL_NO"`
			BillName     string `json:"BILL_NAME"`
			Proposer     string `json:"PROPOSER"`
			ProposerKind string `json:"PROPOSER_KIND"`
			ProposeDt    string `json:"PROPOSE_DT"`
			Committee    string `json:"COMMITTEE"`
			ProcResult   string `json:"PROC_RESULT"`
			ProcDt       string `json:"PROC_DT"`
			LinkURL      string `json:"LINK_URL"`
			DetailLink   string `json:"DETAIL_LINK"`
		} `json:"row"`
	} `json:"TVBPMBILL11"`
}

func (f *KoreanAssemblyFetcher) makeRequest(serviceID string, params url.Values) (*http.Response, error) {
	if f.APIKey == "" {
		return nil, fmt.Errorf("KOREAN_ASSEMBLY_API_KEY environment variable not set")
	}

	// Korean Assembly API uses path-based authentication
	// Format: /portal/openapi/{serviceID}?KEY={apikey}&...
	fullURL := fmt.Sprintf("%s/%s?KEY=%s&Type=json&%s",
		f.BaseURL, serviceID, f.APIKey, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{}
	return client.Do(req)
}

// FetchKoreanMembers fetches current National Assembly members
func (f *KoreanAssemblyFetcher) FetchKoreanMembers(d *db.DB, limit int) (int, error) {
	if limit == 0 {
		limit = 300 // Max members in assembly
	}

	params := url.Values{}
	params.Set("pIndex", "1")
	params.Set("pSize", fmt.Sprintf("%d", limit))

	resp, err := f.makeRequest("ALLNAMEMBER", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result kaJSONMembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_assembly_members
		(mona_cd, hg_nm, hj_nm, eng_nm, bth_date, poly_nm, orig_nm,
		elect_gbn_nm, reele_gbn_nm, units, cmits, tel_no, email, mem_title)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_assembly_members_fts
		(mona_cd, hg_nm, poly_nm, orig_nm, cmits)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, m := range result.Response.Row {
		_, err := stmt.Exec(
			m.MonaCd, m.HgNm, m.HjNm, m.EngNm, m.BthDate, m.PolyNm, m.OrigNm,
			m.ElectGbnNm, m.ReeleGbnNm, m.Units, m.Cmits, m.TelNo, m.Email, m.MemTitle,
		)
		if err != nil {
			continue
		}
		ftsStmt.Exec(m.MonaCd, m.HgNm, m.PolyNm, m.OrigNm, m.Cmits)
		count++
	}

	return count, nil
}

// FetchKoreanBills fetches bills from the National Assembly
// query: Bill name search keyword
// age: Assembly age (e.g., "22" for the 22nd National Assembly)
func (f *KoreanAssemblyFetcher) FetchKoreanBills(d *db.DB, query string, age string, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}
	if age == "" {
		age = "22" // Current assembly age
	}

	params := url.Values{}
	params.Set("pIndex", "1")
	params.Set("pSize", fmt.Sprintf("%d", limit))
	params.Set("AGE", age)
	if query != "" {
		params.Set("BILL_NAME", query)
	}

	resp, err := f.makeRequest("TVBPMBILL11", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result kaJSONBillsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_bills
		(bill_id, bill_no, bill_name, proposer, proposer_kind, propose_dt,
		committee, proc_result, proc_dt, link_url, detail_link)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_bills_fts
		(bill_id, bill_name, proposer, committee, proc_result)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, b := range result.Response.Row {
		_, err := stmt.Exec(
			b.BillID, b.BillNo, b.BillName, b.Proposer, b.ProposerKind, b.ProposeDt,
			b.Committee, b.ProcResult, b.ProcDt, b.LinkURL, b.DetailLink,
		)
		if err != nil {
			continue
		}
		ftsStmt.Exec(b.BillID, b.BillName, b.Proposer, b.Committee, b.ProcResult)
		count++
	}

	return count, nil
}

// =====================
// Vote Smart API
// =====================

type VoteSmartFetcher struct {
	BaseURL string
	APIKey  string
}

func NewVoteSmartFetcher() *VoteSmartFetcher {
	apiKey := os.Getenv("VOTESMART_API_KEY")
	return &VoteSmartFetcher{
		BaseURL: "https://api.votesmart.org",
		APIKey:  apiKey,
	}
}

// Vote Smart API response structures
type vsXMLCandidateBioResponse struct {
	XMLName   xml.Name `xml:"bio"`
	Candidate struct {
		CandidateID    string `xml:"candidateId"`
		FirstName      string `xml:"firstName"`
		MiddleName     string `xml:"middleName"`
		LastName       string `xml:"lastName"`
		Suffix         string `xml:"suffix"`
		NickName       string `xml:"nickName"`
		BirthDate      string `xml:"birthDate"`
		BirthPlace     string `xml:"birthPlace"`
		Pronunciation  string `xml:"pronunciation"`
		Gender         string `xml:"gender"`
		Family         string `xml:"family"`
		HomeCity       string `xml:"homeCity"`
		HomeState      string `xml:"homeState"`
		Education      string `xml:"education"`
		Profession     string `xml:"profession"`
		Political      string `xml:"political"`
		Religion       string `xml:"religion"`
		CongressOffice string `xml:"congMemberOffice>congressOffice>address1"`
	} `xml:"candidate"`
}

type vsXMLRatingResponse struct {
	XMLName xml.Name `xml:"ratings"`
	Rating  []struct {
		RatingID   string `xml:"ratingId"`
		RatingName string `xml:"ratingName"`
		SigID      string `xml:"sigId"`
		SigName    string `xml:"sigName"`
		Rating     string `xml:"rating"`
		RatingText string `xml:"ratingText"`
		Timespan   string `xml:"timespan"`
	} `xml:"rating"`
}

type vsJSONBioResponse struct {
	Bio struct {
		Candidate struct {
			CandidateID   string `json:"candidateId"`
			FirstName     string `json:"firstName"`
			MiddleName    string `json:"middleName"`
			LastName      string `json:"lastName"`
			Suffix        string `json:"suffix"`
			NickName      string `json:"nickName"`
			BirthDate     string `json:"birthDate"`
			BirthPlace    string `json:"birthPlace"`
			Pronunciation string `json:"pronunciation"`
			Gender        string `json:"gender"`
			Family        string `json:"family"`
			HomeCity      string `json:"homeCity"`
			HomeState     string `json:"homeState"`
			Education     string `json:"education"`
			Profession    string `json:"profession"`
			Political     string `json:"political"`
			Religion      string `json:"religion"`
		} `json:"candidate"`
	} `json:"bio"`
}

type vsJSONRatingsResponse struct {
	CandidateRating struct {
		Rating []struct {
			RatingID   string `json:"ratingId"`
			RatingName string `json:"ratingName"`
			SigID      string `json:"sigId"`
			SigName    string `json:"sigName"`
			Rating     string `json:"rating"`
			RatingText string `json:"ratingText"`
			Timespan   string `json:"timespan"`
		} `json:"rating"`
	} `json:"candidateRating"`
}

func (f *VoteSmartFetcher) makeRequest(method string, params url.Values) (*http.Response, error) {
	if f.APIKey == "" {
		return nil, fmt.Errorf("VOTESMART_API_KEY environment variable not set")
	}

	params.Set("key", f.APIKey)
	params.Set("o", "JSON")

	fullURL := fmt.Sprintf("%s/%s?%s", f.BaseURL, method, params.Encode())
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mimir-mcp/1.0")

	client := &http.Client{}
	return client.Do(req)
}

// FetchCandidateBio fetches biographical information for a candidate
// candidateID: Vote Smart candidate ID
func (f *VoteSmartFetcher) FetchCandidateBio(d *db.DB, candidateID string) (int, error) {
	if candidateID == "" {
		return 0, fmt.Errorf("candidate ID is required")
	}

	params := url.Values{}
	params.Set("candidateId", candidateID)

	resp, err := f.makeRequest("CandidateBio.getBio", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result vsJSONBioResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	c := result.Bio.Candidate
	if c.CandidateID == "" {
		return 0, nil
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO candidate_bios
		(candidate_id, first_name, middle_name, last_name, suffix, nick_name,
		birth_date, birth_place, pronunciation, gender, family,
		home_city, home_state, education, profession, political, religion)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO candidate_bios_fts
		(candidate_id, first_name, last_name, home_state, profession, political)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	_, err = stmt.Exec(
		c.CandidateID, c.FirstName, c.MiddleName, c.LastName, c.Suffix, c.NickName,
		c.BirthDate, c.BirthPlace, c.Pronunciation, c.Gender, c.Family,
		c.HomeCity, c.HomeState, c.Education, c.Profession, c.Political, c.Religion,
	)
	if err != nil {
		return 0, err
	}

	ftsStmt.Exec(c.CandidateID, c.FirstName, c.LastName, c.HomeState, c.Profession, c.Political)
	return 1, nil
}

// FetchCandidateRatings fetches ratings for a candidate from various SIGs
// candidateID: Vote Smart candidate ID
func (f *VoteSmartFetcher) FetchCandidateRatings(d *db.DB, candidateID string) (int, error) {
	if candidateID == "" {
		return 0, fmt.Errorf("candidate ID is required")
	}

	params := url.Values{}
	params.Set("candidateId", candidateID)

	resp, err := f.makeRequest("Rating.getCandidateRating", params)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result vsJSONRatingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO politician_ratings
		(candidate_id, rating_id, rating_name, sig_id, sig_name, rating, rating_text, timespan)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO politician_ratings_fts
		(candidate_id, rating_name, sig_name, rating_text)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, r := range result.CandidateRating.Rating {
		_, err := stmt.Exec(
			candidateID, r.RatingID, r.RatingName, r.SigID, r.SigName,
			r.Rating, r.RatingText, r.Timespan,
		)
		if err != nil {
			continue
		}
		ftsStmt.Exec(candidateID, r.RatingName, r.SigName, r.RatingText)
		count++
	}

	return count, nil
}

// =====================
// Unified Politics Fetcher
// =====================

type PoliticsFetcher struct {
	ProPublica     *ProPublicaFetcher
	OpenSecrets    *OpenSecretsFetcher
	KoreanAssembly *KoreanAssemblyFetcher
	VoteSmart      *VoteSmartFetcher
}

func NewPoliticsFetcher() *PoliticsFetcher {
	return &PoliticsFetcher{
		ProPublica:     NewProPublicaFetcher(),
		OpenSecrets:    NewOpenSecretsFetcher(),
		KoreanAssembly: NewKoreanAssemblyFetcher(),
		VoteSmart:      NewVoteSmartFetcher(),
	}
}

// PoliticsFetchResult holds the results from a politics fetch operation
type PoliticsFetchResult struct {
	CongressMembers int `json:"congress_members"`
	CongressBills   int `json:"congress_bills"`
	CongressVotes   int `json:"congress_votes"`
	CampaignFinance int `json:"campaign_finance"`
	KoreanMembers   int `json:"korean_members"`
	KoreanBills     int `json:"korean_bills"`
	CandidateBios   int `json:"candidate_bios"`
	Ratings         int `json:"ratings"`
}

// FetchAll fetches politics data from all available sources
// ALL POLITICS APIS REQUIRE KEYS - returns empty result if none available
// ProPublica: PROPUBLICA_API_KEY
// OpenSecrets: OPENSECRETS_API_KEY
// Korean Assembly: KOREAN_ASSEMBLY_API_KEY
// Vote Smart: VOTESMART_API_KEY
func (f *PoliticsFetcher) FetchAll(d *db.DB) (*PoliticsFetchResult, error) {
	result := &PoliticsFetchResult{}

	// Check if any sources are available
	sources := f.AvailableSources()
	hasAny := false
	for _, v := range sources {
		if v {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return result, nil // No keys set, return empty
	}

	// Ensure schema exists
	if err := EnsurePoliticsSchema(d); err != nil {
		return nil, err
	}

	// US Congress - ProPublica (requires key)
	if f.ProPublica.APIKey != "" {
		if count, err := f.ProPublica.FetchCongressMembers(d, "118", "senate"); err == nil {
			result.CongressMembers += count
		}
		if count, err := f.ProPublica.FetchCongressMembers(d, "118", "house"); err == nil {
			result.CongressMembers += count
		}
		if count, err := f.ProPublica.FetchRecentBills(d, "118", "both", "introduced", 100); err == nil {
			result.CongressBills = count
		}
		if count, err := f.ProPublica.FetchCongressVotes(d, "senate", 50); err == nil {
			result.CongressVotes += count
		}
		if count, err := f.ProPublica.FetchCongressVotes(d, "house", 50); err == nil {
			result.CongressVotes += count
		}
	}

	// Korean National Assembly (requires key)
	if f.KoreanAssembly.APIKey != "" {
		if count, err := f.KoreanAssembly.FetchKoreanMembers(d, 0); err == nil {
			result.KoreanMembers = count
		}
		if count, err := f.KoreanAssembly.FetchKoreanBills(d, "", "", 100); err == nil {
			result.KoreanBills = count
		}
	}

	return result, nil
}

// AvailableSources returns which politics sources are available
// NOTE: All politics APIs require API keys
func (f *PoliticsFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"propublica":       f.ProPublica.APIKey != "",      // Requires PROPUBLICA_API_KEY
		"opensecrets":      f.OpenSecrets.APIKey != "",     // Requires OPENSECRETS_API_KEY
		"korean_assembly":  f.KoreanAssembly.APIKey != "",  // Requires KOREAN_ASSEMBLY_API_KEY
		"votesmart":        f.VoteSmart.APIKey != "",       // Requires VOTESMART_API_KEY
	}
}

// PoliticsStats holds statistics from politics tables
type PoliticsStats struct {
	CongressMembers int `json:"congress_members"`
	CongressBills   int `json:"congress_bills"`
	CongressVotes   int `json:"congress_votes"`
	CampaignFinance int `json:"campaign_finance"`
	KoreanMembers   int `json:"korean_members"`
	KoreanBills     int `json:"korean_bills"`
	CandidateBios   int `json:"candidate_bios"`
	Ratings         int `json:"ratings"`
}

// GetStats returns statistics from all politics tables
func (f *PoliticsFetcher) GetStats(d *db.DB) (*PoliticsStats, error) {
	stats := &PoliticsStats{}

	tables := map[string]*int{
		"congress_members":        &stats.CongressMembers,
		"congress_bills":          &stats.CongressBills,
		"congress_votes":          &stats.CongressVotes,
		"campaign_finance":        &stats.CampaignFinance,
		"korean_assembly_members": &stats.KoreanMembers,
		"korean_bills":            &stats.KoreanBills,
		"candidate_bios":          &stats.CandidateBios,
		"politician_ratings":      &stats.Ratings,
	}

	for table, ptr := range tables {
		var count int
		row := d.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
		if err := row.Scan(&count); err == nil {
			*ptr = count
		}
	}

	return stats, nil
}

// =====================
// Search Functions
// =====================

// CongressMemberResult represents a congress member search result
type CongressMemberResult struct {
	MemberID  string  `json:"member_id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Party     string  `json:"party"`
	State     string  `json:"state"`
	Chamber   string  `json:"chamber"`
	VotesPct  float64 `json:"votes_with_party_pct"`
}

// CongressBillResult represents a congress bill search result
type CongressBillResult struct {
	BillID         string `json:"bill_id"`
	Title          string `json:"title"`
	SponsorName    string `json:"sponsor_name"`
	PrimarySubject string `json:"primary_subject"`
	IntroducedDate string `json:"introduced_date"`
	LatestAction   string `json:"latest_action"`
}

// KoreanBillResult represents a Korean bill search result
type KoreanBillResult struct {
	BillID     string `json:"bill_id"`
	BillName   string `json:"bill_name"`
	Proposer   string `json:"proposer"`
	Committee  string `json:"committee"`
	ProcResult string `json:"proc_result"`
	ProposeDt  string `json:"propose_dt"`
}

// PoliticsSearchResult holds results from a unified politics search
type PoliticsSearchResult struct {
	CongressMembers []CongressMemberResult `json:"congress_members"`
	CongressBills   []CongressBillResult   `json:"congress_bills"`
	KoreanBills     []KoreanBillResult     `json:"korean_bills"`
}

// Search performs a unified search across all politics sources
func (f *PoliticsFetcher) Search(d *db.DB, query string, limit int) (*PoliticsSearchResult, error) {
	if limit == 0 {
		limit = 20
	}

	result := &PoliticsSearchResult{
		CongressMembers: []CongressMemberResult{},
		CongressBills:   []CongressBillResult{},
		KoreanBills:     []KoreanBillResult{},
	}

	// Search congress members
	memberRows, err := d.Query(`
		SELECT member_id, first_name, last_name, party, state, chamber
		FROM congress_members_fts
		WHERE congress_members_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer memberRows.Close()
		for memberRows.Next() {
			var m CongressMemberResult
			if err := memberRows.Scan(&m.MemberID, &m.FirstName, &m.LastName, &m.Party, &m.State, &m.Chamber); err == nil {
				result.CongressMembers = append(result.CongressMembers, m)
			}
		}
	}

	// Search congress bills
	billRows, err := d.Query(`
		SELECT bill_id, title, sponsor_name, primary_subject, latest_major_action
		FROM congress_bills_fts
		WHERE congress_bills_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer billRows.Close()
		for billRows.Next() {
			var b CongressBillResult
			if err := billRows.Scan(&b.BillID, &b.Title, &b.SponsorName, &b.PrimarySubject, &b.LatestAction); err == nil {
				result.CongressBills = append(result.CongressBills, b)
			}
		}
	}

	// Search Korean bills
	kbillRows, err := d.Query(`
		SELECT bill_id, bill_name, proposer, committee, proc_result
		FROM korean_bills_fts
		WHERE korean_bills_fts MATCH ?
		LIMIT ?
	`, query, limit)
	if err == nil {
		defer kbillRows.Close()
		for kbillRows.Next() {
			var kb KoreanBillResult
			if err := kbillRows.Scan(&kb.BillID, &kb.BillName, &kb.Proposer, &kb.Committee, &kb.ProcResult); err == nil {
				result.KoreanBills = append(result.KoreanBills, kb)
			}
		}
	}

	return result, nil
}
