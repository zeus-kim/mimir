package fetch

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeus-kim/mimir/internal/db"
)

// =============================================================================
// FRED API (Federal Reserve Economic Data)
// =============================================================================

type FREDFetcher struct {
	BaseURL string
	APIKey  string
}

func NewFREDFetcher() *FREDFetcher {
	apiKey := os.Getenv("FRED_API_KEY")
	return &FREDFetcher{
		BaseURL: "https://api.stlouisfed.org/fred",
		APIKey:  apiKey,
	}
}

type fredSeriesResponse struct {
	Seriess []struct {
		ID                  string `json:"id"`
		Title               string `json:"title"`
		Frequency           string `json:"frequency"`
		Units               string `json:"units"`
		SeasonalAdjustment  string `json:"seasonal_adjustment"`
		LastUpdated         string `json:"last_updated"`
		ObservationStart    string `json:"observation_start"`
		ObservationEnd      string `json:"observation_end"`
	} `json:"seriess"`
}

type fredObservationsResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

// FetchFREDSeries fetches economic data for a specific FRED series
// Common series: GDP, UNRATE (unemployment), CPIAUCSL (inflation), FEDFUNDS, etc.
func (f *FREDFetcher) FetchFREDSeries(d *db.DB, seriesID string) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("FRED_API_KEY environment variable not set")
	}

	// First, get series metadata
	metaParams := url.Values{}
	metaParams.Set("series_id", seriesID)
	metaParams.Set("api_key", f.APIKey)
	metaParams.Set("file_type", "json")

	metaResp, err := http.Get(f.BaseURL + "/series?" + metaParams.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching series metadata: %w", err)
	}
	defer metaResp.Body.Close()

	var seriesInfo fredSeriesResponse
	if err := json.NewDecoder(metaResp.Body).Decode(&seriesInfo); err != nil {
		return 0, fmt.Errorf("decoding series metadata: %w", err)
	}

	if len(seriesInfo.Seriess) == 0 {
		return 0, fmt.Errorf("series not found: %s", seriesID)
	}

	series := seriesInfo.Seriess[0]

	// Then, get recent observations (last 5 years)
	obsParams := url.Values{}
	obsParams.Set("series_id", seriesID)
	obsParams.Set("api_key", f.APIKey)
	obsParams.Set("file_type", "json")
	obsParams.Set("sort_order", "desc")
	obsParams.Set("limit", "100")

	obsResp, err := http.Get(f.BaseURL + "/series/observations?" + obsParams.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching observations: %w", err)
	}
	defer obsResp.Body.Close()

	var observations fredObservationsResponse
	if err := json.NewDecoder(obsResp.Body).Decode(&observations); err != nil {
		return 0, fmt.Errorf("decoding observations: %w", err)
	}

	// Insert into database
	stmt, err := d.Prepare(`INSERT OR REPLACE INTO economic_data
		(series_id, title, observation_date, value, units, frequency, seasonal_adjustment)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, obs := range observations.Observations {
		if obs.Value == "." { // FRED uses "." for missing values
			continue
		}

		value, err := strconv.ParseFloat(obs.Value, 64)
		if err != nil {
			continue
		}

		_, err = stmt.Exec(
			seriesID, series.Title, obs.Date, value,
			series.Units, series.Frequency, series.SeasonalAdjustment,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// FetchMultipleFREDSeries fetches multiple economic indicators
func (f *FREDFetcher) FetchMultipleFREDSeries(d *db.DB, seriesIDs []string) (int, error) {
	total := 0
	for _, id := range seriesIDs {
		count, err := f.FetchFREDSeries(d, id)
		if err != nil {
			fmt.Printf("FRED fetch error (%s): %v\n", id, err)
			continue
		}
		total += count
	}
	return total, nil
}

// CommonEconomicIndicators returns common FRED series IDs
var CommonEconomicIndicators = []string{
	"GDP",       // Gross Domestic Product
	"UNRATE",    // Unemployment Rate
	"CPIAUCSL",  // Consumer Price Index (All Urban Consumers)
	"FEDFUNDS",  // Federal Funds Rate
	"DGS10",     // 10-Year Treasury Constant Maturity Rate
	"M2SL",      // M2 Money Supply
	"INDPRO",    // Industrial Production Index
	"PAYEMS",    // Total Nonfarm Payrolls
	"HOUST",     // Housing Starts
	"RSXFS",     // Retail Sales (ex. Food Services)
}

// =============================================================================
// Yahoo Finance (Unofficial API)
// =============================================================================

type YahooFinanceFetcher struct {
	BaseURL string
}

func NewYahooFinanceFetcher() *YahooFinanceFetcher {
	return &YahooFinanceFetcher{
		BaseURL: "https://query1.finance.yahoo.com/v8/finance",
	}
}

type yahooQuoteResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol                     string  `json:"symbol"`
			ShortName                  string  `json:"shortName"`
			LongName                   string  `json:"longName"`
			RegularMarketPrice         float64 `json:"regularMarketPrice"`
			RegularMarketChange        float64 `json:"regularMarketChange"`
			RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
			RegularMarketVolume        int64   `json:"regularMarketVolume"`
			MarketCap                  int64   `json:"marketCap"`
			FiftyTwoWeekHigh           float64 `json:"fiftyTwoWeekHigh"`
			FiftyTwoWeekLow            float64 `json:"fiftyTwoWeekLow"`
			TrailingPE                 float64 `json:"trailingPE"`
			ForwardPE                  float64 `json:"forwardPE"`
			DividendYield              float64 `json:"dividendYield"`
			Currency                   string  `json:"currency"`
			Exchange                   string  `json:"exchange"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"quoteResponse"`
}

// FetchStockQuote fetches current stock quote for a symbol
func (f *YahooFinanceFetcher) FetchStockQuote(d *db.DB, symbol string) (int, error) {
	return f.FetchStockQuotes(d, []string{symbol})
}

// FetchStockQuotes fetches quotes for multiple symbols
func (f *YahooFinanceFetcher) FetchStockQuotes(d *db.DB, symbols []string) (int, error) {
	if len(symbols) == 0 {
		return 0, nil
	}

	symbolsStr := strings.Join(symbols, ",")
	reqURL := fmt.Sprintf("%s/chart/%s?interval=1d&range=1d", f.BaseURL, symbols[0])

	// For multiple symbols, use the quote endpoint
	if len(symbols) > 1 || true {
		reqURL = fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s", url.QueryEscape(symbolsStr))
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching quotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("yahoo finance returned status %d", resp.StatusCode)
	}

	var result yahooQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	if result.QuoteResponse.Error != nil {
		return 0, fmt.Errorf("yahoo finance error: %v", result.QuoteResponse.Error)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO stock_quotes
		(symbol, name, price, change, change_percent, volume, market_cap,
		 fifty_two_week_high, fifty_two_week_low, pe_ratio, dividend_yield,
		 currency, exchange, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, quote := range result.QuoteResponse.Result {
		name := quote.LongName
		if name == "" {
			name = quote.ShortName
		}

		_, err := stmt.Exec(
			quote.Symbol, name, quote.RegularMarketPrice,
			quote.RegularMarketChange, quote.RegularMarketChangePercent,
			quote.RegularMarketVolume, quote.MarketCap,
			quote.FiftyTwoWeekHigh, quote.FiftyTwoWeekLow,
			quote.TrailingPE, quote.DividendYield,
			quote.Currency, quote.Exchange,
		)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// =============================================================================
// Alpha Vantage
// =============================================================================

type AlphaVantageFetcher struct {
	BaseURL string
	APIKey  string
}

func NewAlphaVantageFetcher() *AlphaVantageFetcher {
	apiKey := os.Getenv("ALPHAVANTAGE_API_KEY")
	return &AlphaVantageFetcher{
		BaseURL: "https://www.alphavantage.co/query",
		APIKey:  apiKey,
	}
}

type alphaVantageTimeSeriesResponse struct {
	MetaData struct {
		Symbol        string `json:"2. Symbol"`
		LastRefreshed string `json:"3. Last Refreshed"`
	} `json:"Meta Data"`
	TimeSeries map[string]struct {
		Open   string `json:"1. open"`
		High   string `json:"2. high"`
		Low    string `json:"3. low"`
		Close  string `json:"4. close"`
		Volume string `json:"5. volume"`
	} `json:"Time Series (Daily)"`
}

type alphaVantageGlobalQuoteResponse struct {
	GlobalQuote struct {
		Symbol           string `json:"01. symbol"`
		Open             string `json:"02. open"`
		High             string `json:"03. high"`
		Low              string `json:"04. low"`
		Price            string `json:"05. price"`
		Volume           string `json:"06. volume"`
		LatestTradingDay string `json:"07. latest trading day"`
		PreviousClose    string `json:"08. previous close"`
		Change           string `json:"09. change"`
		ChangePercent    string `json:"10. change percent"`
	} `json:"Global Quote"`
}

// FetchAlphaVantageDaily fetches daily time series for a symbol
func (f *AlphaVantageFetcher) FetchAlphaVantageDaily(d *db.DB, symbol string, outputSize string) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("ALPHAVANTAGE_API_KEY environment variable not set")
	}

	if outputSize == "" {
		outputSize = "compact" // compact = last 100 days, full = 20+ years
	}

	params := url.Values{}
	params.Set("function", "TIME_SERIES_DAILY")
	params.Set("symbol", symbol)
	params.Set("outputsize", outputSize)
	params.Set("apikey", f.APIKey)

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching data: %w", err)
	}
	defer resp.Body.Close()

	var result alphaVantageTimeSeriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	if result.TimeSeries == nil {
		return 0, fmt.Errorf("no time series data returned for %s (rate limit may apply)", symbol)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO stock_daily_prices
		(symbol, date, open, high, low, close, volume)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	count := 0
	for date, data := range result.TimeSeries {
		open, _ := strconv.ParseFloat(data.Open, 64)
		high, _ := strconv.ParseFloat(data.High, 64)
		low, _ := strconv.ParseFloat(data.Low, 64)
		close, _ := strconv.ParseFloat(data.Close, 64)
		volume, _ := strconv.ParseInt(data.Volume, 10, 64)

		_, err := stmt.Exec(symbol, date, open, high, low, close, volume)
		if err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// FetchAlphaVantageQuote fetches the latest global quote for a symbol
func (f *AlphaVantageFetcher) FetchAlphaVantageQuote(d *db.DB, symbol string) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("ALPHAVANTAGE_API_KEY environment variable not set")
	}

	params := url.Values{}
	params.Set("function", "GLOBAL_QUOTE")
	params.Set("symbol", symbol)
	params.Set("apikey", f.APIKey)

	resp, err := http.Get(f.BaseURL + "?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching quote: %w", err)
	}
	defer resp.Body.Close()

	var result alphaVantageGlobalQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	q := result.GlobalQuote
	if q.Symbol == "" {
		return 0, fmt.Errorf("no quote data returned for %s", symbol)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO stock_quotes
		(symbol, name, price, change, change_percent, volume, market_cap,
		 fifty_two_week_high, fifty_two_week_low, pe_ratio, dividend_yield,
		 currency, exchange, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, NULL, 'USD', 'ALPHAVANTAGE', datetime('now'))`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	price, _ := strconv.ParseFloat(q.Price, 64)
	change, _ := strconv.ParseFloat(q.Change, 64)
	changePercent := strings.TrimSuffix(q.ChangePercent, "%")
	changePct, _ := strconv.ParseFloat(changePercent, 64)
	volume, _ := strconv.ParseInt(q.Volume, 10, 64)

	_, err = stmt.Exec(q.Symbol, q.Symbol, price, change, changePct, volume)
	if err != nil {
		return 0, fmt.Errorf("inserting quote: %w", err)
	}

	return 1, nil
}

