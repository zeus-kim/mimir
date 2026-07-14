package fetch

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/zeus-kim/mimir/internal/db"
)

type PubMedFetcher struct {
	SearchURL string
	FetchURL  string
}

func NewPubMedFetcher() *PubMedFetcher {
	return &PubMedFetcher{
		SearchURL: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi",
		FetchURL:  "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi",
	}
}

type esearchResult struct {
	XMLName xml.Name `xml:"eSearchResult"`
	IdList  struct {
		Id []string `xml:"Id"`
	} `xml:"IdList"`
}

type pubmedArticleSet struct {
	XMLName  xml.Name `xml:"PubmedArticleSet"`
	Articles []struct {
		MedlineCitation struct {
			PMID    string `xml:"PMID"`
			Article struct {
				ArticleTitle string `xml:"ArticleTitle"`
				Abstract     struct {
					AbstractText string `xml:"AbstractText"`
				} `xml:"Abstract"`
				AuthorList struct {
					Authors []struct {
						LastName string `xml:"LastName"`
						ForeName string `xml:"ForeName"`
					} `xml:"Author"`
				} `xml:"AuthorList"`
				Journal struct {
					Title        string `xml:"Title"`
					ISOAbbreviation string `xml:"ISOAbbreviation"`
				} `xml:"Journal"`
				ELocationID struct {
					EIdType string `xml:"EIdType,attr"`
					Value   string `xml:",chardata"`
				} `xml:"ELocationID"`
			} `xml:"Article"`
			DateCompleted struct {
				Year string `xml:"Year"`
			} `xml:"DateCompleted"`
		} `xml:"MedlineCitation"`
	} `xml:"PubmedArticle"`
}

func (f *PubMedFetcher) Fetch(d *db.DB, query string, limit int) (int, error) {
	if limit == 0 {
		limit = 50
	}

	// Search for IDs
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("term", query)
	params.Set("retmax", fmt.Sprintf("%d", limit))
	params.Set("retmode", "xml")
	params.Set("sort", "relevance")

	resp, err := http.Get(f.SearchURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var search esearchResult
	if err := xml.NewDecoder(resp.Body).Decode(&search); err != nil {
		return 0, err
	}

	if len(search.IdList.Id) == 0 {
		return 0, nil
	}

	// Fetch details
	fetchParams := url.Values{}
	fetchParams.Set("db", "pubmed")
	fetchParams.Set("id", strings.Join(search.IdList.Id, ","))
	fetchParams.Set("retmode", "xml")

	fetchResp, err := http.Get(f.FetchURL + "?" + fetchParams.Encode())
	if err != nil {
		return 0, err
	}
	defer fetchResp.Body.Close()

	var articles pubmedArticleSet
	if err := xml.NewDecoder(fetchResp.Body).Decode(&articles); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO pubmed_articles
		(pmid, title, abstract, authors, journal, journal_abbrev, pub_year, doi)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO pubmed_fts
		(pmid, title, abstract, authors, journal)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer ftsStmt.Close()

	count := 0
	for _, article := range articles.Articles {
		mc := article.MedlineCitation
		a := mc.Article

		var authors []string
		for _, author := range a.AuthorList.Authors {
			authors = append(authors, author.LastName+" "+author.ForeName)
		}
		authorsStr := strings.Join(authors, "; ")

		var doi string
		if a.ELocationID.EIdType == "doi" {
			doi = a.ELocationID.Value
		}

		var year int
		fmt.Sscanf(mc.DateCompleted.Year, "%d", &year)

		_, err := stmt.Exec(
			mc.PMID, a.ArticleTitle, a.Abstract.AbstractText,
			authorsStr, a.Journal.Title, a.Journal.ISOAbbreviation,
			year, doi,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(mc.PMID, a.ArticleTitle, a.Abstract.AbstractText, authorsStr, a.Journal.Title)
		count++
	}

	return count, nil
}
