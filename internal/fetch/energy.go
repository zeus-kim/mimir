package fetch

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/db"
)

// ============================================================================
// Database Schema for Energy Data
// ============================================================================

// EnsureEnergySchema creates the energy-related tables
func EnsureEnergySchema(d *db.DB) error {
	schemas := []string{
		// EIA time series data (oil, gas, electricity, renewables)
		`CREATE TABLE IF NOT EXISTS eia_series (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			series_id TEXT NOT NULL,
			name TEXT,
			description TEXT,
			units TEXT,
			frequency TEXT,
			source TEXT,
			period TEXT NOT NULL,
			value REAL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(series_id, period)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_eia_series_id ON eia_series(series_id)`,
		`CREATE INDEX IF NOT EXISTS idx_eia_period ON eia_series(period)`,

		// ERCOT real-time grid data (Texas)
		`CREATE TABLE IF NOT EXISTS ercot_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone TEXT NOT NULL,
			settlement_point TEXT,
			price_type TEXT NOT NULL,
			price REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(zone, price_type, timestamp)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ercot_zone ON ercot_prices(zone)`,
		`CREATE INDEX IF NOT EXISTS idx_ercot_timestamp ON ercot_prices(timestamp)`,

		`CREATE TABLE IF NOT EXISTS ercot_load (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			zone TEXT NOT NULL,
			load_mw REAL,
			forecast_mw REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(zone, timestamp)
		)`,

		`CREATE TABLE IF NOT EXISTS ercot_generation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fuel_type TEXT NOT NULL,
			generation_mw REAL,
			capacity_mw REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(fuel_type, timestamp)
		)`,

		// Korea Power Exchange (KPX) data
		`CREATE TABLE IF NOT EXISTS kpx_prices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			market_type TEXT NOT NULL,
			price_type TEXT NOT NULL,
			price REAL,
			unit TEXT,
			trade_date TEXT NOT NULL,
			trade_hour INTEGER,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(market_type, price_type, trade_date, trade_hour)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kpx_date ON kpx_prices(trade_date)`,

		`CREATE TABLE IF NOT EXISTS kpx_demand (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			demand_mw REAL,
			supply_mw REAL,
			reserve_mw REAL,
			reserve_rate REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(timestamp)
		)`,

		// ENTSO-E European grid data
		`CREATE TABLE IF NOT EXISTS entsoe_generation (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			country_code TEXT NOT NULL,
			production_type TEXT NOT NULL,
			generation_mw REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(country_code, production_type, timestamp)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entsoe_country ON entsoe_generation(country_code)`,
		`CREATE INDEX IF NOT EXISTS idx_entsoe_timestamp ON entsoe_generation(timestamp)`,

		`CREATE TABLE IF NOT EXISTS entsoe_load (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			country_code TEXT NOT NULL,
			load_mw REAL,
			forecast_mw REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(country_code, timestamp)
		)`,

		`CREATE TABLE IF NOT EXISTS entsoe_crossborder (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_country TEXT NOT NULL,
			to_country TEXT NOT NULL,
			flow_mw REAL,
			scheduled_mw REAL,
			timestamp TEXT NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(from_country, to_country, timestamp)
		)`,

		// FTS for energy data search
		`CREATE VIRTUAL TABLE IF NOT EXISTS energy_fts USING fts5(
			source, series_id, name, description, units
		)`,
	}

	for _, schema := range schemas {
		if _, err := d.Exec(schema); err != nil {
			return fmt.Errorf("energy schema error: %w", err)
		}
	}

	return nil
}

// ============================================================================
// EIA API Fetcher (US Energy Information Administration)
// ============================================================================

type EIAFetcher struct {
	BaseURL string
	APIKey  string
}

func NewEIAFetcher() *EIAFetcher {
	apiKey := os.Getenv("EIA_API_KEY")
	return &EIAFetcher{
		BaseURL: "https://api.eia.gov/v2",
		APIKey:  apiKey,
	}
}

