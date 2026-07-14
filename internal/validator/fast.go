package validator

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RSS/Atom content markers
var feedMarkers = [][]byte{
	[]byte("<rss"),
	[]byte("<feed"),
	[]byte("<channel"),
	[]byte("<?xml"),
	[]byte("<atom"),
}

// FeedCheckResult holds the result of a single URL validation
type FeedCheckResult struct {
	URL     string
	Valid   bool
	Latency time.Duration
	Error   error
}

// ValidationStats tracks overall validation progress
type ValidationStats struct {
	Total     int64
	Valid     int64
	Failed    int64
	StartTime time.Time
}

// Rate returns validations per second
func (s *ValidationStats) Rate() float64 {
	elapsed := time.Since(s.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&s.Total)) / elapsed
}

// ValidPercent returns percentage of valid feeds
func (s *ValidationStats) ValidPercent() int {
	total := atomic.LoadInt64(&s.Total)
	if total == 0 {
		return 0
	}
	return int(atomic.LoadInt64(&s.Valid) * 100 / total)
}

// FastValidator performs concurrent HTTP validation of feed URLs
type FastValidator struct {
	Workers     int
	Timeout     time.Duration
	UserAgent   string
	MaxBodyRead int

	client *http.Client
	cache  sync.Map // URL -> bool
}

// NewFastValidator creates a validator with default settings
func NewFastValidator(workers int) *FastValidator {
	if workers <= 0 {
		workers = 500
	}

	timeout := 3 * time.Second

	return &FastValidator{
		Workers:     workers,
		Timeout:     timeout,
		UserAgent:   "Mozilla/5.0 FeedValidator/1.0",
		MaxBodyRead: 1024, // Read first 1KB to check markers

		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        workers,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// ValidateURL checks if a single URL returns valid RSS/Atom content
func (v *FastValidator) ValidateURL(ctx context.Context, url string) FeedCheckResult {
	start := time.Now()

	// Check cache first
	if cached, ok := v.cache.Load(url); ok {
		return FeedCheckResult{
			URL:     url,
			Valid:   cached.(bool),
			Latency: time.Since(start),
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return FeedCheckResult{URL: url, Valid: false, Error: err, Latency: time.Since(start)}
	}
	req.Header.Set("User-Agent", v.UserAgent)

	resp, err := v.client.Do(req)
	if err != nil {
		return FeedCheckResult{URL: url, Valid: false, Error: err, Latency: time.Since(start)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FeedCheckResult{
			URL:     url,
			Valid:   false,
			Error:   fmt.Errorf("status %d", resp.StatusCode),
			Latency: time.Since(start),
		}
	}

	// Read first chunk to check for feed markers
	buf := make([]byte, v.MaxBodyRead)
	n, err := io.ReadAtLeast(resp.Body, buf, 100)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		// Still check what we got if we read something
		if n == 0 {
			return FeedCheckResult{URL: url, Valid: false, Error: err, Latency: time.Since(start)}
		}
	}

	content := bytes.ToLower(buf[:n])
	valid := false
	for _, marker := range feedMarkers {
		if bytes.Contains(content, marker) {
			valid = true
			break
		}
	}

	// Cache the result
	v.cache.Store(url, valid)

	return FeedCheckResult{
		URL:     url,
		Valid:   valid,
		Latency: time.Since(start),
	}
}

// ValidateBatch validates multiple URLs concurrently
func (v *FastValidator) ValidateBatch(ctx context.Context, urls []string) []FeedCheckResult {
	results := make([]FeedCheckResult, len(urls))
	var wg sync.WaitGroup

	// Semaphore to limit concurrent workers
	sem := make(chan struct{}, v.Workers)

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			results[idx] = v.ValidateURL(ctx, u)
		}(i, url)
	}

	wg.Wait()
	return results
}

// ValidateStream processes URLs from a channel and sends results to another
func (v *FastValidator) ValidateStream(ctx context.Context, urls <-chan string, results chan<- FeedCheckResult) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, v.Workers)

	for url := range urls {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := v.ValidateURL(ctx, u)
			select {
			case results <- result:
			case <-ctx.Done():
			}
		}(url)
	}

	wg.Wait()
}

// ValidateFile reads URLs from input file and writes valid ones to output file
// Returns stats about the validation process
func (v *FastValidator) ValidateFile(ctx context.Context, inputPath, outputPath string, progress func(stats *ValidationStats)) (*ValidationStats, error) {
	// Open input file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	defer inFile.Close()

	// Open output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	// Read all URLs first
	var urls []string
	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
			urls = append(urls, url)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan input: %w", err)
	}

	stats := &ValidationStats{StartTime: time.Now()}

	// Process in batches
	batchSize := v.Workers * 2
	var mu sync.Mutex

	for i := 0; i < len(urls); i += batchSize {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(urls) {
			end = len(urls)
		}
		batch := urls[i:end]

		results := v.ValidateBatch(ctx, batch)

		for _, r := range results {
			atomic.AddInt64(&stats.Total, 1)
			if r.Valid {
				atomic.AddInt64(&stats.Valid, 1)
				mu.Lock()
				writer.WriteString(r.URL + "\n")
				mu.Unlock()
			} else {
				atomic.AddInt64(&stats.Failed, 1)
			}
		}

		// Flush periodically
		if stats.Total%1000 == 0 {
			mu.Lock()
			writer.Flush()
			mu.Unlock()
		}

		// Report progress
		if progress != nil && stats.Total%1000 == 0 {
			progress(stats)
		}
	}

	writer.Flush()
	return stats, nil
}

// ClearCache removes all cached results
func (v *FastValidator) ClearCache() {
	v.cache = sync.Map{}
}

// CacheSize returns approximate number of cached entries
func (v *FastValidator) CacheSize() int {
	count := 0
	v.cache.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// IsCached checks if a URL result is in the cache
func (v *FastValidator) IsCached(url string) (valid bool, found bool) {
	if cached, ok := v.cache.Load(url); ok {
		return cached.(bool), true
	}
	return false, false
}
