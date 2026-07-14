package i18n

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Language represents a supported language
type Language string

const (
	EN Language = "en"
	KO Language = "ko"
	JA Language = "ja"
	ZH Language = "zh"
	ES Language = "es"
	FR Language = "fr"
	DE Language = "de"
)

// String returns the string representation of the language
func (l Language) String() string {
	return string(l)
}

// Messages holds all translated messages for a language
type Messages struct {
	// General
	AppName        string `json:"app_name"`
	AppDescription string `json:"app_description"`
	Version        string `json:"version"`

	// Errors
	ErrNotFound       string `json:"err_not_found"`
	ErrInvalidInput   string `json:"err_invalid_input"`
	ErrAPIFailed      string `json:"err_api_failed"`
	ErrRateLimit      string `json:"err_rate_limit"`
	ErrNoAPIKey       string `json:"err_no_api_key"`
	ErrDBError        string `json:"err_db_error"`
	ErrTimeout        string `json:"err_timeout"`

	// Verticals
	VerticalCreated     string `json:"vertical_created"`
	VerticalDeleted     string `json:"vertical_deleted"`
	VerticalNotFound    string `json:"vertical_not_found"`
	VerticalExists      string `json:"vertical_exists"`
	VerticalFetchStart  string `json:"vertical_fetch_start"`
	VerticalFetchDone   string `json:"vertical_fetch_done"`
	VerticalPruneStart  string `json:"vertical_prune_start"`
	VerticalPruneDone   string `json:"vertical_prune_done"`

	// Data Sources
	SourceDiscovered string `json:"source_discovered"`
	SourceValidated  string `json:"source_validated"`
	SourceAdded      string `json:"source_added"`
	SourceRemoved    string `json:"source_removed"`
	SourcesTotal     string `json:"sources_total"`

	// Fetching
	FetchStarted   string `json:"fetch_started"`
	FetchCompleted string `json:"fetch_completed"`
	FetchFailed    string `json:"fetch_failed"`
	FetchSkipped   string `json:"fetch_skipped"`
	DocumentsAdded string `json:"documents_added"`

	// Search
	SearchResults string `json:"search_results"`
	SearchNoHits  string `json:"search_no_hits"`

	// Briefing
	BriefingGenerated string `json:"briefing_generated"`
	BriefingSent      string `json:"briefing_sent"`

	// Statistics
	StatDocuments  string `json:"stat_documents"`
	StatFeeds      string `json:"stat_feeds"`
	StatFitPercent string `json:"stat_fit_percent"`
	StatLastUpdate string `json:"stat_last_update"`

	// API Status
	APIAvailable   string `json:"api_available"`
	APIUnavailable string `json:"api_unavailable"`
	APIKeyMissing  string `json:"api_key_missing"`

	// Domains
	DomainPharma    string `json:"domain_pharma"`
	DomainAI        string `json:"domain_ai"`
	DomainLegal     string `json:"domain_legal"`
	DomainFinance   string `json:"domain_finance"`
	DomainPolitics  string `json:"domain_politics"`
	DomainEnergy    string `json:"domain_energy"`
	DomainFood      string `json:"domain_food"`
	DomainTech      string `json:"domain_tech"`

	// Tool Descriptions
	ToolHealthDesc     string `json:"tool_health_desc"`
	ToolMetricsDesc    string `json:"tool_metrics_desc"`
	ToolSystemInfoDesc string `json:"tool_system_info_desc"`
	ToolAPIStatusDesc  string `json:"tool_api_status_desc"`

	// System
	APIStatusNote string `json:"api_status_note"`
}

