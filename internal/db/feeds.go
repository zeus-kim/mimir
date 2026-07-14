package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Feed represents a single RSS/YouTube feed entry
type Feed struct {
	ID           int64   `json:"id"`
	URL          string  `json:"url"`
	Domain       string  `json:"domain"`
	Title        string  `json:"title,omitempty"`
	Category     string  `json:"category"`
	Language     string  `json:"language"`
	Country      string  `json:"country"`
	FeedType     string  `json:"feed_type"`
	Status       string  `json:"status"`
	QualityScore float64 `json:"quality_score"`
	Source       string  `json:"source,omitempty"`
	CreatedAt    int64   `json:"created_at"`
}

// FeedStats holds aggregate statistics for feeds
type FeedStats struct {
	Total           int            `json:"total"`
	ByLanguage      map[string]int `json:"by_language"`
	ByCountry       map[string]int `json:"by_country"`
	ByCategory      map[string]int `json:"by_category"`
	ByStatus        map[string]int `json:"by_status"`
	ByFeedType      map[string]int `json:"by_feed_type"`
	ActiveCount     int            `json:"active_count"`
	InactiveCount   int            `json:"inactive_count"`
	AvgQualityScore float64        `json:"avg_quality_score"`
}

// TLD to country mapping
var tldToCountry = map[string]string{
	"kr": "kr", "co.kr": "kr", "jp": "jp", "co.jp": "jp",
	"cn": "cn", "tw": "tw", "hk": "hk", "vn": "vn", "th": "th",
	"de": "de", "fr": "fr", "es": "es", "it": "it", "nl": "nl",
	"uk": "uk", "co.uk": "uk", "ru": "ru", "pl": "pl",
	"br": "br", "com.br": "br", "mx": "mx", "ar": "ar",
	"au": "au", "com.au": "au", "in": "in", "co.in": "in",
}

// Country to language mapping
var countryToLang = map[string]string{
	"kr": "ko", "jp": "ja", "cn": "zh", "tw": "zh", "hk": "zh",
	"vn": "vi", "th": "th", "de": "de", "fr": "fr", "es": "es",
	"it": "it", "nl": "nl", "ru": "ru", "pl": "pl", "br": "pt",
	"pt": "pt", "uk": "en", "us": "en", "au": "en", "in": "en",
}

const feedsSchema = `
CREATE TABLE IF NOT EXISTS feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,
    domain TEXT NOT NULL,
    title TEXT,
    category TEXT DEFAULT 'other',
    language TEXT DEFAULT 'unknown',
    country TEXT DEFAULT 'unknown',
    feed_type TEXT DEFAULT 'rss',
    status TEXT DEFAULT 'active',
    quality_score REAL DEFAULT 50,
    source TEXT,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_feeds_lang ON feeds(language);
CREATE INDEX IF NOT EXISTS idx_feeds_country ON feeds(country);
CREATE INDEX IF NOT EXISTS idx_feeds_category ON feeds(category);
CREATE INDEX IF NOT EXISTS idx_feeds_domain ON feeds(domain);
CREATE INDEX IF NOT EXISTS idx_feeds_status ON feeds(status);
`

// EnsureFeedsSchema creates the feeds table and indexes if they don't exist
func (d *DB) EnsureFeedsSchema() error {
	_, err := d.Exec(feedsSchema)
	if err != nil {
		return fmt.Errorf("failed to create feeds schema: %w", err)
	}
	return nil
}

// GetDomain extracts the domain (netloc) from a URL
func GetDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// GetCountryFromDomain infers country code from domain TLD
func GetCountryFromDomain(domain string) string {
	parts := strings.Split(strings.ToLower(domain), ".")
	if len(parts) < 1 {
		return "global"
	}

	// Try two-part TLD first (e.g., co.kr, com.br)
	if len(parts) >= 2 {
		twoPartTLD := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if country, ok := tldToCountry[twoPartTLD]; ok {
			return country
		}
	}

	// Try single TLD
	singleTLD := parts[len(parts)-1]
	if country, ok := tldToCountry[singleTLD]; ok {
		return country
	}

	return "global"
}

