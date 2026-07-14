package hints

type DomainGuide struct {
	Description     string   `json:"description"`
	PrimarySources  []Source `json:"primary_sources"`
	GitHubSearches  []string `json:"github_searches"`
	WebSearchHints  []string `json:"web_search_hints"`
	RSSFeeds        []string `json:"rss_feeds"`
	APIs            []API    `json:"apis"`
	Keywords        []string `json:"keywords"`
}

type Source struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type API struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Docs    string `json:"docs"`
	Auth    string `json:"auth"`
}

var Domains = map[string]DomainGuide{
	"pharma": {
		Description: "제약/바이오 임상시험, 신약개발, FDA 규제",
		PrimarySources: []Source{
			{Name: "ClinicalTrials.gov", URL: "https://clinicaltrials.gov", Description: "전세계 임상시험 등록 DB"},
			{Name: "PubMed", URL: "https://pubmed.ncbi.nlm.nih.gov", Description: "의학/생명과학 논문"},
			{Name: "FDA", URL: "https://www.fda.gov", Description: "미국 FDA 승인/안전성"},
			{Name: "EMA", URL: "https://www.ema.europa.eu", Description: "유럽 의약품청"},
			{Name: "SEC EDGAR", URL: "https://www.sec.gov/edgar", Description: "제약사 공시 (8-K, 10-K)"},
			{Name: "BioSpace", URL: "https://www.biospace.com", Description: "바이오 업계 뉴스"},
			{Name: "STAT News", URL: "https://www.statnews.com", Description: "헬스케어 심층 보도"},
		},
		GitHubSearches: []string{
			"awesome-pharma",
			"awesome-drug-discovery",
			"clinical-trials-data",
			"cheminformatics",
			"drug-repurposing",
		},
		WebSearchHints: []string{
			"site:nature.com {query}",
			"site:nejm.org {query}",
			"site:thelancet.com {query}",
			"site:clinicaltrials.gov {query}",
			"{drug} phase 3 results",
			"{company} FDA approval",
		},
		RSSFeeds: []string{
			"https://www.biospace.com/rss/news/",
			"https://www.fiercepharma.com/rss/xml",
			"https://www.statnews.com/feed/",
		},
		APIs: []API{
			{Name: "ClinicalTrials.gov", BaseURL: "https://clinicaltrials.gov/api/v2", Docs: "https://clinicaltrials.gov/data-api/api", Auth: "none"},
			{Name: "PubMed E-utilities", BaseURL: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils", Docs: "https://www.ncbi.nlm.nih.gov/books/NBK25501/", Auth: "none (API key optional)"},
			{Name: "openFDA", BaseURL: "https://api.fda.gov", Docs: "https://open.fda.gov/apis/", Auth: "none"},
		},
		Keywords: []string{"임상시험", "clinical trial", "FDA", "신약", "drug approval", "제약", "phase 3", "NDA", "BLA"},
	},

	"ai": {
		Description: "인공지능, 머신러닝, LLM, 딥러닝 연구",
		PrimarySources: []Source{
			{Name: "arXiv", URL: "https://arxiv.org", Description: "AI/ML 프리프린트"},
			{Name: "Papers With Code", URL: "https://paperswithcode.com", Description: "논문 + 코드 + 벤치마크"},
			{Name: "Hugging Face", URL: "https://huggingface.co", Description: "모델/데이터셋 허브"},
			{Name: "Google AI Blog", URL: "https://blog.google/technology/ai/", Description: "구글 AI 연구"},
			{Name: "OpenAI Blog", URL: "https://openai.com/blog", Description: "OpenAI 발표"},
			{Name: "Anthropic News", URL: "https://www.anthropic.com/news", Description: "Anthropic 발표"},
		},
		GitHubSearches: []string{
			"awesome-llm",
			"awesome-machine-learning",
			"awesome-deep-learning",
			"awesome-transformers",
			"{model} implementation",
			"{paper-name} code",
		},
		WebSearchHints: []string{
			"site:arxiv.org {query}",
			"site:paperswithcode.com {query}",
			"site:huggingface.co {query}",
			"{model} benchmark results",
			"{paper} explained",
		},
		RSSFeeds: []string{
			"https://export.arxiv.org/rss/cs.AI",
			"https://export.arxiv.org/rss/cs.LG",
			"https://export.arxiv.org/rss/cs.CL",
			"https://blog.google/technology/ai/rss/",
		},
		APIs: []API{
			{Name: "arXiv API", BaseURL: "https://export.arxiv.org/api", Docs: "https://arxiv.org/help/api", Auth: "none"},
			{Name: "Semantic Scholar", BaseURL: "https://api.semanticscholar.org", Docs: "https://api.semanticscholar.org/api-docs/", Auth: "API key (free)"},
			{Name: "Hugging Face Hub", BaseURL: "https://huggingface.co/api", Docs: "https://huggingface.co/docs/hub/api", Auth: "token (optional)"},
		},
		Keywords: []string{"LLM", "transformer", "GPT", "딥러닝", "machine learning", "neural network", "fine-tuning"},
	},

	"legal": {
		Description: "법률, 판례, 소송, 규제",
		PrimarySources: []Source{
			{Name: "대법원 종합법률정보", URL: "https://glaw.scourt.go.kr", Description: "한국 판례"},
			{Name: "국가법령정보센터", URL: "https://www.law.go.kr", Description: "한국 법령"},
			{Name: "CourtListener", URL: "https://www.courtlistener.com", Description: "미국 판례"},
			{Name: "PACER", URL: "https://pacer.uscourts.gov", Description: "미국 연방법원"},
			{Name: "법률신문", URL: "https://www.lawtimes.co.kr", Description: "한국 법률 뉴스"},
		},
		GitHubSearches: []string{
			"awesome-legal",
			"legal-nlp",
			"contract-analysis",
			"case-law-data",
		},
		WebSearchHints: []string{
			"site:scourt.go.kr {query}",
			"site:law.go.kr {query}",
			"{사건} 판례",
			"{법률} 해석",
		},
		RSSFeeds: []string{
			"https://www.lawtimes.co.kr/rss/allnews.xml",
		},
		APIs: []API{
			{Name: "CourtListener", BaseURL: "https://www.courtlistener.com/api/rest/v3", Docs: "https://www.courtlistener.com/help/api/", Auth: "API key (free)"},
			{Name: "공공데이터포털 법령", BaseURL: "https://www.data.go.kr", Docs: "https://www.data.go.kr", Auth: "API key"},
		},
		Keywords: []string{"판례", "소송", "법원", "법률", "계약", "규제", "litigation", "court"},
	},

	"finance": {
		Description: "금융, 투자, 경제, 시장 분석",
		PrimarySources: []Source{
			{Name: "SEC EDGAR", URL: "https://www.sec.gov/edgar", Description: "미국 기업 공시"},
			{Name: "DART", URL: "https://dart.fss.or.kr", Description: "한국 기업 공시"},
			{Name: "한국은행", URL: "https://www.bok.or.kr", Description: "통화정책/경제지표"},
			{Name: "Bloomberg", URL: "https://www.bloomberg.com", Description: "글로벌 금융 뉴스"},
			{Name: "Reuters", URL: "https://www.reuters.com", Description: "글로벌 뉴스"},
		},
		GitHubSearches: []string{
			"awesome-quant",
			"awesome-finance",
			"stock-prediction",
			"financial-analysis",
			"algorithmic-trading",
		},
		WebSearchHints: []string{
			"site:sec.gov {company} 10-K",
			"site:dart.fss.or.kr {회사}",
			"{ticker} earnings report",
			"{company} investor relations",
		},
		RSSFeeds: []string{
			"https://www.bloomberg.com/feed/podcast/",
			"https://feeds.reuters.com/reuters/businessNews",
		},
		APIs: []API{
			{Name: "SEC EDGAR", BaseURL: "https://efts.sec.gov/LATEST", Docs: "https://www.sec.gov/developer", Auth: "none"},
			{Name: "DART OpenAPI", BaseURL: "https://opendart.fss.or.kr/api", Docs: "https://opendart.fss.or.kr", Auth: "API key"},
			{Name: "Alpha Vantage", BaseURL: "https://www.alphavantage.co/query", Docs: "https://www.alphavantage.co/documentation/", Auth: "API key (free tier)"},
			{Name: "Yahoo Finance", BaseURL: "https://query1.finance.yahoo.com/v8", Docs: "unofficial", Auth: "none"},
		},
		Keywords: []string{"주식", "채권", "금리", "earnings", "공시", "투자", "M&A", "IPO"},
	},

	"tech": {
		Description: "기술, 스타트업, 개발, 오픈소스",
		PrimarySources: []Source{
			{Name: "Hacker News", URL: "https://news.ycombinator.com", Description: "테크 커뮤니티"},
			{Name: "TechCrunch", URL: "https://techcrunch.com", Description: "스타트업 뉴스"},
			{Name: "The Verge", URL: "https://www.theverge.com", Description: "테크 뉴스"},
			{Name: "Ars Technica", URL: "https://arstechnica.com", Description: "심층 기술 기사"},
			{Name: "GitHub Trending", URL: "https://github.com/trending", Description: "인기 오픈소스"},
		},
		GitHubSearches: []string{
			"awesome-{topic}",
			"{technology} examples",
			"{framework} boilerplate",
			"{language} best-practices",
		},
		WebSearchHints: []string{
			"site:news.ycombinator.com {query}",
			"site:dev.to {query}",
			"site:medium.com {query}",
			"{technology} tutorial",
			"{tool} vs {tool}",
		},
		RSSFeeds: []string{
			"https://hnrss.org/frontpage",
			"https://techcrunch.com/feed/",
			"https://feeds.arstechnica.com/arstechnica/index",
		},
		APIs: []API{
			{Name: "GitHub API", BaseURL: "https://api.github.com", Docs: "https://docs.github.com/en/rest", Auth: "token (optional)"},
			{Name: "HN API", BaseURL: "https://hacker-news.firebaseio.com/v0", Docs: "https://github.com/HackerNews/API", Auth: "none"},
			{Name: "Product Hunt", BaseURL: "https://api.producthunt.com/v2", Docs: "https://api.producthunt.com/v2/docs", Auth: "OAuth"},
		},
		Keywords: []string{"startup", "open source", "developer", "API", "framework", "SaaS"},
	},

	"energy": {
		Description: "에너지, 전력, 재생에너지, 탄소중립",
		PrimarySources: []Source{
			{Name: "EIA", URL: "https://www.eia.gov", Description: "미국 에너지정보청"},
			{Name: "IEA", URL: "https://www.iea.org", Description: "국제에너지기구"},
			{Name: "전력거래소", URL: "https://www.kpx.or.kr", Description: "한국 전력시장"},
			{Name: "에너지경제연구원", URL: "https://www.keei.re.kr", Description: "한국 에너지 연구"},
			{Name: "BloombergNEF", URL: "https://about.bnef.com", Description: "에너지 전환 분석"},
		},
		GitHubSearches: []string{
			"awesome-energy",
			"renewable-energy",
			"smart-grid",
			"energy-forecasting",
		},
		WebSearchHints: []string{
			"site:eia.gov {query}",
			"site:iea.org {query}",
			"{energy source} market outlook",
			"{country} renewable energy policy",
		},
		APIs: []API{
			{Name: "EIA API", BaseURL: "https://api.eia.gov/v2", Docs: "https://www.eia.gov/opendata/", Auth: "API key (free)"},
			{Name: "ERCOT", BaseURL: "https://www.ercot.com/api", Docs: "https://www.ercot.com/services/data", Auth: "varies"},
		},
		Keywords: []string{"전력", "재생에너지", "태양광", "풍력", "ESS", "탄소중립", "grid", "solar", "wind"},
	},

	"food": {
		Description: "식품, 요리, 레스토랑, 푸드테크",
		PrimarySources: []Source{
			{Name: "Eater", URL: "https://www.eater.com", Description: "레스토랑/푸드 뉴스"},
			{Name: "Serious Eats", URL: "https://www.seriouseats.com", Description: "요리 과학"},
			{Name: "Food & Wine", URL: "https://www.foodandwine.com", Description: "음식 문화"},
			{Name: "식품의약품안전처", URL: "https://www.mfds.go.kr", Description: "한국 식품 규제"},
		},
		GitHubSearches: []string{
			"awesome-food",
			"recipe-dataset",
			"food-recognition",
			"nutrition-api",
		},
		WebSearchHints: []string{
			"site:seriouseats.com {query}",
			"{ingredient} recipe",
			"{cuisine} restaurant {city}",
		},
		APIs: []API{
			{Name: "Spoonacular", BaseURL: "https://api.spoonacular.com", Docs: "https://spoonacular.com/food-api/docs", Auth: "API key"},
			{Name: "USDA FoodData", BaseURL: "https://api.nal.usda.gov/fdc/v1", Docs: "https://fdc.nal.usda.gov/api-guide.html", Auth: "API key (free)"},
		},
		Keywords: []string{"레시피", "식당", "요리", "식품", "nutrition", "restaurant", "chef"},
	},

	"politics": {
		Description: "정치, 정책, 선거, 국제관계",
		PrimarySources: []Source{
			{Name: "국회 의안정보", URL: "https://likms.assembly.go.kr/bill", Description: "한국 법안"},
			{Name: "대한민국 정책브리핑", URL: "https://www.korea.kr", Description: "정부 정책"},
			{Name: "Congress.gov", URL: "https://www.congress.gov", Description: "미국 의회"},
			{Name: "Politico", URL: "https://www.politico.com", Description: "미국 정치"},
			{Name: "Foreign Affairs", URL: "https://www.foreignaffairs.com", Description: "국제관계"},
		},
		GitHubSearches: []string{
			"awesome-political-science",
			"election-data",
			"policy-analysis",
			"congress-api",
		},
		WebSearchHints: []string{
			"site:assembly.go.kr {법안}",
			"site:congress.gov {bill}",
			"{정책} 입법예고",
			"{country} foreign policy",
		},
		APIs: []API{
			{Name: "국회 열린국회정보", BaseURL: "https://open.assembly.go.kr/portal/openapi", Docs: "https://open.assembly.go.kr/portal/openapi/main.do", Auth: "API key"},
			{Name: "Congress.gov API", BaseURL: "https://api.congress.gov/v3", Docs: "https://api.congress.gov/", Auth: "API key (free)"},
			{Name: "ProPublica Congress", BaseURL: "https://api.propublica.org/congress/v1", Docs: "https://projects.propublica.org/api-docs/congress-api/", Auth: "API key"},
		},
		Keywords: []string{"정치", "국회", "선거", "정책", "법안", "외교", "congress", "election"},
	},
}

func Get(domain string) (DomainGuide, bool) {
	guide, ok := Domains[domain]
	return guide, ok
}

func List() []string {
	var domains []string
	for k := range Domains {
		domains = append(domains, k)
	}
	return domains
}