// =============================================================================
// DART API (Korea Financial Supervisory Service)
// =============================================================================

type DARTFetcher struct {
	BaseURL string
	APIKey  string
}

func NewDARTFetcher() *DARTFetcher {
	apiKey := os.Getenv("DART_API_KEY")
	return &DARTFetcher{
		BaseURL: "https://opendart.fss.or.kr/api",
		APIKey:  apiKey,
	}
}

type dartCorpCodeResponse struct {
	XMLName xml.Name `xml:"result"`
	List    []struct {
		CorpCode   string `xml:"corp_code"`
		CorpName   string `xml:"corp_name"`
		StockCode  string `xml:"stock_code"`
		ModifyDate string `xml:"modify_date"`
	} `xml:"list"`
}

type dartDisclosureResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	List    []struct {
		CorpCode    string `json:"corp_code"`
		CorpName    string `json:"corp_name"`
		CorpCls     string `json:"corp_cls"` // Y: 유가증권, K: 코스닥, N: 코넥스, E: 기타
		ReportNm    string `json:"report_nm"`
		RceptNo     string `json:"rcept_no"`
		FlrNm       string `json:"flr_nm"` // 공시제출인명
		RceptDt     string `json:"rcept_dt"`
		Rm          string `json:"rm"` // 비고
	} `json:"list"`
	PageNo     int `json:"page_no"`
	PageCount  int `json:"page_count"`
	TotalCount int `json:"total_count"`
	TotalPage  int `json:"total_page"`
}

