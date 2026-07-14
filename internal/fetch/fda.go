package fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/zeus-kim/mimir/internal/db"
)

type FDAFetcher struct {
	DrugsURL  string
	EventsURL string
}

func NewFDAFetcher() *FDAFetcher {
	return &FDAFetcher{
		DrugsURL:  "https://api.fda.gov/drug/drugsfda.json",
		EventsURL: "https://api.fda.gov/drug/event.json",
	}
}

type fdaDrugResponse struct {
	Results []struct {
		ApplicationNumber string `json:"application_number"`
		SponsorName       string `json:"sponsor_name"`
		Products          []struct {
			BrandName  string `json:"brand_name"`
			ActiveIngredients []struct {
				Name string `json:"name"`
			} `json:"active_ingredients"`
		} `json:"products"`
		Submissions []struct {
			SubmissionType   string `json:"submission_type"`
			SubmissionStatus string `json:"submission_status"`
			SubmissionStatusDate string `json:"submission_status_date"`
		} `json:"submissions"`
	} `json:"results"`
}

type fdaEventResponse struct {
	Results []struct {
		SafetyReportID string `json:"safetyreportid"`
		Serious        int    `json:"serious"`
		ReceiveDate    string `json:"receivedate"`
		Patient        struct {
			Drug []struct {
				MedicinalProduct string `json:"medicinalproduct"`
			} `json:"drug"`
			Reaction []struct {
				ReactionMedDRApt string `json:"reactionmeddrapt"`
			} `json:"reaction"`
		} `json:"patient"`
	} `json:"results"`
}

func (f *FDAFetcher) FetchApprovals(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}

	params := url.Values{}
	if query != "" {
		params.Set("search", fmt.Sprintf("products.brand_name:%s", query))
	}
	params.Set("limit", fmt.Sprintf("%d", min(limit, 100)))

	resp, err := http.Get(f.DrugsURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result fdaDrugResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO fda_approvals
		(application_number, brand_name, generic_name, sponsor_name, approval_date, submission_type)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, drug := range result.Results {
		var brandName, genericName, approvalDate, submissionType string

		if len(drug.Products) > 0 {
			brandName = drug.Products[0].BrandName
			if len(drug.Products[0].ActiveIngredients) > 0 {
				genericName = drug.Products[0].ActiveIngredients[0].Name
			}
		}

		for _, sub := range drug.Submissions {
			if sub.SubmissionStatus == "AP" {
				approvalDate = sub.SubmissionStatusDate
				submissionType = sub.SubmissionType
				break
			}
		}

		_, err := stmt.Exec(
			drug.ApplicationNumber, brandName, genericName,
			drug.SponsorName, approvalDate, submissionType,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

func (f *FDAFetcher) FetchAdverseEvents(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}

	params := url.Values{}
	if query != "" {
		params.Set("search", fmt.Sprintf("patient.drug.medicinalproduct:%s", query))
	}
	params.Set("limit", fmt.Sprintf("%d", min(limit, 100)))

	resp, err := http.Get(f.EventsURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result fdaEventResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO fda_adverse_events
		(report_id, drug_names, reactions, serious, receive_date)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, event := range result.Results {
		var drugs, reactions []string

		for _, drug := range event.Patient.Drug {
			if drug.MedicinalProduct != "" {
				drugs = append(drugs, drug.MedicinalProduct)
			}
		}
		for _, reaction := range event.Patient.Reaction {
			if reaction.ReactionMedDRApt != "" {
				reactions = append(reactions, reaction.ReactionMedDRApt)
			}
		}

		_, err := stmt.Exec(
			event.SafetyReportID,
			strings.Join(drugs, "; "),
			strings.Join(reactions, "; "),
			event.Serious,
			event.ReceiveDate,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}
