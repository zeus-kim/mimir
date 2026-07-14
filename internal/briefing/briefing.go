package briefing

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/db"
)

// Language represents the output language for briefings
type Language string

const (
	Korean  Language = "ko"
	English Language = "en"
)

// Statement represents a political/news statement with source info
type Statement struct {
	Actor    string
	Topic    string
	Claim    string
	Stance   string
	Source   string
	SourceID string
}

// SourceSummary contains categorized statements by source type
type SourceSummary struct {
	Government   []Statement
	Conservative []Statement
	Progressive  []Statement
}

// Briefing contains the complete briefing output
type Briefing struct {
	GeneratedAt   time.Time
	Language      Language
	TotalCount    int
	SourceSummary SourceSummary
	Statements    []Statement
}

// BriefingConfig configures the briefing generation
type BriefingConfig struct {
	DBPath        string
	Language      Language
	HoursBack     int
	MaxStatements int
}

// DefaultConfig returns a default configuration
func DefaultConfig() BriefingConfig {
	return BriefingConfig{
		Language:      Korean,
		HoursBack:     24,
		MaxStatements: 50,
	}
}

// Generator creates briefings from database content
type Generator struct {
	db     *db.DB
	config BriefingConfig
}

// NewGenerator creates a new briefing generator
func NewGenerator(database *db.DB, config BriefingConfig) *Generator {
	return &Generator{
		db:     database,
		config: config,
	}
}

// mapURLToSource converts a URL to a human-readable source name
func (g *Generator) mapURLToSource(url, feedName string) string {
	if g.config.Language == Korean {
		return g.mapURLToSourceKo(url, feedName)
	}
	return g.mapURLToSourceEn(url, feedName)
}

func (g *Generator) mapURLToSourceKo(url, feedName string) string {
	switch {
	case strings.Contains(url, "korea.kr"):
		return "정책브리핑"
	case strings.Contains(url, "youtube") && feedName != "":
		return feedName
	case strings.Contains(url, "youtube"):
		return "유튜브"
	case strings.Contains(url, "yna.co.kr"):
		return "연합뉴스"
	case strings.Contains(url, "hani.co.kr"):
		return "한겨레"
	case strings.Contains(url, "khan.co.kr"):
		return "경향신문"
	case strings.Contains(url, "chosun"):
		return "조선일보"
	case strings.Contains(url, "donga"):
		return "동아일보"
	case strings.Contains(url, "joongang"):
		return "중앙일보"
	default:
		return "뉴스"
	}
}

func (g *Generator) mapURLToSourceEn(url, feedName string) string {
	switch {
	case strings.Contains(url, "korea.kr"):
		return "Government Policy"
	case strings.Contains(url, "youtube") && feedName != "":
		return feedName
	case strings.Contains(url, "youtube"):
		return "YouTube"
	case strings.Contains(url, "yna.co.kr"):
		return "Yonhap News"
	case strings.Contains(url, "hani.co.kr"):
		return "Hankyoreh"
	case strings.Contains(url, "khan.co.kr"):
		return "Kyunghyang"
	case strings.Contains(url, "chosun"):
		return "Chosun Ilbo"
	case strings.Contains(url, "donga"):
		return "Donga Ilbo"
	case strings.Contains(url, "joongang"):
		return "JoongAng Ilbo"
	default:
		return "News"
	}
}

// GetStatementsWithSources fetches statements with source attribution
func (g *Generator) GetStatementsWithSources() ([]Statement, error) {
	cutoff := time.Now().Unix() - int64(g.config.HoursBack*3600)
	limit := g.config.MaxStatements
	if limit == 0 {
		limit = 50
	}

	query := `
		SELECT
			ps.actor, ps.topic, ps.claim, ps.stance,
			d.url,
			COALESCE(f.name, '') as feed_name
		FROM political_statements ps
		JOIN documents d ON ps.source_url = d.url
		LEFT JOIN feeds f ON d.url LIKE '%' ||
			CASE WHEN f.url LIKE '%channel_id=%'
				THEN SUBSTR(f.url, INSTR(f.url, 'channel_id=') + 11, 24)
				ELSE '' END || '%'
		WHERE ps.created_at > ?
		ORDER BY ps.created_at DESC
		LIMIT ?
	`

	rows, err := g.db.Query(query, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("query statements: %w", err)
	}
	defer rows.Close()

	var statements []Statement
	for rows.Next() {
		var actor, topic, claim, stance, url, feedName string
		if err := rows.Scan(&actor, &topic, &claim, &stance, &url, &feedName); err != nil {
			continue
		}
		statements = append(statements, Statement{
			Actor:    actor,
			Topic:    topic,
			Claim:    claim,
			Stance:   stance,
			Source:   g.mapURLToSource(url, feedName),
			SourceID: url,
		})
	}

	return statements, rows.Err()
}

