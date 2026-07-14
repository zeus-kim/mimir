package db

// #cgo CFLAGS: -DSQLITE_ENABLE_FTS5
import "C"

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
	Path string
}

func Open(path string) (*DB, error) {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".mine", "lite.db")
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	return &DB{DB: db, Path: path}, nil
}

func (d *DB) EnsureSchema() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS clinical_trials (
			nct_id TEXT PRIMARY KEY,
			title TEXT,
			sponsor TEXT,
			phase TEXT,
			status TEXT,
			conditions TEXT,
			interventions TEXT,
			brief_summary TEXT,
			start_date TEXT,
			completion_date TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS clinical_trials_fts USING fts5(
			nct_id, title, sponsor, conditions, interventions, brief_summary
		)`,
		`CREATE TABLE IF NOT EXISTS pubmed_articles (
			pmid TEXT PRIMARY KEY,
			title TEXT,
			abstract TEXT,
			authors TEXT,
			journal TEXT,
			journal_abbrev TEXT,
			pub_year INTEGER,
			pub_date TEXT,
			doi TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS pubmed_fts USING fts5(
			pmid, title, abstract, authors, journal
		)`,
		`CREATE TABLE IF NOT EXISTS fda_approvals (
			application_number TEXT PRIMARY KEY,
			brand_name TEXT,
			generic_name TEXT,
			sponsor_name TEXT,
			approval_date TEXT,
			submission_type TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS fda_adverse_events (
			report_id TEXT PRIMARY KEY,
			drug_names TEXT,
			reactions TEXT,
			serious INTEGER,
			receive_date TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS sec_filings (
			accession_number TEXT PRIMARY KEY,
			company_name TEXT,
			ticker TEXT,
			form_type TEXT,
			filed_date TEXT,
			description TEXT,
			url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		// AI/ML Research tables
		`CREATE TABLE IF NOT EXISTS ai_papers (
			paper_id TEXT PRIMARY KEY,
			source TEXT,
			title TEXT,
			abstract TEXT,
			authors TEXT,
			categories TEXT,
			published TEXT,
			updated TEXT,
			pdf_link TEXT,
			citation_count INTEGER DEFAULT 0,
			influential_citations INTEGER DEFAULT 0,
			venue TEXT,
			proceeding TEXT,
			arxiv_id TEXT,
			url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS ai_papers_fts USING fts5(
			paper_id, title, abstract, authors, categories
		)`,
		`CREATE TABLE IF NOT EXISTS ai_models (
			model_id TEXT PRIMARY KEY,
			source TEXT,
			author TEXT,
			downloads INTEGER DEFAULT 0,
			likes INTEGER DEFAULT 0,
			tags TEXT,
			pipeline_tag TEXT,
			library_name TEXT,
			last_updated TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS ai_models_fts USING fts5(
			model_id, author, tags, pipeline_tag
		)`,
		// Legal domain tables
		`CREATE TABLE IF NOT EXISTS legal_cases (
			case_id TEXT PRIMARY KEY,
			case_name TEXT,
			case_name_full TEXT,
			court TEXT,
			court_id TEXT,
			date_filed TEXT,
			citation TEXT,
			snippet TEXT,
			status TEXT,
			source TEXT,
			url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS legal_cases_fts USING fts5(
			case_id, case_name, court, snippet
		)`,
		`CREATE TABLE IF NOT EXISTS legislation (
			bill_id TEXT PRIMARY KEY,
			bill_number TEXT,
			title TEXT,
			congress TEXT,
			chamber TEXT,
			sponsor TEXT,
			sponsor_party TEXT,
			sponsor_state TEXT,
			policy_area TEXT,
			latest_action TEXT,
			latest_action_date TEXT,
			introduced_date TEXT,
			cosponsors_count INTEGER DEFAULT 0,
			jurisdiction TEXT,
			source TEXT,
			url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS legislation_fts USING fts5(
			bill_id, bill_number, title, sponsor, policy_area, latest_action
		)`,
		`CREATE TABLE IF NOT EXISTS federal_register (
			document_number TEXT PRIMARY KEY,
			title TEXT,
			abstract TEXT,
			document_type TEXT,
			agencies TEXT,
			publication_date TEXT,
			effective_date TEXT,
			citation TEXT,
			topics TEXT,
			cfr_references TEXT,
			url TEXT,
			pdf_url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS federal_register_fts USING fts5(
			document_number, title, abstract, agencies, topics
		)`,
		// Food domain tables
		`CREATE TABLE IF NOT EXISTS food_items (
			fdc_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			brand TEXT,
			data_type TEXT,
			serving_size REAL,
			serving_unit TEXT,
			calories REAL,
			protein_g REAL,
			fat_g REAL,
			carbs_g REAL,
			fiber_g REAL,
			sugar_g REAL,
			sodium_mg REAL,
			ingredients TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS food_items_fts USING fts5(
			fdc_id, name, brand, ingredients
		)`,
		`CREATE TABLE IF NOT EXISTS recipes (
			recipe_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT,
			cuisine TEXT,
			instructions TEXT,
			ingredients TEXT,
			tags TEXT,
			image_url TEXT,
			video_url TEXT,
			source_url TEXT,
			source_type TEXT,
			servings INTEGER,
			prep_time_min INTEGER,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS recipes_fts USING fts5(
			recipe_id, name, category, cuisine, ingredients, tags
		)`,
		// Finance domain tables
		`CREATE TABLE IF NOT EXISTS economic_data (
			series_id TEXT,
			title TEXT,
			observation_date TEXT,
			value REAL,
			units TEXT,
			frequency TEXT,
			seasonal_adjustment TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (series_id, observation_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_economic_data_series ON economic_data(series_id)`,
		`CREATE INDEX IF NOT EXISTS idx_economic_data_date ON economic_data(observation_date)`,
		`CREATE TABLE IF NOT EXISTS stock_quotes (
			symbol TEXT PRIMARY KEY,
			name TEXT,
			price REAL,
			change REAL,
			change_percent REAL,
			volume INTEGER,
			market_cap INTEGER,
			fifty_two_week_high REAL,
			fifty_two_week_low REAL,
			pe_ratio REAL,
			dividend_yield REAL,
			currency TEXT,
			exchange TEXT,
			fetched_at TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS stock_daily_prices (
			symbol TEXT,
			date TEXT,
			open REAL,
			high REAL,
			low REAL,
			close REAL,
			volume INTEGER,
			created_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (symbol, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_daily_symbol ON stock_daily_prices(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_stock_daily_date ON stock_daily_prices(date)`,
		`CREATE TABLE IF NOT EXISTS korean_disclosures (
			rcept_no TEXT PRIMARY KEY,
			corp_code TEXT,
			corp_name TEXT,
			corp_cls TEXT,
			report_nm TEXT,
			flr_nm TEXT,
			rcept_dt TEXT,
			rm TEXT,
			url TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS korean_disclosures_fts USING fts5(
			rcept_no, corp_name, report_nm, flr_nm
		)`,
		`CREATE INDEX IF NOT EXISTS idx_korean_disclosures_corp ON korean_disclosures(corp_code)`,
		`CREATE INDEX IF NOT EXISTS idx_korean_disclosures_date ON korean_disclosures(rcept_dt)`,
	}

	for _, schema := range schemas {
		if _, err := d.Exec(schema); err != nil {
			return fmt.Errorf("schema error: %w", err)
		}
	}

	return nil
}

func (d *DB) Stats() (map[string]int, error) {
	stats := make(map[string]int)
	tables := []string{
		"clinical_trials", "pubmed_articles", "fda_approvals", "fda_adverse_events", "sec_filings",
		"ai_papers", "ai_models",
		"legal_cases", "legislation", "federal_register",
		"food_items", "recipes",
		"economic_data", "stock_quotes", "stock_daily_prices", "korean_disclosures",
	}

	for _, table := range tables {
		var count int
		row := d.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
		if err := row.Scan(&count); err == nil {
			stats[table] = count
		}
	}

	return stats, nil
}