type eiaResponse struct {
	Response struct {
		Data []struct {
			Period      string  `json:"period"`
			Value       float64 `json:"value"`
			SeriesID    string  `json:"series-id,omitempty"`
			ProductName string  `json:"product-name,omitempty"`
			AreaName    string  `json:"area-name,omitempty"`
			Units       string  `json:"units,omitempty"`
		} `json:"data"`
		Description string `json:"description,omitempty"`
	} `json:"response"`
}

// Common EIA series IDs for energy data
var EIASeriesPresets = map[string]string{
	// Petroleum
	"crude_oil_price":      "petroleum/pri/spt",
	"crude_oil_production": "petroleum/crd/crpdn",
	"crude_oil_stocks":     "petroleum/stoc/wstk",

	// Natural Gas
	"natural_gas_price":      "natural-gas/pri/sum",
	"natural_gas_production": "natural-gas/sum/snd",

	// Electricity
	"electricity_generation": "electricity/electric-power-operational-data",
	"electricity_retail":     "electricity/retail-sales",

	// Renewables
	"solar_generation": "electricity/electric-power-operational-data",
	"wind_generation":  "electricity/electric-power-operational-data",
}

// FetchEIAData fetches data from EIA API for a specific series/route
func (f *EIAFetcher) FetchEIAData(d *db.DB, route string, params map[string]string) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("EIA_API_KEY environment variable not set")
	}

	// Build URL
	reqURL := fmt.Sprintf("%s/%s/data/", f.BaseURL, route)
	urlParams := url.Values{}
	urlParams.Set("api_key", f.APIKey)
	urlParams.Set("frequency", "monthly")
	urlParams.Set("data[0]", "value")
	urlParams.Set("sort[0][column]", "period")
	urlParams.Set("sort[0][direction]", "desc")
	urlParams.Set("length", "100")

	for k, v := range params {
		urlParams.Set(k, v)
	}

	resp, err := http.Get(reqURL + "?" + urlParams.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("EIA API error %d: %s", resp.StatusCode, string(body))
	}

	var result eiaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO eia_series
		(series_id, name, description, units, frequency, source, period, value)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, item := range result.Response.Data {
		seriesID := route
		if item.SeriesID != "" {
			seriesID = item.SeriesID
		}

		name := item.ProductName
		if item.AreaName != "" && name != "" {
			name = name + " - " + item.AreaName
		} else if item.AreaName != "" {
			name = item.AreaName
		}

		_, err := stmt.Exec(
			seriesID, name, result.Response.Description,
			item.Units, "monthly", "EIA",
			item.Period, item.Value,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// FetchOilPrices fetches crude oil spot prices
func (f *EIAFetcher) FetchOilPrices(d *db.DB) (int, error) {
	return f.FetchEIAData(d, "petroleum/pri/spt", map[string]string{
		"facets[product][]": "EPCBRENT", // Brent crude
	})
}

// FetchNaturalGasPrices fetches natural gas prices
func (f *EIAFetcher) FetchNaturalGasPrices(d *db.DB) (int, error) {
	return f.FetchEIAData(d, "natural-gas/pri/sum", map[string]string{
		"facets[process][]": "PRS", // Spot price
	})
}

// FetchElectricityGeneration fetches electricity generation by source
func (f *EIAFetcher) FetchElectricityGeneration(d *db.DB) (int, error) {
	return f.FetchEIAData(d, "electricity/electric-power-operational-data/data", map[string]string{
		"facets[sectorid][]": "99", // All sectors
	})
}

// ============================================================================
// ERCOT Fetcher (Texas Grid)
// ============================================================================

type ERCOTFetcher struct {
	BaseURL string
}

func NewERCOTFetcher() *ERCOTFetcher {
	return &ERCOTFetcher{
		BaseURL: "https://www.ercot.com/content/cdr/html",
	}
}

// ERCOT zones/settlement points
var ERCOTZones = []string{
	"LZ_HOUSTON", "LZ_NORTH", "LZ_SOUTH", "LZ_WEST",
	"HB_HUBAVG", "HB_HOUSTON", "HB_NORTH", "HB_SOUTH", "HB_WEST",
}

// FetchERCOTPrices fetches real-time prices from ERCOT
func (f *ERCOTFetcher) FetchERCOTPrices(d *db.DB) (int, error) {
	// ERCOT publishes real-time SPP (Settlement Point Prices) as CSV
	// Note: Actual ERCOT API requires authentication for some data
	// This fetches from their public data portal

	// Real-Time SPP data endpoint (public)
	rtURL := "https://www.ercot.com/content/cdr/html/actual_loads_of_weather_zones.html"

	resp, err := http.Get(rtURL)
	if err != nil {
		return 0, fmt.Errorf("ERCOT fetch error: %w", err)
	}
	defer resp.Body.Close()

	// Parse HTML table or use their API endpoint
	// For now, we'll use the ERCOT SCED data API

	return f.fetchERCOTFromAPI(d)
}

// fetchERCOTFromAPI uses ERCOT's data API
func (f *ERCOTFetcher) fetchERCOTFromAPI(d *db.DB) (int, error) {
	// ERCOT API endpoint for real-time prices (public data)
	apiURL := "https://api.ercot.com/api/public-reports/np4-190-cd/dam_stlmnt_point_prices"

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// Fall back to CSV parsing
		return f.fetchERCOTCSV(d)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return f.fetchERCOTCSV(d)
	}

	var result struct {
		Data []struct {
			DeliveryDate    string  `json:"deliveryDate"`
			HourEnding      string  `json:"hourEnding"`
			SettlementPoint string  `json:"settlementPoint"`
			SPP             float64 `json:"spp"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return f.fetchERCOTCSV(d)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ercot_prices
		(zone, settlement_point, price_type, price, timestamp)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, item := range result.Data {
		zone := extractERCOTZone(item.SettlementPoint)
		timestamp := fmt.Sprintf("%s %s:00", item.DeliveryDate, item.HourEnding)

		_, err := stmt.Exec(zone, item.SettlementPoint, "SPP", item.SPP, timestamp)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// fetchERCOTCSV parses ERCOT CSV data files
func (f *ERCOTFetcher) fetchERCOTCSV(d *db.DB) (int, error) {
	// ERCOT publishes historical data as downloadable CSVs
	// This is a fallback when the API is unavailable

	csvURL := "https://www.ercot.com/files/docs/data/electricity/settlements/dam-settlement-point-prices.csv"
	resp, err := http.Get(csvURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, err
	}

	if len(records) < 2 {
		return 0, fmt.Errorf("no data in ERCOT CSV")
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ercot_prices
		(zone, settlement_point, price_type, price, timestamp)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}
		if len(record) < 4 {
			continue
		}

		settlementPoint := record[0]
		zone := extractERCOTZone(settlementPoint)
		price, _ := strconv.ParseFloat(record[2], 64)
		timestamp := record[1]

		_, err := stmt.Exec(zone, settlementPoint, "DAM_SPP", price, timestamp)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// FetchERCOTLoad fetches system load data
func (f *ERCOTFetcher) FetchERCOTLoad(d *db.DB) (int, error) {
	// ERCOT system load data
	loadURL := "https://www.ercot.com/content/cdr/html/actual_loads_of_weather_zones.html"

	resp, err := http.Get(loadURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Parse HTML table (simplified parsing)
	return parseERCOTLoadHTML(d, string(body))
}

func parseERCOTLoadHTML(d *db.DB, html string) (int, error) {
	// Simplified HTML parsing - in production would use proper HTML parser
	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ercot_load
		(zone, load_mw, forecast_mw, timestamp)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// Note: This is a placeholder - actual implementation would parse
	// the ERCOT HTML tables properly
	return 0, nil
}

// FetchERCOTGeneration fetches generation by fuel type
func (f *ERCOTFetcher) FetchERCOTGeneration(d *db.DB) (int, error) {
	// ERCOT generation mix data
	genURL := "https://www.ercot.com/content/cdr/html/fuel_mix.html"

	resp, err := http.Get(genURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return parseERCOTGenHTML(d, string(body))
}

func parseERCOTGenHTML(d *db.DB, html string) (int, error) {
	stmt, err := d.Prepare(`INSERT OR REPLACE INTO ercot_generation
		(fuel_type, generation_mw, capacity_mw, timestamp)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// Placeholder - actual implementation would parse HTML
	return 0, nil
}

func extractERCOTZone(settlementPoint string) string {
	for _, zone := range ERCOTZones {
		if strings.HasPrefix(settlementPoint, zone) || settlementPoint == zone {
			return zone
		}
	}
	// Extract zone prefix
	parts := strings.Split(settlementPoint, "_")
	if len(parts) >= 2 {
		return parts[0] + "_" + parts[1]
	}
	return settlementPoint
}

// ============================================================================
// Korea Power Exchange (KPX) Fetcher
// ============================================================================

type KPXFetcher struct {
	BaseURL string
}

func NewKPXFetcher() *KPXFetcher {
	return &KPXFetcher{
		BaseURL: "https://www.kpx.or.kr",
	}
}

type kpxSMPResponse struct {
	Result struct {
		List []struct {
			TradeDate string  `json:"trdDt"`
			TradeHour int     `json:"trdHr"`
			SMP       float64 `json:"smp"`
			MaxSMP    float64 `json:"maxSmp"`
			MinSMP    float64 `json:"minSmp"`
		} `json:"list"`
	} `json:"result"`
}

// FetchKoreaPowerPrices fetches SMP (System Marginal Price) from KPX
func (f *KPXFetcher) FetchKoreaPowerPrices(d *db.DB) (int, error) {
	// KPX API endpoint for SMP data
	// Note: Actual KPX API may require authentication
	apiURL := f.BaseURL + "/api/smp/list.do"

	today := time.Now().Format("20060102")
	params := url.Values{}
	params.Set("fromDt", today)
	params.Set("toDt", today)

	resp, err := http.Get(apiURL + "?" + params.Encode())
	if err != nil {
		// Fall back to web scraping
		return f.scrapeKPXPrices(d)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return f.scrapeKPXPrices(d)
	}

	var result kpxSMPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return f.scrapeKPXPrices(d)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO kpx_prices
		(market_type, price_type, price, unit, trade_date, trade_hour)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, item := range result.Result.List {
		_, err := stmt.Exec("SPOT", "SMP", item.SMP, "KRW/kWh", item.TradeDate, item.TradeHour)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// scrapeKPXPrices scrapes KPX website for price data
func (f *KPXFetcher) scrapeKPXPrices(d *db.DB) (int, error) {
	// KPX public page with SMP data
	pageURL := f.BaseURL + "/kpxweb/selectKpxSmpSmpGrid.do"

	resp, err := http.Get(pageURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Parse HTML response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return parseKPXHTML(d, string(body))
}

func parseKPXHTML(d *db.DB, html string) (int, error) {
	// Simplified parsing - actual implementation would use goquery or similar
	stmt, err := d.Prepare(`INSERT OR REPLACE INTO kpx_prices
		(market_type, price_type, price, unit, trade_date, trade_hour)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// Placeholder - in production, parse the actual KPX HTML table
	return 0, nil
}

// FetchKoreaDemand fetches real-time demand/supply data from KPX
func (f *KPXFetcher) FetchKoreaDemand(d *db.DB) (int, error) {
	// KPX real-time demand data
	demandURL := f.BaseURL + "/api/demand/realtime.do"

	resp, err := http.Get(demandURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Demand      float64 `json:"demand"`
			Supply      float64 `json:"supply"`
			Reserve     float64 `json:"reserve"`
			ReserveRate float64 `json:"reserveRate"`
			Timestamp   string  `json:"timestamp"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO kpx_demand
		(demand_mw, supply_mw, reserve_mw, reserve_rate, timestamp)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		result.Data.Demand, result.Data.Supply,
		result.Data.Reserve, result.Data.ReserveRate,
		result.Data.Timestamp,
	)
	if err != nil {
		return 0, err
	}

	return 1, nil
}

// ============================================================================
// ENTSO-E Fetcher (European Grid)
// ============================================================================

type ENTSOEFetcher struct {
	BaseURL   string
	APIToken  string
}

func NewENTSOEFetcher() *ENTSOEFetcher {
	token := os.Getenv("ENTSOE_API_TOKEN")
	return &ENTSOEFetcher{
		BaseURL:  "https://web-api.tp.entsoe.eu/api",
		APIToken: token,
	}
}

// ENTSO-E country codes (EIC codes)
var ENTSOECountries = map[string]string{
	"DE": "10Y1001A1001A83F", // Germany
	"FR": "10YFR-RTE------C", // France
	"ES": "10YES-REE------0", // Spain
	"IT": "10YIT-GRTN-----B", // Italy
	"NL": "10YNL----------L", // Netherlands
	"BE": "10YBE----------2", // Belgium
	"AT": "10YAT-APG------L", // Austria
	"PL": "10YPL-AREA-----S", // Poland
	"GB": "10YGB----------A", // Great Britain
	"NO": "10YNO-0--------C", // Norway
	"SE": "10YSE-1--------K", // Sweden
	"DK": "10Y1001A1001A65H", // Denmark
	"FI": "10YFI-1--------U", // Finland
}

// Document types for ENTSO-E API
const (
	ENTSOEDocActualGeneration   = "A75" // Actual generation per type
	ENTSOEDocActualLoad         = "A65" // Actual total load
	ENTSOEDocDayAheadPrices     = "A44" // Day ahead prices
	ENTSOEDocCrossBorderFlows   = "A11" // Cross-border physical flows
)

type entsoeResponse struct {
	XMLName         xml.Name `xml:"GL_MarketDocument"`
	TimeSeries      []struct {
		InDomainMCP     string `xml:"in_Domain.mcp"`
		OutDomainMCP    string `xml:"out_Domain.mcp"`
		PsrType         string `xml:"MktPSRType>psrType"`
		Period          struct {
			TimeInterval struct {
				Start string `xml:"start"`
				End   string `xml:"end"`
			} `xml:"timeInterval"`
			Points []struct {
				Position int     `xml:"position"`
				Quantity float64 `xml:"quantity"`
			} `xml:"Point"`
		} `xml:"Period"`
	} `xml:"TimeSeries"`
}

// FetchEuropeanGrid fetches grid data for a specific country
func (f *ENTSOEFetcher) FetchEuropeanGrid(d *db.DB, country string) (int, error) {
	if f.APIToken == "" {
		return 0, fmt.Errorf("ENTSOE_API_TOKEN environment variable not set")
	}

	eicCode, ok := ENTSOECountries[strings.ToUpper(country)]
	if !ok {
		return 0, fmt.Errorf("unknown country code: %s (available: DE, FR, ES, IT, NL, BE, AT, PL, GB, NO, SE, DK, FI)", country)
	}

	total := 0

	// Fetch generation data
	genCount, err := f.fetchGeneration(d, eicCode, country)
	if err == nil {
		total += genCount
	}

	// Fetch load data
	loadCount, err := f.fetchLoad(d, eicCode, country)
	if err == nil {
		total += loadCount
	}

	return total, nil
}

func (f *ENTSOEFetcher) fetchGeneration(d *db.DB, eicCode, country string) (int, error) {
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour).Format("200601021500")
	end := now.Format("200601021500")

	params := url.Values{}
	params.Set("securityToken", f.APIToken)
	params.Set("documentType", ENTSOEDocActualGeneration)
	params.Set("processType", "A16") // Realised
	params.Set("in_Domain", eicCode)
	params.Set("periodStart", start)
	params.Set("periodEnd", end)

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("ENTSO-E API error %d: %s", resp.StatusCode, string(body))
	}

	var result entsoeResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO entsoe_generation
		(country_code, production_type, generation_mw, timestamp)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, ts := range result.TimeSeries {
		prodType := mapENTSOEPsrType(ts.PsrType)
		baseTime, _ := time.Parse("2006-01-02T15:04Z", ts.Period.TimeInterval.Start)

		for _, point := range ts.Period.Points {
			pointTime := baseTime.Add(time.Duration(point.Position-1) * 15 * time.Minute)
			timestamp := pointTime.Format(time.RFC3339)

			_, err := stmt.Exec(country, prodType, point.Quantity, timestamp)
			if err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

func (f *ENTSOEFetcher) fetchLoad(d *db.DB, eicCode, country string) (int, error) {
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour).Format("200601021500")
	end := now.Format("200601021500")

	params := url.Values{}
	params.Set("securityToken", f.APIToken)
	params.Set("documentType", ENTSOEDocActualLoad)
	params.Set("processType", "A16") // Realised
	params.Set("outBiddingZone_Domain", eicCode)
	params.Set("periodStart", start)
	params.Set("periodEnd", end)

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ENTSO-E load API error: %d", resp.StatusCode)
	}

	var result entsoeResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO entsoe_load
		(country_code, load_mw, forecast_mw, timestamp)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, ts := range result.TimeSeries {
		baseTime, _ := time.Parse("2006-01-02T15:04Z", ts.Period.TimeInterval.Start)

		for _, point := range ts.Period.Points {
			pointTime := baseTime.Add(time.Duration(point.Position-1) * 15 * time.Minute)
			timestamp := pointTime.Format(time.RFC3339)

			_, err := stmt.Exec(country, point.Quantity, 0, timestamp)
			if err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

// FetchCrossBorderFlows fetches cross-border electricity flows
func (f *ENTSOEFetcher) FetchCrossBorderFlows(d *db.DB, fromCountry, toCountry string) (int, error) {
	if f.APIToken == "" {
		return 0, fmt.Errorf("ENTSOE_API_TOKEN environment variable not set")
	}

	fromEIC, ok := ENTSOECountries[strings.ToUpper(fromCountry)]
	if !ok {
		return 0, fmt.Errorf("unknown country code: %s", fromCountry)
	}
	toEIC, ok := ENTSOECountries[strings.ToUpper(toCountry)]
	if !ok {
		return 0, fmt.Errorf("unknown country code: %s", toCountry)
	}

	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour).Format("200601021500")
	end := now.Format("200601021500")

	params := url.Values{}
	params.Set("securityToken", f.APIToken)
	params.Set("documentType", ENTSOEDocCrossBorderFlows)
	params.Set("in_Domain", toEIC)
	params.Set("out_Domain", fromEIC)
	params.Set("periodStart", start)
	params.Set("periodEnd", end)

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ENTSO-E cross-border API error: %d", resp.StatusCode)
	}

	var result entsoeResponse
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO entsoe_crossborder
		(from_country, to_country, flow_mw, scheduled_mw, timestamp)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, ts := range result.TimeSeries {
		baseTime, _ := time.Parse("2006-01-02T15:04Z", ts.Period.TimeInterval.Start)

		for _, point := range ts.Period.Points {
			pointTime := baseTime.Add(time.Duration(point.Position-1) * 15 * time.Minute)
			timestamp := pointTime.Format(time.RFC3339)

			_, err := stmt.Exec(fromCountry, toCountry, point.Quantity, 0, timestamp)
			if err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

// mapENTSOEPsrType maps ENTSO-E PSR type codes to human-readable names
func mapENTSOEPsrType(code string) string {
	types := map[string]string{
		"B01": "Biomass",
		"B02": "Fossil Brown Coal/Lignite",
		"B03": "Fossil Coal-derived Gas",
		"B04": "Fossil Gas",
		"B05": "Fossil Hard Coal",
		"B06": "Fossil Oil",
		"B07": "Fossil Oil Shale",
		"B08": "Fossil Peat",
		"B09": "Geothermal",
		"B10": "Hydro Pumped Storage",
		"B11": "Hydro Run-of-river and poundage",
		"B12": "Hydro Water Reservoir",
		"B13": "Marine",
		"B14": "Nuclear",
		"B15": "Other renewable",
		"B16": "Solar",
		"B17": "Waste",
		"B18": "Wind Offshore",
		"B19": "Wind Onshore",
		"B20": "Other",
	}
	if name, ok := types[code]; ok {
		return name
	}
	return code
}

// ============================================================================
// Unified Energy Fetcher
// ============================================================================

type EnergyFetcher struct {
	eia    *EIAFetcher
	ercot  *ERCOTFetcher
	kpx    *KPXFetcher
	entsoe *ENTSOEFetcher
}

func NewEnergyFetcher() *EnergyFetcher {
	return &EnergyFetcher{
		eia:    NewEIAFetcher(),
		ercot:  NewERCOTFetcher(),
		kpx:    NewKPXFetcher(),
		entsoe: NewENTSOEFetcher(),
	}
}

// EnergyFetchResult holds results from energy data fetch
type EnergyFetchResult struct {
	EIA        int    `json:"eia"`
	ERCOT      int    `json:"ercot"`
	KPX        int    `json:"kpx"`
	ENTSOE     int    `json:"entsoe"`
	Source     string `json:"source,omitempty"`
	Error      string `json:"error,omitempty"`
}

// FetchAll fetches data from all energy sources
// Key-free: ERCOT (Texas grid, public data)
// Key-required: EIA, KPX, ENTSO-E (skipped if no key)
func (f *EnergyFetcher) FetchAll(d *db.DB) (*EnergyFetchResult, error) {
	if err := EnsureEnergySchema(d); err != nil {
		return nil, err
	}

	result := &EnergyFetchResult{}

	// ERCOT - NO KEY REQUIRED (priority)
	count, err := f.ercot.FetchERCOTPrices(d)
	if err == nil {
		result.ERCOT = count
		result.Source = "ERCOT (Texas)"
	}

	// EIA - requires key (skip if not set)
	if f.eia.APIKey != "" {
		count, err := f.eia.FetchOilPrices(d)
		if err == nil {
			result.EIA += count
		}
	}

	// KPX - requires key/scraping (skip for now)
	// Korean power exchange data not reliably available without auth

	// ENTSO-E - requires key (skip if not set)
	if f.entsoe.APIToken != "" {
		for _, country := range []string{"DE", "FR", "GB"} {
			count, err := f.entsoe.FetchEuropeanGrid(d, country)
			if err == nil {
				result.ENTSOE += count
			}
		}
	}

	return result, nil
}

// AvailableSources returns which energy sources are available
func (f *EnergyFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"ercot":  true,                      // Always available (Texas grid)
		"eia":    f.eia.APIKey != "",        // Requires EIA_API_KEY
		"kpx":    false,                     // Korean - requires auth
		"entsoe": f.entsoe.APIToken != "",   // Requires ENTSOE_API_TOKEN
	}
}

// GetEnergyStats returns statistics for energy data
func GetEnergyStats(d *db.DB) (map[string]int, error) {
	stats := make(map[string]int)
	tables := []string{
		"eia_series", "ercot_prices", "ercot_load", "ercot_generation",
		"kpx_prices", "kpx_demand",
		"entsoe_generation", "entsoe_load", "entsoe_crossborder",
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

// SearchEnergy searches across energy data using FTS
func SearchEnergy(d *db.DB, query string, limit int) ([]map[string]interface{}, error) {
	if limit == 0 {
		limit = 20
	}

	results := []map[string]interface{}{}

	// Search EIA series
	rows, err := d.Query(`
		SELECT series_id, name, description, units, period, value
		FROM eia_series
		WHERE series_id LIKE ? OR name LIKE ?
		ORDER BY period DESC
		LIMIT ?
	`, "%"+query+"%", "%"+query+"%", limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var seriesID, name, description, units, period string
			var value float64
			if err := rows.Scan(&seriesID, &name, &description, &units, &period, &value); err == nil {
				results = append(results, map[string]interface{}{
					"source":      "EIA",
					"series_id":   seriesID,
					"name":        name,
					"description": description,
					"units":       units,
					"period":      period,
					"value":       value,
				})
			}
		}
	}

	return results, nil
}
