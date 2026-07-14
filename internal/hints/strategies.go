package hints

type ExplorationStrategy struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []Step   `json:"steps"`
	Patterns    []string `json:"patterns"`
}

type Step struct {
	Action    string   `json:"action"`
	Target    string   `json:"target"`
	Questions []string `json:"questions"`
	Outputs   []string `json:"outputs"`
}

var DomainStrategies = map[string]ExplorationStrategy{
	"pharma": {
		Name:        "제약/바이오 심층 탐색",
		Description: "신약개발 파이프라인을 따라 정보를 수집하고 연결",
		Steps: []Step{
			{
				Action: "파이프라인 매핑",
				Target: "ClinicalTrials.gov",
				Questions: []string{
					"이 약물의 현재 개발 단계는?",
					"Phase 1/2/3 각각 몇 건 진행 중?",
					"어떤 적응증을 타겟?",
				},
				Outputs: []string{"NCT ID 목록", "스폰서-적응증 매트릭스"},
			},
			{
				Action: "과학적 근거 확인",
				Target: "PubMed",
				Questions: []string{
					"작용 기전(MoA)은?",
					"전임상/임상 결과 논문은?",
					"경쟁 약물 대비 장단점?",
				},
				Outputs: []string{"핵심 논문 목록", "MoA 요약"},
			},
			{
				Action: "규제 현황 파악",
				Target: "FDA/EMA",
				Questions: []string{
					"기존 승인 이력?",
					"FDA 자문위원회 일정?",
					"PDUFA 날짜?",
				},
				Outputs: []string{"규제 타임라인", "승인 확률 평가"},
			},
			{
				Action: "기업 전략 분석",
				Target: "SEC EDGAR / IR",
				Questions: []string{
					"최근 8-K에서 임상 관련 공시?",
					"파트너십/라이센싱 계약?",
					"R&D 투자 규모?",
				},
				Outputs: []string{"기업 전략 요약", "재무 영향 분석"},
			},
			{
				Action: "시장 맥락 이해",
				Target: "뉴스/애널리스트 리포트",
				Questions: []string{
					"경쟁 환경?",
					"시장 규모 전망?",
					"주요 리스크?",
				},
				Outputs: []string{"경쟁 지형도", "시장 전망"},
			},
		},
		Patterns: []string{
			"약물 → 적응증 → 임상시험 → 결과 → 규제 → 상업화",
			"기업 → 파이프라인 → 핵심 자산 → 마일스톤",
			"질환 → 현재 치료법 → 미충족 수요 → 개발 중인 약물",
		},
	},

	"ai": {
		Name:        "AI 연구 심층 탐색",
		Description: "논문에서 시작해 코드, 벤치마크, 응용까지 추적",
		Steps: []Step{
			{
				Action: "핵심 논문 파악",
				Target: "arXiv / Semantic Scholar",
				Questions: []string{
					"이 분야의 seminal paper?",
					"최신 SOTA 논문?",
					"인용 네트워크?",
				},
				Outputs: []string{"논문 계보", "핵심 저자/랩"},
			},
			{
				Action: "구현 확인",
				Target: "Papers With Code / GitHub",
				Questions: []string{
					"공식 구현 있음?",
					"재현 가능?",
					"어떤 프레임워크?",
				},
				Outputs: []string{"코드 링크", "재현 노트"},
			},
			{
				Action: "벤치마크 비교",
				Target: "Papers With Code / 논문",
				Questions: []string{
					"어떤 벤치마크에서 평가?",
					"SOTA 대비 성능?",
					"compute 요구사항?",
				},
				Outputs: []string{"성능 비교표", "효율성 분석"},
			},
			{
				Action: "실용성 평가",
				Target: "Hugging Face / 블로그",
				Questions: []string{
					"프로덕션 적용 사례?",
					"fine-tuning 난이도?",
					"라이센스?",
				},
				Outputs: []string{"적용 가이드", "제약사항"},
			},
		},
		Patterns: []string{
			"문제정의 → 기존방법 → 새방법 → 실험 → 한계",
			"아키텍처 → 학습방법 → 데이터 → 스케일링",
			"이론 → 구현 → 벤치마크 → 응용",
		},
	},

	"finance": {
		Name:        "금융/기업 심층 분석",
		Description: "기업 공시에서 시작해 시장 맥락까지 확장",
		Steps: []Step{
			{
				Action: "공시 분석",
				Target: "SEC EDGAR / DART",
				Questions: []string{
					"최근 10-K/사업보고서 핵심?",
					"8-K에서 중요 이벤트?",
					"MD&A에서 경영진 시각?",
				},
				Outputs: []string{"재무 요약", "리스크 요인"},
			},
			{
				Action: "실적 추적",
				Target: "어닝콜 / IR",
				Questions: []string{
					"가이던스 vs 실적?",
					"애널리스트 Q&A 핵심?",
					"향후 전망?",
				},
				Outputs: []string{"실적 트렌드", "경영진 코멘트"},
			},
			{
				Action: "산업 맥락",
				Target: "산업 리포트 / 뉴스",
				Questions: []string{
					"섹터 전체 트렌드?",
					"경쟁사 비교?",
					"거시경제 영향?",
				},
				Outputs: []string{"산업 포지셔닝", "경쟁 분석"},
			},
			{
				Action: "밸류에이션",
				Target: "재무 데이터",
				Questions: []string{
					"P/E, EV/EBITDA 등 멀티플?",
					"DCF 가정?",
					"peer 대비?",
				},
				Outputs: []string{"밸류에이션 요약"},
			},
		},
		Patterns: []string{
			"공시 → 실적 → 가이던스 → 주가 반응",
			"매크로 → 섹터 → 기업 → 밸류에이션",
			"이벤트 → 영향 분석 → 시나리오",
		},
	},

	"legal": {
		Name:        "법률 리서치 전략",
		Description: "쟁점에서 시작해 판례, 법령, 학설까지 탐색",
		Steps: []Step{
			{
				Action: "쟁점 정리",
				Target: "사실관계",
				Questions: []string{
					"법적 쟁점은?",
					"관련 법령은?",
					"당사자 주장?",
				},
				Outputs: []string{"쟁점 목록", "법령 목록"},
			},
			{
				Action: "판례 검색",
				Target: "대법원/하급심",
				Questions: []string{
					"선례 있음?",
					"판례 경향?",
					"최근 변화?",
				},
				Outputs: []string{"관련 판례", "판례 분석"},
			},
			{
				Action: "학설 검토",
				Target: "논문/주석서",
				Questions: []string{
					"다수설/소수설?",
					"학계 비판?",
					"입법론?",
				},
				Outputs: []string{"학설 정리"},
			},
			{
				Action: "실무 확인",
				Target: "실무자료/가이드",
				Questions: []string{
					"실무 처리 방식?",
					"관할/절차?",
					"비용/기간?",
				},
				Outputs: []string{"실무 가이드"},
			},
		},
		Patterns: []string{
			"사실 → 쟁점 → 법령 → 판례 → 결론",
			"법령 → 판례 → 학설 → 입법론",
			"분쟁 → 절차 → 증거 → 판단기준",
		},
	},

	"tech": {
		Name:        "기술 리서치 전략",
		Description: "기술 선택부터 프로덕션까지",
		Steps: []Step{
			{
				Action: "기술 조사",
				Target: "공식문서 / GitHub",
				Questions: []string{
					"핵심 기능?",
					"아키텍처?",
					"성숙도?",
				},
				Outputs: []string{"기술 개요", "장단점"},
			},
			{
				Action: "대안 비교",
				Target: "비교 글 / 벤치마크",
				Questions: []string{
					"대안은?",
					"각각 장단점?",
					"사용 사례별 추천?",
				},
				Outputs: []string{"비교표", "선택 가이드"},
			},
			{
				Action: "실제 사용 사례",
				Target: "블로그 / 케이스스터디",
				Questions: []string{
					"프로덕션 사례?",
					"마이그레이션 경험?",
					"문제점?",
				},
				Outputs: []string{"사례 분석", "lessons learned"},
			},
			{
				Action: "커뮤니티 평가",
				Target: "HN / Reddit / Discord",
				Questions: []string{
					"커뮤니티 평판?",
					"자주 나오는 문제?",
					"로드맵?",
				},
				Outputs: []string{"커뮤니티 평가"},
			},
		},
		Patterns: []string{
			"문제 → 요구사항 → 후보 → 비교 → 선택",
			"POC → 파일럿 → 프로덕션 → 최적화",
			"도입 → 문제발생 → 해결 → 안정화",
		},
	},
}

func GetStrategy(domain string) (ExplorationStrategy, bool) {
	strategy, ok := DomainStrategies[domain]
	return strategy, ok
}
