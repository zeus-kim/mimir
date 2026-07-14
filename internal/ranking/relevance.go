package ranking

import (
	"math"
	"regexp"
	"strings"
)

// RelevanceClassifier - TF-IDF 기반 관련성 분류
// ACHE 방식 참고: TF-IDF 벡터로 페이지 표현, 코사인 유사도로 관련성 판단
type RelevanceClassifier struct {
	Topic          string
	Keywords       []string
	KeywordWeights map[string]float64
	PositiveDocs   [][]string
	NegativeDocs   [][]string
	IdfCache       map[string]float64
}

func NewRelevanceClassifier(keywords []string, topic string) *RelevanceClassifier {
	rc := &RelevanceClassifier{
		Topic:          topic,
		Keywords:       make([]string, len(keywords)),
		KeywordWeights: make(map[string]float64),
		IdfCache:       make(map[string]float64),
	}

	for i, k := range keywords {
		rc.Keywords[i] = strings.ToLower(k)
	}

	rc.buildKeywordWeights()
	return rc
}

func (rc *RelevanceClassifier) buildKeywordWeights() {
	total := float64(len(rc.Keywords))
	for i, kw := range rc.Keywords {
		weight := 1.0 + (total-float64(i))/total
		rc.KeywordWeights[kw] = weight
	}
}

// Score returns relevance score (0.0 ~ 1.0)
func (rc *RelevanceClassifier) Score(text string) float64 {
	if text == "" {
		return 0.0
	}

	textLower := strings.ToLower(text)
	tokens := tokenize(textLower)

	if len(tokens) == 0 {
		return 0.0
	}

	keywordScore := rc.keywordScore(textLower, tokens)

	var tfidfScore float64
	if len(rc.PositiveDocs) > 0 {
		tfidfScore = rc.tfidfScore(tokens)
		return 0.6*keywordScore + 0.4*tfidfScore
	}

	return keywordScore
}

func (rc *RelevanceClassifier) keywordScore(text string, tokens []string) float64 {
	if len(rc.Keywords) == 0 {
		return 0.5
	}

	var totalWeight, matchedWeight float64
	for _, w := range rc.KeywordWeights {
		totalWeight += w
	}

	tokenSet := make(map[string]bool)
	for _, t := range tokens {
		tokenSet[t] = true
	}

	for kw, weight := range rc.KeywordWeights {
		if tokenSet[kw] {
			matchedWeight += weight
		} else if strings.Contains(text, kw) {
			matchedWeight += weight * 0.8
		} else {
			for t := range tokenSet {
				if strings.Contains(t, kw) {
					matchedWeight += weight * 0.5
					break
				}
			}
		}
	}

	if totalWeight > 0 {
		return math.Min(1.0, matchedWeight/totalWeight)
	}
	return 0.0
}

func (rc *RelevanceClassifier) tfidfScore(tokens []string) float64 {
	if len(rc.PositiveDocs) == 0 {
		return 0.0
	}

	docVec := rc.tfidfVector(tokens)

	var totalSim float64
	for _, posDoc := range rc.PositiveDocs {
		posVec := rc.tfidfVector(posDoc)
		sim := cosineSimilarity(docVec, posVec)
		totalSim += sim
	}

	return totalSim / float64(len(rc.PositiveDocs))
}

func (rc *RelevanceClassifier) tfidfVector(tokens []string) map[string]float64 {
	tf := termFrequency(tokens)
	vec := make(map[string]float64)

	for term, freq := range tf {
		idf := rc.getIdf(term)
		vec[term] = freq * idf
	}

	return vec
}

func (rc *RelevanceClassifier) getIdf(term string) float64 {
	if idf, ok := rc.IdfCache[term]; ok {
		return idf
	}

	docCount := 0
	for _, doc := range rc.PositiveDocs {
		for _, t := range doc {
			if t == term {
				docCount++
				break
			}
		}
	}

	totalDocs := len(rc.PositiveDocs) + 1
	idf := math.Log(float64(totalDocs) / float64(docCount+1))
	rc.IdfCache[term] = idf

	return idf
}

// AddPositive adds a positive example for learning
func (rc *RelevanceClassifier) AddPositive(text string) {
	tokens := tokenize(strings.ToLower(text))
	if len(tokens) > 0 {
		rc.PositiveDocs = append(rc.PositiveDocs, tokens)
		rc.IdfCache = make(map[string]float64)
	}
}

// AddNegative adds a negative example for learning
func (rc *RelevanceClassifier) AddNegative(text string) {
	tokens := tokenize(strings.ToLower(text))
	if len(tokens) > 0 {
		rc.NegativeDocs = append(rc.NegativeDocs, tokens)
	}
}

// Classify returns true if text is relevant (score >= threshold)
func (rc *RelevanceClassifier) Classify(text string, threshold float64) bool {
	return rc.Score(text) >= threshold
}

// Helper functions

var tokenRegex = regexp.MustCompile(`[\p{L}\p{N}]+`)

func tokenize(text string) []string {
	matches := tokenRegex.FindAllString(text, -1)
	var tokens []string
	for _, m := range matches {
		if len(m) >= 2 {
			tokens = append(tokens, m)
		}
	}
	return tokens
}

func termFrequency(tokens []string) map[string]float64 {
	counts := make(map[string]int)
	for _, t := range tokens {
		counts[t]++
	}

	tf := make(map[string]float64)
	maxCount := 1
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	for term, count := range counts {
		tf[term] = 0.5 + 0.5*float64(count)/float64(maxCount)
	}

	return tf
}

func cosineSimilarity(a, b map[string]float64) float64 {
	var dotProduct, normA, normB float64

	for term, valA := range a {
		normA += valA * valA
		if valB, ok := b[term]; ok {
			dotProduct += valA * valB
		}
	}

	for _, valB := range b {
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
