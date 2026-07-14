# mimir

[English](README.md) | [한국어](README.ko.md)

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/Tests-134%20passed-success)](https://github.com/zeus-kim/mimir)

**버티컬 검색엔진 팩토리** — 자연어로 도메인 특화 검색엔진을 생성하세요.

버티컬 인텔리전스 시스템을 구축, 관리, 운영하기 위한 독립형 MCP(Model Context Protocol) 서버. 단일 바이너리, 외부 의존성 없음.

## 주요 기능

- **8개 도메인 Fetcher**: 제약, AI/ML, 법률, 금융, 에너지, 식품, 정치, 기술
- **Key-Free API**: 대부분의 API가 등록 없이 작동
- **ACHE/DISCO 알고리즘**: 학술 수준의 TF-IDF 관련성 점수 및 베이지안 랭킹
- **버티컬 관리**: 다중 검색엔진 생성, 설정, 관리
- **다국어 지원**: 7개 언어 (EN, KO, JA, ZH, ES, FR, DE)
- **프로덕션 준비**: 헬스체크, 메트릭, 구조화된 로깅, Docker 지원

## 설치

### 소스에서 빌드

```bash
git clone https://github.com/zeus-kim/mimir.git
cd mimir
make build
```

### Go 사용

```bash
go install github.com/zeus-kim/mimir/cmd/mimir-mcp@latest
```

### Docker

```bash
docker build -t mimir .
docker run -p 8080:8080 -v ~/.mimir:/data mimir
```

## 빠른 시작

```bash
# 빌드
make build

# MCP 서버 실행
./bin/mimir-mcp

# CLI 명령어
./bin/mimir-mcp help
./bin/mimir-mcp vertical list
./bin/mimir-mcp vertical create my-pharma --domain pharma
./bin/mimir-mcp health
./bin/mimir-mcp metrics
```

## CLI 명령어

```
mimir-mcp [command]

Commands:
  serve              MCP 서버 시작 (기본값)
  vertical, v        버티컬 관리
    list             모든 버티컬 목록
    create <name>    버티컬 생성 (--domain, --keywords, --languages)
    show <name>      버티컬 상세 정보
    delete <name>    버티컬 삭제
    stats <name>     통계 보기
  health             헬스 체크
  metrics            메트릭 조회
  config             설정 관리
  version            버전 정보
  help               도움말
```

## 도메인 API

### Key-Free (등록 불필요)

| 도메인 | API |
|--------|-----|
| **제약** | ClinicalTrials.gov, PubMed, FDA, SEC EDGAR |
| **AI/ML** | arXiv, Semantic Scholar, HuggingFace, Papers With Code |
| **법률** | Federal Register, CourtListener |
| **금융** | Yahoo Finance, SEC EDGAR |
| **식품** | Open Food Facts, TheMealDB |
| **에너지** | ERCOT (텍사스 전력망) |
| **기술** | GitHub Trending, HackerNews, DevTo |

### 선택적 (무료 API 키 필요)

| 도메인 | API | 환경변수 |
|--------|-----|----------|
| 금융 | FRED | `FRED_API_KEY` |
| 에너지 | EIA, ENTSO-E | `EIA_API_KEY`, `ENTSOE_API_KEY` |
| 정치 | Congress.gov, ProPublica | `CONGRESS_API_KEY`, `PROPUBLICA_API_KEY` |
| 식품 | USDA, Spoonacular | `USDA_API_KEY`, `SPOONACULAR_KEY` |

## MCP 도구

### 버티컬 관리
- `create_vertical` — 도메인 프리셋 또는 커스텀으로 생성
- `list_verticals` — 모든 인스턴스 목록
- `get_vertical` — 설정 및 통계 조회
- `delete_vertical` — 버티컬 삭제
- `vertical_stats` — 문서 수, 피드 수, 적합도%

### 도메인 Fetcher
- `fetch_ai_research` — arXiv, Semantic Scholar, HuggingFace
- `fetch_legal` — Federal Register, CourtListener
- `fetch_finance` — Yahoo Finance, SEC, FRED
- `fetch_energy` — ERCOT, EIA, ENTSO-E
- `fetch_food` — Open Food Facts, TheMealDB
- `fetch_politics` — Congress.gov, ProPublica
- `fetch_clinical_trials` — ClinicalTrials.gov
- `fetch_pubmed` — PubMed 논문
- `fetch_fda_approvals` — FDA 약물 승인

### 시스템
- `health` — 헬스 체크
- `metrics` — 서버 메트릭
- `api_status` — 사용 가능한 API 확인

## 설정

### 환경변수

```bash
MIMIR_LANGUAGE=ko          # UI 언어
MIMIR_LOG_LEVEL=info       # debug|info|warn|error
MIMIR_DATA_DIR=~/.mimir    # 데이터 디렉토리
```

### 설정 파일

```json
{
  "data_dir": "~/.mimir-verticals",
  "server": { "language": "ko" },
  "logging": { "level": "info", "format": "json" },
  "verticals": { "min_fit_percent": 50.0, "max_feeds": 200 }
}
```

## Claude Desktop 연동

`claude_desktop_config.json`에 추가:

```json
{
  "mcpServers": {
    "mimir": {
      "command": "/path/to/mimir-mcp",
      "args": ["-config", "/path/to/config.json"]
    }
  }
}
```

## 아키텍처

```
mimir/
├── cmd/mimir-mcp/      # 진입점 (CLI + MCP 서버)
├── internal/
│   ├── config/         # 설정
│   ├── db/             # SQLite + FTS5
│   ├── fetch/          # 도메인 API fetcher
│   ├── health/         # 헬스 체크
│   ├── httpclient/     # HTTP (재시도/레이트리밋)
│   ├── i18n/           # 다국어
│   ├── logger/         # 구조화된 로깅
│   ├── metrics/        # 메트릭 수집
│   ├── ranking/        # ACHE/DISCO 알고리즘
│   ├── tools/          # MCP 도구 레지스트리
│   └── vertical/       # 버티컬 관리
├── Dockerfile
├── Makefile
└── README.md
```

## 개발

```bash
make test          # 테스트 실행
make test-coverage # 커버리지 리포트
make lint          # 코드 린트
make build-all     # 전체 플랫폼 빌드
make docker        # Docker 이미지 빌드
```

## 참고 자료

- [ACHE](https://github.com/ViDA-NYU/ache) — 도메인 분류기를 갖춘 포커스드 크롤러
- [DISCO](https://github.com/ViDA-NYU/domain-discovery-crawler) — 도메인 발견 및 시드 확장
- [MCP](https://modelcontextprotocol.io/) — Model Context Protocol

## 라이센스

MIT
