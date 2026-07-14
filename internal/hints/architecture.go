package hints

type ArchitectureRef struct {
	Name        string   `json:"name"`
	Repo        string   `json:"repo"`
	Concepts    []string `json:"concepts"`
	MimirUsage  string   `json:"mimir_usage"`
}

type VerticalBuildingGuide struct {
	Phase       string   `json:"phase"`
	Description string   `json:"description"`
	Techniques  []string `json:"techniques"`
	Tools       []string `json:"tools"`
}

var ReferenceArchitectures = []ArchitectureRef{
	{
		Name: "ACHE",
		Repo: "github.com/ViDA-NYU/ache",
		Concepts: []string{
			"Focused Crawler - 도메인에 집중하는 크롤러",
			"Domain Classifier - 페이지가 도메인에 속하는지 분류",
			"Link Prioritization - 어떤 링크를 먼저 따라갈지",
			"Relevance Score - 페이지 관련성 점수",
			"Frontier Management - 크롤링 대기열 관리",
		},
		MimirUsage: "domain_fit 계산, 피드 프루닝, 관련성 기반 수집 우선순위",
	},
	{
		Name: "DISCO",
		Repo: "github.com/ViDA-NYU/domain-discovery-crawler",
		Concepts: []string{
			"Domain Bootstrapping - 시드에서 도메인 확장",
			"Seed Expansion - 초기 URL에서 관련 URL 발견",
			"Domain Discovery - 새로운 관련 소스 발굴",
			"Ranking - 발견된 소스 순위화",
			"Active Learning - 피드백으로 학습",
		},
		MimirUsage: "bootstrap_vertical, discover_feeds, auto_refine 사이클",
	},
	{
		Name: "Intel Semantic Search",
		Repo: "github.com/IntelLabs/open-domain-question-answering",
		Concepts: []string{
			"Semantic Search - 의미 기반 검색",
			"Dense Retrieval - 임베딩 기반 검색",
			"Question Answering - 질문에 대한 답변 추출",
			"Document Reranking - 검색 결과 재순위화",
		},
		MimirUsage: "embed_docs, 시맨틱 검색, 관련 문서 클러스터링",
	},
}

var VerticalBuildingProcess = []VerticalBuildingGuide{
	{
		Phase:       "1. 도메인 정의",
		Description: "버티컬의 범위와 핵심 개념 정의",
		Techniques: []string{
			"핵심 키워드 추출",
			"관련/비관련 경계 설정",
			"하위 토픽 구조화",
			"언어/지역 범위 설정",
		},
		Tools: []string{"set_keywords", "set_vertical_settings"},
	},
	{
		Phase:       "2. 시드 수집 (Bootstrapping)",
		Description: "초기 고품질 소스 확보",
		Techniques: []string{
			"권위 있는 소스 식별 (학회, 정부, 주요 매체)",
			"RSS/API 가용성 확인",
			"시드 품질 검증",
		},
		Tools: []string{"authoritative_sources", "add_source", "bootstrap_vertical"},
	},
	{
		Phase:       "3. 확장 (Expansion)",
		Description: "시드에서 관련 소스 발굴",
		Techniques: []string{
			"DISCO 스타일 seed expansion",
			"도메인 내 링크 추적",
			"유사 소스 검색",
			"커뮤니티/포럼 탐색",
		},
		Tools: []string{"discover_feeds", "auto_discover_feeds", "bulk_discover"},
	},
	{
		Phase:       "4. 필터링 (Focusing)",
		Description: "도메인 적합도 기반 필터링",
		Techniques: []string{
			"ACHE 스타일 domain classifier",
			"적합도(fit) 측정",
			"노이즈 소스 제거",
			"품질 임계값 설정",
		},
		Tools: []string{"domain_fit", "prune_feeds", "evaluate_feeds"},
	},
	{
		Phase:       "5. 정제 (Refinement)",
		Description: "지속적 품질 개선",
		Techniques: []string{
			"Active learning - 피드백 반영",
			"적합도 추세 모니터링",
			"새 소스 자동 발굴",
			"죽은 소스 정리",
		},
		Tools: []string{"auto_refine", "curate_vertical", "operation_report"},
	},
	{
		Phase:       "6. 서빙 (Serving)",
		Description: "검색 및 브리핑 제공",
		Techniques: []string{
			"FTS + 시맨틱 하이브리드 검색",
			"자동 요약/브리핑",
			"알림/구독",
			"대시보드",
		},
		Tools: []string{"search", "generate_briefing", "generate_dashboard"},
	},
}

var MimirUniqueness = `
## mimir가 다른 점

기존 시스템:
- ACHE: 크롤러 (URL → 페이지)
- DISCO: 도메인 발견 (시드 → 소스)
- 시맨틱 검색: 검색 (쿼리 → 문서)

mimir:
- 버티컬 생성기 (자연어 → 검색엔진)
- "제약 임상시험 추적해줘" → 전용 검색엔진 + 자동 브리핑

핵심 차이:
1. AI가 도메인 정의 (키워드, 범위, 소스 전략)
2. AI가 소스 발굴 (DISCO 개념 + LLM 판단)
3. AI가 품질 관리 (ACHE 개념 + fit 자동 측정)
4. AI가 콘텐츠 생성 (브리핑, 요약, 분석)

결과: 사용자는 "도메인만 말하면" 검색엔진이 생김
`

func GetArchitectureRefs() []ArchitectureRef {
	return ReferenceArchitectures
}

func GetBuildingProcess() []VerticalBuildingGuide {
	return VerticalBuildingProcess
}

func GetMimirUniqueness() string {
	return MimirUniqueness
}