// Translations holds all language translations
var translations = map[Language]*Messages{
	EN: {
		AppName:        "Mimir",
		AppDescription: "Vertical Search Engine Factory",
		Version:        "Version",

		ErrNotFound:     "Not found: %s",
		ErrInvalidInput: "Invalid input: %s",
		ErrAPIFailed:    "API request failed: %s",
		ErrRateLimit:    "Rate limit exceeded, please wait",
		ErrNoAPIKey:     "API key required: %s",
		ErrDBError:      "Database error: %s",
		ErrTimeout:      "Request timed out",

		VerticalCreated:    "Vertical '%s' created successfully",
		VerticalDeleted:    "Vertical '%s' deleted",
		VerticalNotFound:   "Vertical '%s' not found",
		VerticalExists:     "Vertical '%s' already exists",
		VerticalFetchStart: "Starting fetch for vertical '%s'",
		VerticalFetchDone:  "Fetch completed: %d documents added",
		VerticalPruneStart: "Pruning low-relevance feeds",
		VerticalPruneDone:  "Pruned %d feeds, %d remaining",

		SourceDiscovered: "Discovered %d potential sources",
		SourceValidated:  "Validated %d sources",
		SourceAdded:      "Added source: %s",
		SourceRemoved:    "Removed source: %s",
		SourcesTotal:     "Total sources: %d",

		FetchStarted:   "Fetching from %s",
		FetchCompleted: "Completed: %d items from %s",
		FetchFailed:    "Failed to fetch from %s: %s",
		FetchSkipped:   "Skipped %s (no API key)",
		DocumentsAdded: "%d documents added",

		SearchResults: "Found %d results for '%s'",
		SearchNoHits:  "No results found for '%s'",

		BriefingGenerated: "Briefing generated: %s",
		BriefingSent:      "Briefing sent via %s",

		StatDocuments:  "Documents: %d",
		StatFeeds:      "Feeds: %d",
		StatFitPercent: "Domain fit: %.1f%%",
		StatLastUpdate: "Last updated: %s",

		APIAvailable:   "✓ %s available",
		APIUnavailable: "✗ %s unavailable",
		APIKeyMissing:  "⚠ %s (API key required)",

		DomainPharma:   "Pharmaceutical & Biotech",
		DomainAI:       "AI & Machine Learning",
		DomainLegal:    "Legal & Regulatory",
		DomainFinance:  "Finance & Economics",
		DomainPolitics: "Politics & Policy",
		DomainEnergy:   "Energy & Utilities",
		DomainFood:     "Food & Nutrition",
		DomainTech:     "Technology & Startups",

		ToolHealthDesc:     "Check server health status",
		ToolMetricsDesc:    "Get server metrics and statistics",
		ToolSystemInfoDesc: "Get system information",
		ToolAPIStatusDesc:  "Check available APIs and their status",

		APIStatusNote: "Key-free APIs work without registration. Optional APIs require environment variables.",
	},
	KO: {
		AppName:        "미미르",
		AppDescription: "버티컬 검색엔진 공장",
		Version:        "버전",

		ErrNotFound:     "찾을 수 없음: %s",
		ErrInvalidInput: "잘못된 입력: %s",
		ErrAPIFailed:    "API 요청 실패: %s",
		ErrRateLimit:    "요청 한도 초과, 잠시 후 재시도",
		ErrNoAPIKey:     "API 키 필요: %s",
		ErrDBError:      "데이터베이스 오류: %s",
		ErrTimeout:      "요청 시간 초과",

		VerticalCreated:    "버티컬 '%s' 생성 완료",
		VerticalDeleted:    "버티컬 '%s' 삭제됨",
		VerticalNotFound:   "버티컬 '%s' 없음",
		VerticalExists:     "버티컬 '%s' 이미 존재",
		VerticalFetchStart: "버티컬 '%s' 수집 시작",
		VerticalFetchDone:  "수집 완료: %d개 문서 추가",
		VerticalPruneStart: "낮은 적합도 피드 정리 중",
		VerticalPruneDone:  "%d개 피드 정리, %d개 남음",

		SourceDiscovered: "%d개 소스 발견",
		SourceValidated:  "%d개 소스 검증됨",
		SourceAdded:      "소스 추가: %s",
		SourceRemoved:    "소스 제거: %s",
		SourcesTotal:     "총 소스: %d개",

		FetchStarted:   "%s에서 수집 중",
		FetchCompleted: "완료: %s에서 %d개",
		FetchFailed:    "%s 수집 실패: %s",
		FetchSkipped:   "%s 건너뜀 (API 키 없음)",
		DocumentsAdded: "%d개 문서 추가됨",

		SearchResults: "'%s' 검색 결과 %d건",
		SearchNoHits:  "'%s' 검색 결과 없음",

		BriefingGenerated: "브리핑 생성: %s",
		BriefingSent:      "%s 통해 브리핑 전송됨",

		StatDocuments:  "문서: %d개",
		StatFeeds:      "피드: %d개",
		StatFitPercent: "도메인 적합도: %.1f%%",
		StatLastUpdate: "최근 업데이트: %s",

		APIAvailable:   "✓ %s 사용 가능",
		APIUnavailable: "✗ %s 사용 불가",
		APIKeyMissing:  "⚠ %s (API 키 필요)",

		DomainPharma:   "제약/바이오",
		DomainAI:       "인공지능/머신러닝",
		DomainLegal:    "법률/규제",
		DomainFinance:  "금융/경제",
		DomainPolitics: "정치/정책",
		DomainEnergy:   "에너지/전력",
		DomainFood:     "식품/영양",
		DomainTech:     "기술/스타트업",

		ToolHealthDesc:     "서버 상태 확인",
		ToolMetricsDesc:    "서버 메트릭 및 통계 조회",
		ToolSystemInfoDesc: "시스템 정보 조회",
		ToolAPIStatusDesc:  "사용 가능한 API 상태 확인",

		APIStatusNote: "Key-free API는 등록 없이 사용 가능. 선택적 API는 환경변수 설정 필요.",
	},
	JA: {
		AppName:        "ミーミル",
		AppDescription: "バーティカル検索エンジンファクトリー",
		Version:        "バージョン",

		ErrNotFound:     "見つかりません: %s",
		ErrInvalidInput: "無効な入力: %s",
		ErrAPIFailed:    "APIリクエスト失敗: %s",
		ErrRateLimit:    "レート制限を超えました",
		ErrNoAPIKey:     "APIキーが必要です: %s",
		ErrDBError:      "データベースエラー: %s",
		ErrTimeout:      "リクエストタイムアウト",

		VerticalCreated:    "バーティカル '%s' を作成しました",
		VerticalDeleted:    "バーティカル '%s' を削除しました",
		VerticalNotFound:   "バーティカル '%s' が見つかりません",
		VerticalExists:     "バーティカル '%s' は既に存在します",
		VerticalFetchStart: "バーティカル '%s' の取得を開始",
		VerticalFetchDone:  "取得完了: %d 件追加",
		VerticalPruneStart: "低関連フィードを削除中",
		VerticalPruneDone:  "%d 件削除、%d 件残存",

		SourceDiscovered: "%d 件のソースを発見",
		SourceValidated:  "%d 件のソースを検証",
		SourceAdded:      "ソース追加: %s",
		SourceRemoved:    "ソース削除: %s",
		SourcesTotal:     "合計ソース: %d",

		FetchStarted:   "%s から取得中",
		FetchCompleted: "完了: %s から %d 件",
		FetchFailed:    "%s の取得失敗: %s",
		FetchSkipped:   "%s をスキップ (APIキーなし)",
		DocumentsAdded: "%d 件の文書を追加",

		SearchResults: "'%s' の検索結果 %d 件",
		SearchNoHits:  "'%s' の検索結果なし",

		BriefingGenerated: "ブリーフィング生成: %s",
		BriefingSent:      "%s でブリーフィングを送信",

		StatDocuments:  "文書: %d",
		StatFeeds:      "フィード: %d",
		StatFitPercent: "ドメイン適合度: %.1f%%",
		StatLastUpdate: "最終更新: %s",

		APIAvailable:   "✓ %s 利用可能",
		APIUnavailable: "✗ %s 利用不可",
		APIKeyMissing:  "⚠ %s (APIキー必要)",

		DomainPharma:   "製薬/バイオ",
		DomainAI:       "AI/機械学習",
		DomainLegal:    "法律/規制",
		DomainFinance:  "金融/経済",
		DomainPolitics: "政治/政策",
		DomainEnergy:   "エネルギー",
		DomainFood:     "食品/栄養",
		DomainTech:     "テクノロジー",
	},
	ZH: {
		AppName:        "弥米尔",
		AppDescription: "垂直搜索引擎工厂",
		Version:        "版本",

		ErrNotFound:     "未找到: %s",
		ErrInvalidInput: "无效输入: %s",
		ErrAPIFailed:    "API请求失败: %s",
		ErrRateLimit:    "请求频率超限",
		ErrNoAPIKey:     "需要API密钥: %s",
		ErrDBError:      "数据库错误: %s",
		ErrTimeout:      "请求超时",

		VerticalCreated:    "垂直领域 '%s' 创建成功",
		VerticalDeleted:    "垂直领域 '%s' 已删除",
		VerticalNotFound:   "未找到垂直领域 '%s'",
		VerticalExists:     "垂直领域 '%s' 已存在",
		VerticalFetchStart: "开始获取垂直领域 '%s'",
		VerticalFetchDone:  "获取完成: 添加 %d 条文档",
		VerticalPruneStart: "正在清理低相关性订阅源",
		VerticalPruneDone:  "已清理 %d 个订阅源，剩余 %d 个",

		SourceDiscovered: "发现 %d 个来源",
		SourceValidated:  "验证 %d 个来源",
		SourceAdded:      "添加来源: %s",
		SourceRemoved:    "移除来源: %s",
		SourcesTotal:     "总来源数: %d",

		FetchStarted:   "正在从 %s 获取",
		FetchCompleted: "完成: 从 %s 获取 %d 条",
		FetchFailed:    "从 %s 获取失败: %s",
		FetchSkipped:   "跳过 %s (无API密钥)",
		DocumentsAdded: "添加 %d 条文档",

		SearchResults: "'%s' 搜索结果 %d 条",
		SearchNoHits:  "'%s' 无搜索结果",

		BriefingGenerated: "简报已生成: %s",
		BriefingSent:      "简报已通过 %s 发送",

		StatDocuments:  "文档: %d",
		StatFeeds:      "订阅源: %d",
		StatFitPercent: "领域匹配度: %.1f%%",
		StatLastUpdate: "最后更新: %s",

		APIAvailable:   "✓ %s 可用",
		APIUnavailable: "✗ %s 不可用",
		APIKeyMissing:  "⚠ %s (需要API密钥)",

		DomainPharma:   "制药/生物",
		DomainAI:       "人工智能",
		DomainLegal:    "法律/监管",
		DomainFinance:  "金融/经济",
		DomainPolitics: "政治/政策",
		DomainEnergy:   "能源/电力",
		DomainFood:     "食品/营养",
		DomainTech:     "科技/创业",
	},
}

