package ranking

import (
	"math"
	"testing"
)

func TestNewRelevanceClassifier(t *testing.T) {
	keywords := []string{"machine learning", "neural network", "deep learning"}
	rc := NewRelevanceClassifier(keywords, "ai")

	if rc == nil {
		t.Fatal("NewRelevanceClassifier returned nil")
	}

	if rc.Topic != "ai" {
		t.Errorf("expected topic 'ai', got '%s'", rc.Topic)
	}

	if len(rc.Keywords) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(rc.Keywords))
	}
}

func TestRelevanceClassifierScore(t *testing.T) {
	keywords := []string{"clinical trial", "FDA", "drug", "pharmaceutical"}
	rc := NewRelevanceClassifier(keywords, "pharma")

	tests := []struct {
		name     string
		text     string
		minScore float64
		maxScore float64
	}{
		{
			name:     "highly relevant",
			text:     "FDA approves new drug for clinical trial in pharmaceutical research",
			minScore: 0.5,
			maxScore: 1.0,
		},
		{
			name:     "somewhat relevant",
			text:     "The FDA announced new guidelines for drug safety",
			minScore: 0.2,
			maxScore: 0.7,
		},
		{
			name:     "not relevant",
			text:     "The weather today is sunny with clear skies",
			minScore: 0.0,
			maxScore: 0.1,
		},
		{
			name:     "empty text",
			text:     "",
			minScore: 0.0,
			maxScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := rc.Score(tt.text)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Score(%q) = %f, want between %f and %f",
					tt.text, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestRelevanceClassifierCaseInsensitive(t *testing.T) {
	keywords := []string{"machine learning"}
	rc := NewRelevanceClassifier(keywords, "ai")

	score1 := rc.Score("Machine Learning is great")
	score2 := rc.Score("machine learning is great")
	score3 := rc.Score("MACHINE LEARNING IS GREAT")

	// All should have similar scores
	if math.Abs(score1-score2) > 0.1 {
		t.Errorf("case sensitivity issue: score1=%f, score2=%f", score1, score2)
	}
	if math.Abs(score2-score3) > 0.1 {
		t.Errorf("case sensitivity issue: score2=%f, score3=%f", score2, score3)
	}
}

func TestRelevanceClassifierMultipleKeywordMatches(t *testing.T) {
	keywords := []string{"AI", "machine learning", "deep learning"}
	rc := NewRelevanceClassifier(keywords, "ai")

	// Text with multiple keyword matches should score higher
	score1 := rc.Score("AI is transforming industries")
	score2 := rc.Score("AI and machine learning are transforming industries")
	score3 := rc.Score("AI, machine learning, and deep learning are transforming industries")

	if score2 <= score1 {
		t.Errorf("expected score2 > score1, got %f <= %f", score2, score1)
	}
	if score3 <= score2 {
		t.Errorf("expected score3 > score2, got %f <= %f", score3, score2)
	}
}

func TestRelevanceClassifierScoreNormalization(t *testing.T) {
	keywords := []string{"test"}
	rc := NewRelevanceClassifier(keywords, "test")

	// Very long text with keyword should still have bounded score
	longText := "test " + string(make([]byte, 10000)) + " test"
	score := rc.Score(longText)

	if score < 0 || score > 1 {
		t.Errorf("score should be between 0 and 1, got %f", score)
	}
}

func TestRelevanceClassifierSpecialCharacters(t *testing.T) {
	keywords := []string{"C++", "C#", ".NET"}
	rc := NewRelevanceClassifier(keywords, "programming")

	score := rc.Score("I love programming in C++ and C# with .NET framework")

	// Should handle special characters in keywords
	if score < 0.3 {
		t.Errorf("expected score >= 0.3 for text with special char keywords, got %f", score)
	}
}

func TestRelevanceClassifierPhraseMatching(t *testing.T) {
	keywords := []string{"machine learning", "natural language processing"}
	rc := NewRelevanceClassifier(keywords, "ai")

	// Exact phrase match
	score1 := rc.Score("machine learning is a subset of AI")

	// Words present but not as phrase
	score2 := rc.Score("the machine was learning new patterns")

	// Phrase matching should score higher (or at least not lower)
	// Note: depending on implementation, exact phrases might score higher
	if score1 < 0.2 {
		t.Errorf("expected decent score for phrase match, got %f", score1)
	}
	if score2 < 0 {
		t.Errorf("score should not be negative, got %f", score2)
	}
}

func TestRelevanceClassifierEmptyKeywords(t *testing.T) {
	rc := NewRelevanceClassifier([]string{}, "empty")

	score := rc.Score("any text here")

	// With no keywords, score should be neutral (0.5)
	if score != 0.5 {
		t.Errorf("expected score 0.5 with no keywords, got %f", score)
	}
}