// GetSourceSummary fetches categorized content summaries
func (g *Generator) GetSourceSummary() (SourceSummary, error) {
	cutoff := time.Now().Unix() - int64(g.config.HoursBack*3600)
	summary := SourceSummary{}

	// Government sources
	govt, err := g.fetchByURLPattern("%korea.kr%", cutoff, 5)
	if err != nil {
		return summary, fmt.Errorf("fetch government: %w", err)
	}
	summary.Government = govt

	// Conservative YouTube
	conservative, err := g.fetchByCategory("보수", cutoff, 5)
	if err != nil {
		return summary, fmt.Errorf("fetch conservative: %w", err)
	}
	summary.Conservative = conservative

	// Progressive YouTube
	progressive, err := g.fetchByCategory("진보", cutoff, 5)
	if err != nil {
		return summary, fmt.Errorf("fetch progressive: %w", err)
	}
	summary.Progressive = progressive

	return summary, nil
}

func (g *Generator) fetchByURLPattern(pattern string, cutoff int64, limit int) ([]Statement, error) {
	query := `
		SELECT ps.actor, ps.topic, ps.claim, d.url
		FROM political_statements ps
		JOIN documents d ON ps.source_url = d.url
		WHERE d.url LIKE ?
		AND ps.created_at > ?
		ORDER BY ps.created_at DESC
		LIMIT ?
	`

	rows, err := g.db.Query(query, pattern, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statements []Statement
	for rows.Next() {
		var actor, topic, claim, url string
		if err := rows.Scan(&actor, &topic, &claim, &url); err != nil {
			continue
		}
		statements = append(statements, Statement{
			Actor:  actor,
			Topic:  topic,
			Claim:  claim,
			Source: g.mapURLToSource(url, ""),
		})
	}
	return statements, rows.Err()
}

func (g *Generator) fetchByCategory(category string, cutoff int64, limit int) ([]Statement, error) {
	query := `
		SELECT f.name, ps.actor, ps.claim, d.url
		FROM political_statements ps
		JOIN documents d ON ps.source_url = d.url
		JOIN feeds f ON d.url LIKE '%youtube%' AND f.category = ?
		WHERE ps.created_at > ?
		ORDER BY ps.created_at DESC
		LIMIT ?
	`

	rows, err := g.db.Query(query, category, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statements []Statement
	for rows.Next() {
		var feedName, actor, claim, url string
		if err := rows.Scan(&feedName, &actor, &claim, &url); err != nil {
			continue
		}
		statements = append(statements, Statement{
			Actor:  actor,
			Claim:  claim,
			Source: feedName,
		})
	}
	return statements, rows.Err()
}

// Generate creates a complete briefing
func (g *Generator) Generate() (*Briefing, error) {
	summary, err := g.GetSourceSummary()
	if err != nil {
		return nil, fmt.Errorf("get source summary: %w", err)
	}

	statements, err := g.GetStatementsWithSources()
	if err != nil {
		return nil, fmt.Errorf("get statements: %w", err)
	}

	return &Briefing{
		GeneratedAt:   time.Now(),
		Language:      g.config.Language,
		TotalCount:    len(statements),
		SourceSummary: summary,
		Statements:    statements,
	}, nil
}

// FormatMarkdown formats the briefing as Markdown text
func (b *Briefing) FormatMarkdown() string {
	var sb strings.Builder

	if b.Language == Korean {
		sb.WriteString(fmt.Sprintf("# 브리핑 (%s)\n\n", b.GeneratedAt.Format("2006-01-02 15:04")))
		sb.WriteString("## 출처별 주요 내용\n\n")

		sb.WriteString("### 정부 공식 (정책브리핑)\n")
		if len(b.SourceSummary.Government) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Government, 3) {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Actor, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (최근 24시간 내 없음)\n")
		}

		sb.WriteString("\n### 보수 유튜브\n")
		if len(b.SourceSummary.Conservative) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Conservative, 3) {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", s.Source, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (최근 24시간 내 없음)\n")
		}

		sb.WriteString("\n### 진보 유튜브\n")
		if len(b.SourceSummary.Progressive) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Progressive, 3) {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", s.Source, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (최근 24시간 내 없음)\n")
		}

		sb.WriteString("\n## 주요 발언 (출처 포함)\n\n")
		seen := make(map[string]bool)
		count := 0
		for _, s := range b.Statements {
			if count >= 15 {
				break
			}
			key := s.Actor + ":" + s.Topic
			if seen[key] {
				continue
			}
			seen[key] = true
			sb.WriteString(fmt.Sprintf("- **[%s]** %s (%s): %s\n", s.Source, s.Actor, s.Stance, truncate(s.Claim, 60)))
			count++
		}

		sb.WriteString("\n---\n")
		sb.WriteString(fmt.Sprintf("총 %d건 발언 분석\n", b.TotalCount))
	} else {
		sb.WriteString(fmt.Sprintf("# Briefing (%s)\n\n", b.GeneratedAt.Format("2006-01-02 15:04")))
		sb.WriteString("## Summary by Source\n\n")

		sb.WriteString("### Government Official\n")
		if len(b.SourceSummary.Government) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Government, 3) {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Actor, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (No items in last 24 hours)\n")
		}

		sb.WriteString("\n### Conservative Media\n")
		if len(b.SourceSummary.Conservative) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Conservative, 3) {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", s.Source, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (No items in last 24 hours)\n")
		}

		sb.WriteString("\n### Progressive Media\n")
		if len(b.SourceSummary.Progressive) > 0 {
			for _, s := range limitStatements(b.SourceSummary.Progressive, 3) {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", s.Source, truncate(s.Claim, 80)))
			}
		} else {
			sb.WriteString("- (No items in last 24 hours)\n")
		}

		sb.WriteString("\n## Key Statements (with sources)\n\n")
		seen := make(map[string]bool)
		count := 0
		for _, s := range b.Statements {
			if count >= 15 {
				break
			}
			key := s.Actor + ":" + s.Topic
			if seen[key] {
				continue
			}
			seen[key] = true
			sb.WriteString(fmt.Sprintf("- **[%s]** %s (%s): %s\n", s.Source, s.Actor, s.Stance, truncate(s.Claim, 60)))
			count++
		}

		sb.WriteString("\n---\n")
		sb.WriteString(fmt.Sprintf("Total %d statements analyzed\n", b.TotalCount))
	}

	return sb.String()
}