// GetLanguageFromCountry maps country code to language code
func GetLanguageFromCountry(country string) string {
	if lang, ok := countryToLang[country]; ok {
		return lang
	}
	return "en"
}

// InsertFeed adds a new feed, ignoring if URL already exists
func (d *DB) InsertFeed(feed *Feed) error {
	if feed.CreatedAt == 0 {
		feed.CreatedAt = time.Now().Unix()
	}
	if feed.Domain == "" {
		feed.Domain = GetDomain(feed.URL)
	}
	if feed.Category == "" {
		feed.Category = "other"
	}
	if feed.Language == "" {
		feed.Language = "unknown"
	}
	if feed.Country == "" {
		feed.Country = "unknown"
	}
	if feed.FeedType == "" {
		feed.FeedType = "rss"
	}
	if feed.Status == "" {
		feed.Status = "active"
	}
	if feed.QualityScore == 0 {
		feed.QualityScore = 50
	}

	_, err := d.Exec(`
		INSERT OR IGNORE INTO feeds(url, domain, title, category, language, country, feed_type, status, quality_score, source, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, feed.URL, feed.Domain, feed.Title, feed.Category, feed.Language, feed.Country,
		feed.FeedType, feed.Status, feed.QualityScore, feed.Source, feed.CreatedAt)

	return err
}

// UpsertFeed inserts or updates a feed by URL
func (d *DB) UpsertFeed(feed *Feed) error {
	if feed.CreatedAt == 0 {
		feed.CreatedAt = time.Now().Unix()
	}
	if feed.Domain == "" {
		feed.Domain = GetDomain(feed.URL)
	}
	if feed.QualityScore == 0 {
		feed.QualityScore = 50
	}

	_, err := d.Exec(`
		INSERT INTO feeds(url, domain, title, category, language, country, feed_type, status, quality_score, source, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			title = COALESCE(excluded.title, feeds.title),
			category = COALESCE(excluded.category, feeds.category),
			language = COALESCE(excluded.language, feeds.language),
			country = COALESCE(excluded.country, feeds.country),
			quality_score = excluded.quality_score,
			status = excluded.status
	`, feed.URL, feed.Domain, feed.Title, feed.Category, feed.Language, feed.Country,
		feed.FeedType, feed.Status, feed.QualityScore, feed.Source, feed.CreatedAt)

	return err
}

// InsertFeeds bulk inserts multiple feeds efficiently
func (d *DB) InsertFeeds(feeds []*Feed) (inserted int, err error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO feeds(url, domain, title, category, language, country, feed_type, status, quality_score, source, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, feed := range feeds {
		if feed.CreatedAt == 0 {
			feed.CreatedAt = now
		}
		if feed.Domain == "" {
			feed.Domain = GetDomain(feed.URL)
		}
		if feed.QualityScore == 0 {
			feed.QualityScore = 50
		}

		result, err := stmt.Exec(
			feed.URL, feed.Domain, feed.Title, feed.Category, feed.Language, feed.Country,
			feed.FeedType, feed.Status, feed.QualityScore, feed.Source, feed.CreatedAt,
		)
		if err != nil {
			continue
		}
		if n, _ := result.RowsAffected(); n > 0 {
			inserted++
		}
	}

	return inserted, tx.Commit()
}

// UpdateFeedStatus updates the status of a feed by URL
func (d *DB) UpdateFeedStatus(feedURL, status string) error {
	result, err := d.Exec("UPDATE feeds SET status = ? WHERE url = ?", status, feedURL)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateFeedQuality updates the quality score of a feed
func (d *DB) UpdateFeedQuality(feedURL string, score float64) error {
	result, err := d.Exec("UPDATE feeds SET quality_score = ? WHERE url = ?", score, feedURL)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetFeed retrieves a single feed by URL
func (d *DB) GetFeed(feedURL string) (*Feed, error) {
	feed := &Feed{}
	err := d.QueryRow(`
		SELECT id, url, domain, title, category, language, country, feed_type, status, quality_score, source, created_at
		FROM feeds WHERE url = ?
	`, feedURL).Scan(
		&feed.ID, &feed.URL, &feed.Domain, &feed.Title, &feed.Category, &feed.Language,
		&feed.Country, &feed.FeedType, &feed.Status, &feed.QualityScore, &feed.Source, &feed.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return feed, nil
}

// ListFeeds retrieves feeds with optional filtering
func (d *DB) ListFeeds(language, country, category, status string, limit int) ([]*Feed, error) {
	query := "SELECT id, url, domain, title, category, language, country, feed_type, status, quality_score, source, created_at FROM feeds WHERE 1=1"
	args := []interface{}{}

	if language != "" {
		query += " AND language = ?"
		args = append(args, language)
	}
	if country != "" {
		query += " AND country = ?"
		args = append(args, country)
	}
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	query += " ORDER BY quality_score DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feeds []*Feed
	for rows.Next() {
		feed := &Feed{}
		var title, source sql.NullString
		err := rows.Scan(
			&feed.ID, &feed.URL, &feed.Domain, &title, &feed.Category, &feed.Language,
			&feed.Country, &feed.FeedType, &feed.Status, &feed.QualityScore, &source, &feed.CreatedAt,
		)
		if err != nil {
			continue
		}
		feed.Title = title.String
		feed.Source = source.String
		feeds = append(feeds, feed)
	}

	return feeds, nil
}

// DeleteFeed removes a feed by URL
func (d *DB) DeleteFeed(feedURL string) error {
	result, err := d.Exec("DELETE FROM feeds WHERE url = ?", feedURL)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetFeedStats returns aggregate statistics about feeds
func (d *DB) GetFeedStats() (*FeedStats, error) {
	stats := &FeedStats{
		ByLanguage: make(map[string]int),
		ByCountry:  make(map[string]int),
		ByCategory: make(map[string]int),
		ByStatus:   make(map[string]int),
		ByFeedType: make(map[string]int),
	}

	// Total count
	if err := d.QueryRow("SELECT COUNT(*) FROM feeds").Scan(&stats.Total); err != nil {
		return nil, err
	}

	if stats.Total == 0 {
		return stats, nil
	}

	// Average quality score
	d.QueryRow("SELECT AVG(quality_score) FROM feeds").Scan(&stats.AvgQualityScore)

	// By language
	rows, err := d.Query("SELECT language, COUNT(*) FROM feeds GROUP BY language ORDER BY COUNT(*) DESC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lang string
			var count int
			if rows.Scan(&lang, &count) == nil {
				stats.ByLanguage[lang] = count
			}
		}
	}

	// By country
	rows, err = d.Query("SELECT country, COUNT(*) FROM feeds GROUP BY country ORDER BY COUNT(*) DESC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var country string
			var count int
			if rows.Scan(&country, &count) == nil {
				stats.ByCountry[country] = count
			}
		}
	}

	// By category
	rows, err = d.Query("SELECT category, COUNT(*) FROM feeds GROUP BY category ORDER BY COUNT(*) DESC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var count int
			if rows.Scan(&cat, &count) == nil {
				stats.ByCategory[cat] = count
			}
		}
	}

	// By status
	rows, err = d.Query("SELECT status, COUNT(*) FROM feeds GROUP BY status")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if rows.Scan(&status, &count) == nil {
				stats.ByStatus[status] = count
				if status == "active" {
					stats.ActiveCount = count
				} else if status == "inactive" {
					stats.InactiveCount = count
				}
			}
		}
	}

	// By feed type
	rows, err = d.Query("SELECT feed_type, COUNT(*) FROM feeds GROUP BY feed_type")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var feedType string
			var count int
			if rows.Scan(&feedType, &count) == nil {
				stats.ByFeedType[feedType] = count
			}
		}
	}

	return stats, nil
}

// PruneLowQualityFeeds removes feeds below a quality threshold
func (d *DB) PruneLowQualityFeeds(minScore float64) (int64, error) {
	result, err := d.Exec("DELETE FROM feeds WHERE quality_score < ?", minScore)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeactivateStaleFeeds marks feeds as inactive if they haven't been updated
func (d *DB) DeactivateStaleFeeds(olderThanUnix int64) (int64, error) {
	result, err := d.Exec(
		"UPDATE feeds SET status = 'inactive' WHERE created_at < ? AND status = 'active'",
		olderThanUnix,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
