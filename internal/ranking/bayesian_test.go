package ranking

import (
	"testing"
)

func TestNewBayesianRanker(t *testing.T) {
	br := NewBayesianRanker(2.0)

	if br == nil {
		t.Fatal("NewBayesianRanker returned nil")
	}

	if br.C != 2.0 {
		t.Errorf("expected C 2.0, got %f", br.C)
	}

	if len(br.PositiveDocs) != 0 {
		t.Errorf("expected 0 positive docs initially, got %d", len(br.PositiveDocs))
	}
}

func TestBayesianRankerAddPositive(t *testing.T) {
	br := NewBayesianRanker(1.0)

	br.AddPositive("machine learning paper")
	br.AddPositive("deep neural networks")

	if len(br.PositiveDocs) != 2 {
		t.Errorf("expected 2 positive docs, got %d", len(br.PositiveDocs))
	}
}

func TestBayesianRankerRank(t *testing.T) {
	br := NewBayesianRanker(1.0)

	// Add more positive examples with overlapping terms (needed for vocabulary building)
	br.AddPositive("machine learning algorithm neural network deep learning")
	br.AddPositive("deep learning transformer model neural network")
	br.AddPositive("artificial intelligence neural network machine learning")
	br.AddPositive("neural network training algorithm machine learning")
	br.AddPositive("deep learning model architecture neural network")

	// Candidates to rank
	candidates := []map[string]interface{}{
		{"text": "machine learning neural network training", "id": 1},
		{"text": "cooking recipes for beginners kitchen", "id": 2},
		{"text": "neural network deep learning architecture", "id": 3},
		{"text": "weather forecast for tomorrow sunny", "id": 4},
	}

	ranked := br.Rank(candidates)

	if len(ranked) != 4 {
		t.Fatalf("expected 4 ranked results, got %d", len(ranked))
	}

	// Check that ranked items have valid scores
	for _, r := range ranked {
		if r.BayesianScore != r.BayesianScore { // NaN check
			t.Errorf("score should not be NaN for item %d", r.Index)
		}
	}

	// AI-related candidates (id 1, 3) should have higher scores than unrelated (id 2, 4)
	var aiScoreSum, otherScoreSum float64
	var aiCount, otherCount int
	for _, r := range ranked {
		data := r.Data.(map[string]interface{})
		id := data["id"].(int)
		if id == 1 || id == 3 {
			aiScoreSum += r.BayesianScore
			aiCount++
		} else {
			otherScoreSum += r.BayesianScore
			otherCount++
		}
	}

	aiAvg := aiScoreSum / float64(aiCount)
	otherAvg := otherScoreSum / float64(otherCount)

	if aiAvg <= otherAvg {
		t.Errorf("expected AI content average score (%f) > other content (%f)", aiAvg, otherAvg)
	}
}

func TestBayesianRankerEmptyPositives(t *testing.T) {
	br := NewBayesianRanker(1.0)

	candidates := []map[string]interface{}{
		{"text": "any text", "id": 1},
	}

	// Should not panic with no positive examples
	ranked := br.Rank(candidates)

	if len(ranked) != 1 {
		t.Errorf("expected 1 result, got %d", len(ranked))
	}
}

func TestBayesianRankerEmptyCandidates(t *testing.T) {
	br := NewBayesianRanker(1.0)
	br.AddPositive("some positive example")

	candidates := []map[string]interface{}{}

	ranked := br.Rank(candidates)

	if len(ranked) != 0 {
		t.Errorf("expected 0 results for empty candidates, got %d", len(ranked))
	}
}

func TestBayesianRankerScoresAreValid(t *testing.T) {
	br := NewBayesianRanker(1.0)
	br.AddPositive("test document")

	candidates := []map[string]interface{}{
		{"text": "test document similar"},
		{"text": "completely different"},
	}

	ranked := br.Rank(candidates)

	for _, r := range ranked {
		// Scores should be finite numbers
		if r.BayesianScore != r.BayesianScore { // NaN check
			t.Error("score is NaN")
		}
	}
}

func TestBayesianRankerPreservesData(t *testing.T) {
	br := NewBayesianRanker(1.0)
	br.AddPositive("example")

	candidates := []map[string]interface{}{
		{"text": "test", "id": 42, "extra": "data"},
	}

	ranked := br.Rank(candidates)

	if len(ranked) != 1 {
		t.Fatalf("expected 1 result, got %d", len(ranked))
	}

	data := ranked[0].Data.(map[string]interface{})
	if data["id"] != 42 {
		t.Errorf("expected id 42, got %v", data["id"])
	}
	if data["extra"] != "data" {
		t.Errorf("expected extra 'data', got %v", data["extra"])
	}
}

func TestBayesianRankerDifferentAlpha(t *testing.T) {
	// Higher alpha = more smoothing
	br1 := NewBayesianRanker(0.1)
	br2 := NewBayesianRanker(10.0)

	br1.AddPositive("machine learning")
	br2.AddPositive("machine learning")

	candidates := []map[string]interface{}{
		{"text": "machine learning is great"},
	}

	ranked1 := br1.Rank(candidates)
	ranked2 := br2.Rank(candidates)

	// Both should produce valid rankings
	if len(ranked1) != 1 || len(ranked2) != 1 {
		t.Fatal("expected 1 result each")
	}

	// Scores might differ but both should be valid
	if ranked1[0].BayesianScore != ranked1[0].BayesianScore {
		t.Error("ranked1 score is NaN")
	}
	if ranked2[0].BayesianScore != ranked2[0].BayesianScore {
		t.Error("ranked2 score is NaN")
	}
}

func TestBayesianRankerLargeInput(t *testing.T) {
	br := NewBayesianRanker(1.0)

	// Add many positive examples
	for i := 0; i < 100; i++ {
		br.AddPositive("machine learning deep learning neural network AI")
	}

	// Many candidates
	candidates := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		candidates[i] = map[string]interface{}{
			"text": "some candidate text about various topics",
			"id":   i,
		}
	}

	ranked := br.Rank(candidates)

	if len(ranked) != 100 {
		t.Errorf("expected 100 results, got %d", len(ranked))
	}
}