// FormatPlain formats the briefing as plain text (for TTS/audio)
func (b *Briefing) FormatPlain() string {
	var sb strings.Builder

	if b.Language == Korean {
		sb.WriteString(fmt.Sprintf("브리핑입니다. %s 기준.\n\n", b.GeneratedAt.Format("1월 2일 15시")))

		if len(b.SourceSummary.Government) > 0 {
			sb.WriteString("정부 발표 내용입니다.\n")
			for _, s := range limitStatements(b.SourceSummary.Government, 2) {
				sb.WriteString(fmt.Sprintf("%s. %s.\n", s.Actor, truncate(s.Claim, 100)))
			}
			sb.WriteString("\n")
		}

		seen := make(map[string]bool)
		count := 0
		sb.WriteString("주요 발언입니다.\n")
		for _, s := range b.Statements {
			if count >= 5 {
				break
			}
			key := s.Actor + ":" + s.Topic
			if seen[key] {
				continue
			}
			seen[key] = true
			sb.WriteString(fmt.Sprintf("%s. %s.\n", s.Actor, truncate(s.Claim, 80)))
			count++
		}

		sb.WriteString(fmt.Sprintf("\n총 %d건의 발언을 분석했습니다.\n", b.TotalCount))
	} else {
		sb.WriteString(fmt.Sprintf("Briefing as of %s.\n\n", b.GeneratedAt.Format("January 2, 3 PM")))

		if len(b.SourceSummary.Government) > 0 {
			sb.WriteString("Government announcements.\n")
			for _, s := range limitStatements(b.SourceSummary.Government, 2) {
				sb.WriteString(fmt.Sprintf("%s. %s.\n", s.Actor, truncate(s.Claim, 100)))
			}
			sb.WriteString("\n")
		}

		seen := make(map[string]bool)
		count := 0
		sb.WriteString("Key statements.\n")
		for _, s := range b.Statements {
			if count >= 5 {
				break
			}
			key := s.Actor + ":" + s.Topic
			if seen[key] {
				continue
			}
			seen[key] = true
			sb.WriteString(fmt.Sprintf("%s said: %s.\n", s.Actor, truncate(s.Claim, 80)))
			count++
		}

		sb.WriteString(fmt.Sprintf("\nTotal of %d statements analyzed.\n", b.TotalCount))
	}

	return sb.String()
}

