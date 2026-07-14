package ranking

import (
	"math"
	"sort"
	"strings"
)

// BayesianRanker - Bayesian Sets 기반 소스 랭킹
// DISCO 방식 참고: "이미 좋은 소스"와 비슷한 후보에 높은 점수
type BayesianRanker struct {
	C            float64 // concentration parameter
	PositiveDocs [][]string
	Vocabulary   map[string]bool
	DocFreq      map[string]int
}

func NewBayesianRanker(c float64) *BayesianRanker {
	if c <= 0 {
		c = 2.0
	}
	return &BayesianRanker{
		C:          c,
		Vocabulary: make(map[string]bool),
		DocFreq:    make(map[string]int),
	}
}

// AddPositive adds a good source for learning
func (br *BayesianRanker) AddPositive(text string) {
	tokens := tokenize(strings.ToLower(text))
	if len(tokens) == 0 {
		return
	}

	br.PositiveDocs = append(br.PositiveDocs, tokens)

	seen := make(map[string]bool)
	for _, t := range tokens {
		br.Vocabulary[t] = true
		if !seen[t] {
			br.DocFreq[t]++
			seen[t] = true
		}
	}
}

// AddPositives adds multiple good sources
func (br *BayesianRanker) AddPositives(texts []string) {
	for _, text := range texts {
		br.AddPositive(text)
	}
}

// RankedItem represents a ranked candidate
type RankedItem struct {
	Index         int
	Text          string
	Data          interface{}
	BayesianScore float64
}

// Rank ranks candidates by Bayesian Sets similarity to positive examples
func (br *BayesianRanker) Rank(candidates []map[string]interface{}) []RankedItem {
	if len(br.PositiveDocs) == 0 {
		result := make([]RankedItem, len(candidates))
		for i, c := range candidates {
			text, _ := c["text"].(string)
			if text == "" {
				text, _ = c["title"].(string)
			}
			result[i] = RankedItem{Index: i, Text: text, Data: c, BayesianScore: 0.5}
		}
		return result
	}

	// Build vocabulary (tokens in >= 2 docs)
	vocab := make([]string, 0)
	for t, freq := range br.DocFreq {
		if freq >= 2 {
			vocab = append(vocab, t)
		}
	}
	if len(vocab) == 0 {
		i := 0
		for t := range br.Vocabulary {
			vocab = append(vocab, t)
			i++
			if i >= 100 {
				break
			}
		}
	}

	// Build TF-IDF matrix for positive docs
	allDocs := make([][]string, len(br.PositiveDocs))
	copy(allDocs, br.PositiveDocs)

	candidateTokens := make([][]string, len(candidates))
	for i, c := range candidates {
		text, _ := c["text"].(string)
		if text == "" {
			text, _ = c["title"].(string)
		}
		tokens := tokenize(strings.ToLower(text))
		candidateTokens[i] = tokens
		allDocs = append(allDocs, tokens)
	}

	// Compute IDF for vocabulary
	idf := make(map[string]float64)
	totalDocs := float64(len(allDocs))
	for _, term := range vocab {
		docCount := 0
		for _, doc := range allDocs {
			for _, t := range doc {
				if t == term {
					docCount++
					break
				}
			}
		}
		idf[term] = math.Log(totalDocs / float64(docCount+1))
	}

	// Compute TF-IDF vectors
	tfidfVectors := make([]map[string]float64, len(allDocs))
	for i, doc := range allDocs {
		tfidfVectors[i] = computeTfidfVector(doc, vocab, idf)
	}

	nPositive := len(br.PositiveDocs)

	// Compute mean positive vector
	meanPosVec := make(map[string]float64)
	for i := 0; i < nPositive; i++ {
		for term, val := range tfidfVectors[i] {
			meanPosVec[term] += val
		}
	}
	for term := range meanPosVec {
		meanPosVec[term] /= float64(nPositive)
	}

	// Score candidates using Bayesian Sets formula
	result := make([]RankedItem, len(candidates))
	for i, c := range candidates {
		text, _ := c["text"].(string)
		if text == "" {
			text, _ = c["title"].(string)
		}

		candVec := tfidfVectors[nPositive+i]
		score := br.bayesianScore(candVec, meanPosVec, vocab)

		result[i] = RankedItem{
			Index:         i,
			Text:          text,
			Data:          c,
			BayesianScore: score,
		}
	}

	// Sort by score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].BayesianScore > result[j].BayesianScore
	})

	return result
}

func (br *BayesianRanker) bayesianScore(candVec, meanPosVec map[string]float64, vocab []string) float64 {
	// Bayesian Sets: P(x|D+) / P(x)
	// Simplified: cosine similarity + term overlap bonus

	// Cosine similarity
	cosSim := cosineSimilarity(candVec, meanPosVec)

	// Term overlap with positive vocabulary
	var overlap float64
	for term, val := range candVec {
		if _, ok := meanPosVec[term]; ok && val > 0 {
			overlap += 1.0
		}
	}

	vocabSize := float64(len(vocab))
	if vocabSize == 0 {
		vocabSize = 1
	}
	overlapRatio := overlap / vocabSize

	// Bayesian combination with concentration parameter
	alpha := br.C / (br.C + float64(len(br.PositiveDocs)))
	score := alpha*0.5 + (1-alpha)*(0.7*cosSim+0.3*overlapRatio)

	return math.Min(1.0, math.Max(0.0, score))
}

func computeTfidfVector(doc []string, vocab []string, idf map[string]float64) map[string]float64 {
	tf := termFrequency(doc)
	vec := make(map[string]float64)

	for _, term := range vocab {
		if tfVal, ok := tf[term]; ok {
			vec[term] = tfVal * idf[term]
		}
	}

	return vec
}

// ScoreText scores a single text against positive examples
func (br *BayesianRanker) ScoreText(text string) float64 {
	candidates := []map[string]interface{}{{"text": text}}
	ranked := br.Rank(candidates)
	if len(ranked) > 0 {
		return ranked[0].BayesianScore
	}
	return 0.0
}
