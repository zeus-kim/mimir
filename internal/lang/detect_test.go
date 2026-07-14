package lang

import "testing"

func TestDetect(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		// Korean
		{"안녕하세요 반갑습니다", LangKorean},
		{"오늘 날씨가 좋습니다", LangKorean},
		{"한국어 테스트입니다", LangKorean},

		// Japanese
		{"こんにちは世界", LangJapanese},
		{"今日は天気がいいです", LangJapanese},
		{"ありがとうございます", LangJapanese},

		// Chinese
		{"你好世界", LangChinese},
		{"今天天气很好", LangChinese},
		{"中文测试文本", LangChinese},

		// English
		{"Hello world this is a test", LangEnglish},
		{"The quick brown fox jumps over the lazy dog", LangEnglish},

		// German
		{"Guten Tag, wie geht es Ihnen", LangGerman},
		{"Das ist ein deutscher Text", LangGerman},

		// French
		{"Bonjour comment allez vous", LangFrench},
		{"C'est une phrase en français", LangFrench},

		// Russian
		{"Привет мир как дела", LangRussian},
		{"Это тестовый текст на русском", LangRussian},

		// Vietnamese
		{"Xin chào thế giới", LangVietnamese},
		{"Đây là văn bản tiếng Việt", LangVietnamese},

		// Empty/short
		{"", LangUnknown},
		{"ab", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(20, len(tt.text))], func(t *testing.T) {
			got := Detect(tt.text)
			if got != tt.want {
				t.Errorf("Detect(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectWithConfidence(t *testing.T) {
	// Korean text should have high confidence
	r := DetectWithConfidence("안녕하세요 반갑습니다")
	if r.Lang != LangKorean {
		t.Errorf("got lang %q, want %q", r.Lang, LangKorean)
	}
	if r.Confidence < 0.5 {
		t.Errorf("got confidence %v, want >= 0.5", r.Confidence)
	}
	if r.Script != "Hangul" {
		t.Errorf("got script %q, want Hangul", r.Script)
	}
}

func TestDetectMultiple(t *testing.T) {
	// Mixed Korean and English - needs more substantial English text
	text := "Hello world this is a test 안녕하세요 반갑습니다"
	results := DetectMultiple(text, 3)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Should detect both Korean and Latin-based language
	hasKorean := false
	hasLatin := false
	for _, r := range results {
		if r.Lang == LangKorean {
			hasKorean = true
		}
		if r.Script == "Latin" {
			hasLatin = true
		}
	}

	if !hasKorean {
		t.Error("expected Korean in results")
	}
	if !hasLatin {
		t.Error("expected Latin script language in results")
	}
}

func TestIsLanguage(t *testing.T) {
	if !IsLanguage("안녕하세요", LangKorean) {
		t.Error("expected Korean")
	}
	if IsLanguage("Hello world", LangKorean) {
		t.Error("expected not Korean")
	}
}

func TestGetScript(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"안녕하세요", "Hangul"},
		{"こんにちは", "Hiragana"},
		{"カタカナ", "Katakana"},
		{"中文", "Han"},
		{"Hello", "Latin"},
		{"Привет", "Cyrillic"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := GetScript(tt.text)
			if got != tt.want {
				t.Errorf("GetScript(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func BenchmarkDetect(b *testing.B) {
	texts := []string{
		"Hello world this is a test of the language detection system",
		"안녕하세요 이것은 한국어 언어 감지 테스트입니다",
		"これは日本語のテキストです言語検出のテスト",
		"这是一个中文文本语言检测测试",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, text := range texts {
			Detect(text)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
