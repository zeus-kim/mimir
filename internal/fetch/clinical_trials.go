package fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/user/mimir-mcp/internal/db"
)

type ClinicalTrialsFetcher struct {
	BaseURL string
}

func NewClinicalTrialsFetcher() *ClinicalTrialsFetcher {
	return &ClinicalTrialsFetcher{
		BaseURL: "https://clinicaltrials.gov/api/v2/studies",
	}
}

type ctStudy struct {
	ProtocolSection struct {
		IdentificationModule struct {
			NCTId        string `json:"nctId"`
			BriefTitle   string `json:"briefTitle"`
			Organization struct {
				FullName string `json:"fullName"`
			} `json:"organization"`
		} `json:"identificationModule"`
		StatusModule struct {
			OverallStatus string `json:"overallStatus"`
			StartDateStruct struct {
				Date string `json:"date"`
			} `json:"startDateStruct"`
			CompletionDateStruct struct {
				Date string `json:"date"`
			} `json:"completionDateStruct"`
		} `json:"statusModule"`
		DesignModule struct {
			Phases []string `json:"phases"`
		} `json:"designModule"`
		ConditionsModule struct {
			Conditions []string `json:"conditions"`
		} `json:"conditionsModule"`
		ArmsInterventionsModule struct {
			Interventions []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"interventions"`
		} `json:"armsInterventionsModule"`
		DescriptionModule struct {
			BriefSummary string `json:"briefSummary"`
		} `json:"descriptionModule"`
	} `json:"protocolSection"`
}

type ctResponse struct {
	Studies    []ctStudy `json:"studies"`
	NextPageToken string `json:"nextPageToken"`
}

func (f *ClinicalTrialsFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}

	params := url.Values{}
	params.Set("query.term", query)
	params.Set("pageSize", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("format", "json")

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result ctResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO clinical_trials
		(nct_id, title, sponsor, phase, status, conditions, interventions, brief_summary, start_date, completion_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO clinical_trials_fts
		(nct_id, title, sponsor, conditions, interventions, brief_summary)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, study := range result.Studies {
		id := study.ProtocolSection.IdentificationModule
		status := study.ProtocolSection.StatusModule
		design := study.ProtocolSection.DesignModule
		cond := study.ProtocolSection.ConditionsModule
		arms := study.ProtocolSection.ArmsInterventionsModule
		desc := study.ProtocolSection.DescriptionModule

		conditions := strings.Join(cond.Conditions, "; ")
		var interventions []string
		for _, i := range arms.Interventions {
			interventions = append(interventions, i.Name)
		}
		interventionsStr := strings.Join(interventions, "; ")
		phase := strings.Join(design.Phases, ", ")

		_, err := stmt.Exec(
			id.NCTId, id.BriefTitle, id.Organization.FullName,
			phase, status.OverallStatus, conditions, interventionsStr,
			desc.BriefSummary, status.StartDateStruct.Date, status.CompletionDateStruct.Date,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(id.NCTId, id.BriefTitle, id.Organization.FullName, conditions, interventionsStr, desc.BriefSummary)
		count++
	}

	return count, nil
}
