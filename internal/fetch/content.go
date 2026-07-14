package fetch

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/user/mimir-mcp/internal/db"
)

// ContentFetcher fetches and extracts article content from URLs
type ContentFetcher struct {
	Client     *http.Client
	MaxWorkers int
	MinLength  int // minimum content length to accept
}

// NewContentFetcher creates a ContentFetcher with default settings
func NewContentFetcher() *ContentFetcher {
	return &ContentFetcher{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		MaxWorkers: 10,
		MinLength:  100,
	}
}

// contentResult holds the result of a single fetch operation
type contentResult struct {
	DocID   int64
	URL     string
	Content string
	Err     error
}

// FetchOne fetches and extracts content from a single URL
func (f *ContentFetcher) FetchOne(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MimirBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse with readability
	article, err := readability.FromReader(strings.NewReader(string(body)), nil)
	if err != nil {
		return "", err
	}

	// Get text content
	content := strings.TrimSpace(article.TextContent)
	if len(content) < f.MinLength {
		return "", fmt.Errorf("content too short: %d chars", len(content))
	}

	return content, nil
}

// BatchFetch fetches content for documents missing content in the database
func (f *ContentFetcher) BatchFetch(d *db.DB, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}

	// Query documents without content (exclude YouTube and Google URLs)
	rows, err := d.Query(`
		SELECT id, url FROM documents
		WHERE (content IS NULL OR content = '')
		AND url NOT LIKE '%youtube%'
		AND url NOT LIKE '%google.com%'
		ORDER BY indexed_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// Collect documents to process
	type doc struct {
		ID  int64
		URL string
	}
	var docs []doc

	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.ID, &d.URL); err != nil {
			continue
		}
		docs = append(docs, d)
	}

	if len(docs) == 0 {
		return 0, nil
	}

	fmt.Printf("Target: %d documents\n", len(docs))

	// Process concurrently
	results := make(chan contentResult, len(docs))
	var wg sync.WaitGroup

	// Semaphore for limiting concurrent workers
	sem := make(chan struct{}, f.MaxWorkers)

	for _, doc := range docs {
		wg.Add(1)
		go func(docID int64, url string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			content, err := f.FetchOne(url)
			results <- contentResult{
				DocID:   docID,
				URL:     url,
				Content: content,
				Err:     err,
			}
		}(doc.ID, doc.URL)
	}

	// Close results channel when all workers done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Update database with results
	stmt, err := d.Prepare("UPDATE documents SET content = ? WHERE id = ?")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	success := 0
	for result := range results {
		if result.Err != nil || result.Content == "" {
			continue
		}

		if _, err := stmt.Exec(result.Content, result.DocID); err != nil {
			continue
		}

		success++
		if success%10 == 0 {
			fmt.Printf("  %d completed...\n", success)
		}
	}

	fmt.Printf("Done: %d/%d succeeded\n", success, len(docs))
	return success, nil
}

// FetchForDocument fetches content for a single document by ID
func (f *ContentFetcher) FetchForDocument(d *db.DB, docID int64) error {
	var url string
	err := d.QueryRow("SELECT url FROM documents WHERE id = ?", docID).Scan(&url)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("document %d not found", docID)
		}
		return err
	}

	content, err := f.FetchOne(url)
	if err != nil {
		return err
	}

	_, err = d.Exec("UPDATE documents SET content = ? WHERE id = ?", content, docID)
	return err
}