var (
	currentLang Language = EN
	mu          sync.RWMutex
)

// SetLanguage sets the current language
func SetLanguage(lang Language) {
	mu.Lock()
	defer mu.Unlock()
	currentLang = lang
}

// GetLanguage returns the current language
func GetLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// Get returns messages for the current language
func Get() *Messages {
	mu.RLock()
	defer mu.RUnlock()
	if m, ok := translations[currentLang]; ok {
		return m
	}
	return translations[EN]
}

// GetFor returns messages for a specific language
func GetFor(lang Language) *Messages {
	if m, ok := translations[lang]; ok {
		return m
	}
	return translations[EN]
}

// T translates a message key with optional formatting arguments
func T(key string, args ...interface{}) string {
	m := Get()
	var template string

	switch key {
	case "app_name":
		template = m.AppName
	case "app_description":
		template = m.AppDescription
	case "err_not_found":
		template = m.ErrNotFound
	case "err_invalid_input":
		template = m.ErrInvalidInput
	case "err_api_failed":
		template = m.ErrAPIFailed
	case "err_rate_limit":
		template = m.ErrRateLimit
	case "err_no_api_key":
		template = m.ErrNoAPIKey
	case "err_db_error":
		template = m.ErrDBError
	case "vertical_created":
		template = m.VerticalCreated
	case "vertical_deleted":
		template = m.VerticalDeleted
	case "vertical_not_found":
		template = m.VerticalNotFound
	case "fetch_completed":
		template = m.FetchCompleted
	case "fetch_failed":
		template = m.FetchFailed
	case "search_results":
		template = m.SearchResults
	case "search_no_hits":
		template = m.SearchNoHits
	case "vertical_exists":
		template = m.VerticalExists
	case "tool_health_desc":
		template = m.ToolHealthDesc
	case "tool_metrics_desc":
		template = m.ToolMetricsDesc
	case "tool_system_info_desc":
		template = m.ToolSystemInfoDesc
	case "tool_api_status_desc":
		template = m.ToolAPIStatusDesc
	case "api_status_note":
		template = m.APIStatusNote
	default:
		return key
	}

	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}
	return template
}

// ParseLanguage parses a language string
func ParseLanguage(s string) Language {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "ko", "korean", "한국어":
		return KO
	case "ja", "japanese", "日本語":
		return JA
	case "zh", "chinese", "中文":
		return ZH
	case "es", "spanish", "español":
		return ES
	case "fr", "french", "français":
		return FR
	case "de", "german", "deutsch":
		return DE
	default:
		return EN
	}
}

// SupportedLanguages returns all supported languages
func SupportedLanguages() []Language {
	return []Language{EN, KO, JA, ZH, ES, FR, DE}
}

// Export exports messages to JSON
func Export(lang Language) ([]byte, error) {
	m := GetFor(lang)
	return json.MarshalIndent(m, "", "  ")
}

// Import imports messages from JSON
func Import(lang Language, data []byte) error {
	var m Messages
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	mu.Lock()
	translations[lang] = &m
	mu.Unlock()
	return nil
}