// truncate shortens a string to max length with ellipsis
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// limitStatements returns at most n statements
func limitStatements(statements []Statement, n int) []Statement {
	if len(statements) <= n {
		return statements
	}
	return statements[:n]
}

// GenerateBriefing is a convenience function for one-shot briefing generation
func GenerateBriefing(dbPath string, lang Language) (string, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	config := DefaultConfig()
	config.DBPath = dbPath
	config.Language = lang

	gen := NewGenerator(database, config)
	briefing, err := gen.Generate()
	if err != nil {
		return "", err
	}

	return briefing.FormatMarkdown(), nil
}

// RecentDocuments fetches recent documents for a general briefing (not politics-specific)
type RecentDocument struct {
	Title     string
	Summary   string
	URL       string
	Source    string
	FetchedAt time.Time
}

// GetRecentDocuments fetches recent documents from the database
func (g *Generator) GetRecentDocuments(limit int) ([]RecentDocument, error) {
	if limit == 0 {
		limit = 20
	}
	cutoff := time.Now().Unix() - int64(g.config.HoursBack*3600)

	query := `
		SELECT d.title, d.summary, d.url, COALESCE(f.name, 'Unknown'), d.fetched_at
		FROM documents d
		LEFT JOIN feeds f ON d.feed_id = f.id
		WHERE d.fetched_at > ?
		ORDER BY d.fetched_at DESC
		LIMIT ?
	`

	rows, err := g.db.Query(query, cutoff, limit)
	if err != nil {
		// Table might not exist - try alternate query
		return g.getRecentDocumentsAlt(limit, cutoff)
	}
	defer rows.Close()

	var docs []RecentDocument
	for rows.Next() {
		var doc RecentDocument
		var fetchedAt int64
		if err := rows.Scan(&doc.Title, &doc.Summary, &doc.URL, &doc.Source, &fetchedAt); err != nil {
			continue
		}
		doc.FetchedAt = time.Unix(fetchedAt, 0)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (g *Generator) getRecentDocumentsAlt(limit int, cutoff int64) ([]RecentDocument, error) {
	query := `
		SELECT title, summary, url, fetched_at
		FROM documents
		WHERE fetched_at > ?
		ORDER BY fetched_at DESC
		LIMIT ?
	`

	rows, err := g.db.Query(query, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []RecentDocument
	for rows.Next() {
		var doc RecentDocument
		var fetchedAt int64
		var urlStr sql.NullString
		if err := rows.Scan(&doc.Title, &doc.Summary, &urlStr, &fetchedAt); err != nil {
			continue
		}
		doc.URL = urlStr.String
		doc.Source = g.mapURLToSource(doc.URL, "")
		doc.FetchedAt = time.Unix(fetchedAt, 0)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// GenerateGeneralBriefing creates a briefing from recent documents (not politics-specific)
func (g *Generator) GenerateGeneralBriefing() (string, error) {
	docs, err := g.GetRecentDocuments(20)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	if g.config.Language == Korean {
		sb.WriteString(fmt.Sprintf("# 브리핑 (%s)\n\n", time.Now().Format("2006-01-02 15:04")))
		sb.WriteString("## 최근 콘텐츠\n\n")

		for _, doc := range docs {
			sb.WriteString(fmt.Sprintf("### %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("출처: %s | %s\n\n", doc.Source, doc.FetchedAt.Format("15:04")))
			if doc.Summary != "" {
				sb.WriteString(truncate(doc.Summary, 200) + "\n\n")
			}
		}

		sb.WriteString(fmt.Sprintf("\n---\n총 %d건\n", len(docs)))
	} else {
		sb.WriteString(fmt.Sprintf("# Briefing (%s)\n\n", time.Now().Format("2006-01-02 15:04")))
		sb.WriteString("## Recent Content\n\n")

		for _, doc := range docs {
			sb.WriteString(fmt.Sprintf("### %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Source: %s | %s\n\n", doc.Source, doc.FetchedAt.Format("3:04 PM")))
			if doc.Summary != "" {
				sb.WriteString(truncate(doc.Summary, 200) + "\n\n")
			}
		}

		sb.WriteString(fmt.Sprintf("\n---\nTotal %d items\n", len(docs)))
	}

	return sb.String(), nil
}