// FetchKoreanDisclosures fetches recent Korean company disclosures
func (f *DARTFetcher) FetchKoreanDisclosures(d *db.DB, corpName string, limit int) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("DART_API_KEY environment variable not set")
	}

	if limit == 0 {
		limit = 100
	}

	params := url.Values{}
	params.Set("crtfc_key", f.APIKey)
	if corpName != "" {
		params.Set("corp_name", corpName)
	}
	params.Set("page_count", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("sort", "date")
	params.Set("sort_mth", "desc")

	resp, err := http.Get(f.BaseURL + "/list.json?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching disclosures: %w", err)
	}
	defer resp.Body.Close()

	var result dartDisclosureResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	if result.Status != "000" {
		return 0, fmt.Errorf("DART API error: %s - %s", result.Status, result.Message)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_disclosures
		(rcept_no, corp_code, corp_name, corp_cls, report_nm, flr_nm, rcept_dt, rm, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_disclosures_fts
		(rcept_no, corp_name, report_nm, flr_nm)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing FTS statement: %w", err)
	}
	defer ftsStmt.Close()

	count := 0
	for _, disc := range result.List {
		url := fmt.Sprintf("https://dart.fss.or.kr/dsaf001/main.do?rcpNo=%s", disc.RceptNo)

		_, err := stmt.Exec(
			disc.RceptNo, disc.CorpCode, disc.CorpName, disc.CorpCls,
			disc.ReportNm, disc.FlrNm, disc.RceptDt, disc.Rm, url,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(disc.RceptNo, disc.CorpName, disc.ReportNm, disc.FlrNm)
		count++
	}

	return count, nil
}

// FetchDisclosuresByDate fetches disclosures within a date range
func (f *DARTFetcher) FetchDisclosuresByDate(d *db.DB, startDate, endDate string, limit int) (int, error) {
	if f.APIKey == "" {
		return 0, fmt.Errorf("DART_API_KEY environment variable not set")
	}

	if limit == 0 {
		limit = 100
	}

	params := url.Values{}
	params.Set("crtfc_key", f.APIKey)
	params.Set("bgn_de", startDate) // YYYYMMDD format
	params.Set("end_de", endDate)   // YYYYMMDD format
	params.Set("page_count", fmt.Sprintf("%d", min(limit, 100)))
	params.Set("sort", "date")
	params.Set("sort_mth", "desc")

	resp, err := http.Get(f.BaseURL + "/list.json?" + params.Encode())
	if err != nil {
		return 0, fmt.Errorf("fetching disclosures: %w", err)
	}
	defer resp.Body.Close()

	var result dartDisclosureResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	if result.Status != "000" {
		return 0, fmt.Errorf("DART API error: %s - %s", result.Status, result.Message)
	}

	stmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_disclosures
		(rcept_no, corp_code, corp_name, corp_cls, report_nm, flr_nm, rcept_dt, rm, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	ftsStmt, err := d.Prepare(`INSERT OR REPLACE INTO korean_disclosures_fts
		(rcept_no, corp_name, report_nm, flr_nm)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing FTS statement: %w", err)
	}
	defer ftsStmt.Close()

	count := 0
	for _, disc := range result.List {
		url := fmt.Sprintf("https://dart.fss.or.kr/dsaf001/main.do?rcpNo=%s", disc.RceptNo)

		_, err := stmt.Exec(
			disc.RceptNo, disc.CorpCode, disc.CorpName, disc.CorpCls,
			disc.ReportNm, disc.FlrNm, disc.RceptDt, disc.Rm, url,
		)
		if err != nil {
			continue
		}

		ftsStmt.Exec(disc.RceptNo, disc.CorpName, disc.ReportNm, disc.FlrNm)
		count++
	}

	return count, nil
}

// =============================================================================
// FinanceFetcher - Unified Finance Data Fetcher
// =============================================================================

type FinanceFetcher struct {
	fred         *FREDFetcher
	yahoo        *YahooFinanceFetcher
	alphaVantage *AlphaVantageFetcher
	dart         *DARTFetcher
}

func NewFinanceFetcher() *FinanceFetcher {
	return &FinanceFetcher{
		fred:         NewFREDFetcher(),
		yahoo:        NewYahooFinanceFetcher(),
		alphaVantage: NewAlphaVantageFetcher(),
		dart:         NewDARTFetcher(),
	}
}

// FinanceFetchResult holds results from finance fetch operations
type FinanceFetchResult struct {
	EconomicData       int `json:"economic_data"`
	StockQuotes        int `json:"stock_quotes"`
	StockDailyPrices   int `json:"stock_daily_prices"`
	KoreanDisclosures  int `json:"korean_disclosures"`
}

// FetchEconomicIndicators fetches common economic indicators from FRED
// Requires FRED_API_KEY - returns 0 if not set
func (f *FinanceFetcher) FetchEconomicIndicators(d *db.DB) (*FinanceFetchResult, error) {
	if f.fred.APIKey == "" {
		return &FinanceFetchResult{}, nil // Skip if no key
	}
	count, err := f.fred.FetchMultipleFREDSeries(d, CommonEconomicIndicators)
	if err != nil {
		return nil, err
	}
	return &FinanceFetchResult{EconomicData: count}, nil
}

// FetchStockQuotes fetches stock quotes for multiple symbols
func (f *FinanceFetcher) FetchStockQuotes(d *db.DB, symbols []string) (*FinanceFetchResult, error) {
	count, err := f.yahoo.FetchStockQuotes(d, symbols)
	if err != nil {
		return nil, err
	}
	return &FinanceFetchResult{StockQuotes: count}, nil
}

// FetchKoreanDisclosures fetches Korean company disclosures
// Requires DART_API_KEY - returns 0 if not set
func (f *FinanceFetcher) FetchKoreanDisclosures(d *db.DB, corpName string, limit int) (*FinanceFetchResult, error) {
	if f.dart.APIKey == "" {
		return &FinanceFetchResult{}, nil // Skip if no key
	}
	count, err := f.dart.FetchKoreanDisclosures(d, corpName, limit)
	if err != nil {
		return nil, err
	}
	return &FinanceFetchResult{KoreanDisclosures: count}, nil
}

// FetchAll fetches from all available finance sources
// Key-free: Yahoo Finance
// Key-required: FRED, Alpha Vantage, DART (skipped if no key)
func (f *FinanceFetcher) FetchAll(d *db.DB, symbols []string) (*FinanceFetchResult, error) {
	result := &FinanceFetchResult{}

	// Yahoo Finance - no key required
	if len(symbols) > 0 {
		count, err := f.yahoo.FetchStockQuotes(d, symbols)
		if err == nil {
			result.StockQuotes = count
		}
	}

	// FRED - requires key
	if f.fred.APIKey != "" {
		count, _ := f.fred.FetchMultipleFREDSeries(d, CommonEconomicIndicators)
		result.EconomicData = count
	}

	// DART - requires key
	if f.dart.APIKey != "" {
		count, _ := f.dart.FetchKoreanDisclosures(d, "", 50)
		result.KoreanDisclosures = count
	}

	return result, nil
}

// AvailableSources returns which finance sources are available
func (f *FinanceFetcher) AvailableSources() map[string]bool {
	return map[string]bool{
		"yahoo_finance": true,                    // Always available
		"fred":          f.fred.APIKey != "",     // Requires FRED_API_KEY
		"alpha_vantage": f.alphaVantage.APIKey != "", // Requires ALPHAVANTAGE_API_KEY
		"dart":          f.dart.APIKey != "",     // Requires DART_API_KEY
	}
}

// FinanceStats holds statistics for finance data
type FinanceStats struct {
	EconomicData      int `json:"economic_data"`
	StockQuotes       int `json:"stock_quotes"`
	StockDailyPrices  int `json:"stock_daily_prices"`
	KoreanDisclosures int `json:"korean_disclosures"`
}

// GetStats returns statistics for all finance data tables
func (f *FinanceFetcher) GetStats(d *db.DB) (*FinanceStats, error) {
	stats := &FinanceStats{}

	tables := map[string]*int{
		"economic_data":       &stats.EconomicData,
		"stock_quotes":        &stats.StockQuotes,
		"stock_daily_prices":  &stats.StockDailyPrices,
		"korean_disclosures":  &stats.KoreanDisclosures,
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
