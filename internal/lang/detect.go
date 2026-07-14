// Package lang provides fast language detection using Unicode ranges and character patterns.
package lang

import (
	"strings"
	"unicode"
)

// Language codes
const (
	LangUnknown    = ""
	LangKorean     = "ko"
	LangJapanese   = "ja"
	LangChinese    = "zh"
	LangEnglish    = "en"
	LangRussian    = "ru"
	LangArabic     = "ar"
	LangThai       = "th"
	LangVietnamese = "vi"
	LangHindi      = "hi"
	LangHebrew     = "he"
	LangGreek      = "el"
	LangTurkish    = "tr"
	LangGerman     = "de"
	LangFrench     = "fr"
	LangSpanish    = "es"
	LangPortuguese = "pt"
	LangItalian    = "it"
	LangDutch      = "nl"
	LangPolish     = "pl"
	LangSwedish    = "sv"
)

// Result holds language detection results
type Result struct {
	Lang       string  // ISO 639-1 language code
	Confidence float64 // 0.0 to 1.0
	Script     string  // Detected script (Hangul, Han, Hiragana, etc.)
}

// Detect returns the most likely language for the given text.
// Returns empty string if detection fails or text is too short.
func Detect(text string) string {
	r := DetectWithConfidence(text)
	if r.Confidence < 0.3 {
		return LangUnknown
	}
	return r.Lang
}

// DetectWithConfidence returns detailed detection result with confidence score.
func DetectWithConfidence(text string) Result {
	if len(text) < 3 {
		return Result{Lang: LangUnknown, Confidence: 0}
	}

	// Count characters by script
	counts := countScripts(text)
	total := counts.total()
	if total == 0 {
		return Result{Lang: LangUnknown, Confidence: 0}
	}

	// Determine primary script
	script, scriptCount := counts.dominant()
	scriptRatio := float64(scriptCount) / float64(total)

	// Script-based detection
	switch script {
	case "Hangul":
		return Result{Lang: LangKorean, Confidence: scriptRatio, Script: script}

	case "Hiragana", "Katakana":
		return Result{Lang: LangJapanese, Confidence: scriptRatio, Script: script}

	case "Han":
		// Distinguish Chinese vs Japanese (Japanese uses kana alongside kanji)
		if counts.hiragana+counts.katakana > 0 {
			kanaRatio := float64(counts.hiragana+counts.katakana) / float64(total)
			if kanaRatio > 0.05 {
				return Result{Lang: LangJapanese, Confidence: scriptRatio, Script: "Han+Kana"}
			}
		}
		return Result{Lang: LangChinese, Confidence: scriptRatio, Script: script}

	case "Cyrillic":
		return Result{Lang: LangRussian, Confidence: scriptRatio, Script: script}

	case "Arabic":
		return Result{Lang: LangArabic, Confidence: scriptRatio, Script: script}

	case "Thai":
		return Result{Lang: LangThai, Confidence: scriptRatio, Script: script}

	case "Hebrew":
		return Result{Lang: LangHebrew, Confidence: scriptRatio, Script: script}

	case "Greek":
		return Result{Lang: LangGreek, Confidence: scriptRatio, Script: script}

	case "Devanagari":
		return Result{Lang: LangHindi, Confidence: scriptRatio, Script: script}

	case "Latin":
		// Use n-gram analysis for Latin script languages
		return detectLatinLanguage(text, scriptRatio)
	}

	return Result{Lang: LangUnknown, Confidence: 0}
}

// scriptCounts holds character counts by script
type scriptCounts struct {
	hangul     int
	hiragana   int
	katakana   int
	han        int
	cyrillic   int
	arabic     int
	thai       int
	hebrew     int
	greek      int
	devanagari int
	latin      int
	other      int
}

func (c *scriptCounts) total() int {
	return c.hangul + c.hiragana + c.katakana + c.han + c.cyrillic +
		c.arabic + c.thai + c.hebrew + c.greek + c.devanagari + c.latin
}

func (c *scriptCounts) dominant() (string, int) {
	max := 0
	script := ""

	if c.hangul > max {
		max, script = c.hangul, "Hangul"
	}
	if c.hiragana > max {
		max, script = c.hiragana, "Hiragana"
	}
	if c.katakana > max {
		max, script = c.katakana, "Katakana"
	}
	if c.han > max {
		max, script = c.han, "Han"
	}
	if c.cyrillic > max {
		max, script = c.cyrillic, "Cyrillic"
	}
	if c.arabic > max {
		max, script = c.arabic, "Arabic"
	}
	if c.thai > max {
		max, script = c.thai, "Thai"
	}
	if c.hebrew > max {
		max, script = c.hebrew, "Hebrew"
	}
	if c.greek > max {
		max, script = c.greek, "Greek"
	}
	if c.devanagari > max {
		max, script = c.devanagari, "Devanagari"
	}
	if c.latin > max {
		max, script = c.latin, "Latin"
	}

	return script, max
}

// countScripts counts characters by Unicode script
func countScripts(text string) scriptCounts {
	var c scriptCounts

	for _, r := range text {
		switch {
		case isHangul(r):
			c.hangul++
		case isHiragana(r):
			c.hiragana++
		case isKatakana(r):
			c.katakana++
		case isHan(r):
			c.han++
		case unicode.Is(unicode.Cyrillic, r):
			c.cyrillic++
		case unicode.Is(unicode.Arabic, r):
			c.arabic++
		case unicode.Is(unicode.Thai, r):
			c.thai++
		case unicode.Is(unicode.Hebrew, r):
			c.hebrew++
		case unicode.Is(unicode.Greek, r):
			c.greek++
		case unicode.Is(unicode.Devanagari, r):
			c.devanagari++
		case isLatin(r):
			c.latin++
		case unicode.IsLetter(r):
			c.other++
		}
	}

	return c
}

// Unicode range checks - inlined for performance

func isHangul(r rune) bool {
	// Hangul Syllables: U+AC00-U+D7AF
	// Hangul Jamo: U+1100-U+11FF
	// Hangul Compatibility Jamo: U+3130-U+318F
	// Hangul Jamo Extended-A: U+A960-U+A97F
	// Hangul Jamo Extended-B: U+D7B0-U+D7FF
	return (r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0x1100 && r <= 0x11FF) ||
		(r >= 0x3130 && r <= 0x318F) ||
		(r >= 0xA960 && r <= 0xA97F) ||
		(r >= 0xD7B0 && r <= 0xD7FF)
}

func isHiragana(r rune) bool {
	// Hiragana: U+3040-U+309F
	return r >= 0x3040 && r <= 0x309F
}

func isKatakana(r rune) bool {
	// Katakana: U+30A0-U+30FF
	// Katakana Phonetic Extensions: U+31F0-U+31FF
	return (r >= 0x30A0 && r <= 0x30FF) || (r >= 0x31F0 && r <= 0x31FF)
}

func isHan(r rune) bool {
	// CJK Unified Ideographs: U+4E00-U+9FFF
	// CJK Extension A: U+3400-U+4DBF
	// CJK Extension B-F: U+20000-U+2EBEF
	// CJK Compatibility Ideographs: U+F900-U+FAFF
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2EBEF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

func isLatin(r rune) bool {
	return unicode.Is(unicode.Latin, r)
}

// detectLatinLanguage uses character frequency, common words, and n-gram analysis
// to distinguish between Latin-script languages.
func detectLatinLanguage(text string, baseConfidence float64) Result {
	lower := strings.ToLower(text)

	// Vietnamese has distinct diacritics
	if hasVietnameseChars(lower) {
		return Result{Lang: LangVietnamese, Confidence: baseConfidence * 0.9, Script: "Latin"}
	}

	// Turkish has distinct characters
	if hasTurkishChars(lower) {
		return Result{Lang: LangTurkish, Confidence: baseConfidence * 0.8, Script: "Latin"}
	}

	// Polish has distinct diacritics
	if hasPolishChars(lower) {
		return Result{Lang: LangPolish, Confidence: baseConfidence * 0.8, Script: "Latin"}
	}

	// Swedish has distinct characters
	if hasSwedishChars(lower) {
		return Result{Lang: LangSwedish, Confidence: baseConfidence * 0.8, Script: "Latin"}
	}

	// Use common word detection first (more reliable than trigrams)
	wordScores := scoreCommonWords(lower)

	// Then use trigram analysis
	trigrams := extractTrigrams(lower)
	trigramScores := scoreTrigrams(trigrams)

	// Combine scores (words weighted more heavily)
	combinedScores := make(map[string]float64)
	for lang := range trigramScores {
		combinedScores[lang] = wordScores[lang]*3 + trigramScores[lang]
	}

	// Find best match
	bestLang := LangEnglish
	bestScore := combinedScores[LangEnglish]

	for lang, score := range combinedScores {
		if score > bestScore {
			bestScore = score
			bestLang = lang
		}
	}

	// Confidence based on score magnitude and distinctiveness
	confidence := baseConfidence * 0.7
	if bestScore > 10 {
		confidence = baseConfidence * 0.85
	}

	return Result{Lang: bestLang, Confidence: confidence, Script: "Latin"}
}

// scoreCommonWords counts occurrences of common words by language
func scoreCommonWords(text string) map[string]float64 {
	scores := make(map[string]float64)

	// Tokenize by whitespace and punctuation
	words := tokenize(text)

	// Common words by language (function words are most distinctive)
	wordLists := map[string][]string{
		LangEnglish: {"the", "a", "an", "is", "are", "was", "were", "be", "been",
			"have", "has", "had", "do", "does", "did", "will", "would", "could",
			"should", "may", "might", "must", "of", "to", "in", "for", "on",
			"with", "at", "by", "from", "this", "that", "it", "not", "but", "or",
			"and", "if", "as", "what", "which", "who", "how", "when", "where"},
		LangGerman: {"der", "die", "das", "den", "dem", "des", "ein", "eine",
			"einer", "einem", "einen", "und", "ist", "sind", "war", "waren",
			"sein", "haben", "hat", "hatte", "werden", "wird", "wurde", "nicht",
			"auf", "mit", "auch", "als", "noch", "nach", "bei", "nur", "wenn",
			"oder", "aber", "wie", "ich", "sie", "wir", "ihr", "kann", "muss"},
		LangFrench: {"le", "la", "les", "un", "une", "des", "de", "du", "au",
			"aux", "est", "sont", "etait", "avoir", "etre", "fait", "faire",
			"avec", "pour", "dans", "sur", "par", "pas", "plus", "tout", "mais",
			"ou", "et", "que", "qui", "ce", "cette", "ces", "nous", "vous", "ils",
			"elle", "lui", "leur", "mon", "ton", "son", "notre", "votre"},
		LangSpanish: {"el", "la", "los", "las", "un", "una", "unos", "unas",
			"de", "del", "al", "es", "son", "esta", "estan", "ser", "estar",
			"tiene", "tienen", "con", "para", "por", "en", "pero", "que", "como",
			"mas", "muy", "su", "sus", "este", "esta", "estos", "estas", "yo",
			"tu", "nosotros", "ellos", "donde", "cuando", "porque", "si", "no"},
		LangPortuguese: {"o", "a", "os", "as", "um", "uma", "uns", "umas",
			"de", "do", "da", "dos", "das", "no", "na", "nos", "nas", "ao",
			"e", "que", "em", "para", "com", "por", "como", "mais", "mas",
			"seu", "sua", "seus", "suas", "este", "esta", "esse", "essa",
			"eu", "ele", "ela", "nos", "eles", "voce", "quando", "se", "nao"},
		LangItalian: {"il", "lo", "la", "i", "gli", "le", "un", "uno", "una",
			"di", "del", "della", "dei", "delle", "al", "alla", "e", "che",
			"non", "con", "per", "come", "anche", "sono", "essere", "stato",
			"ha", "hanno", "questo", "questa", "quello", "quella", "io", "tu",
			"lui", "lei", "noi", "voi", "loro", "mio", "tuo", "suo", "nostro"},
		LangDutch: {"de", "het", "een", "van", "en", "in", "is", "dat", "op",
			"te", "zijn", "voor", "met", "als", "aan", "om", "maar", "die",
			"niet", "ook", "er", "bij", "nog", "wel", "naar", "dan", "wat",
			"kan", "meer", "al", "zou", "ze", "hij", "ik", "wij", "hun", "deze"},
	}

	wordSet := make(map[string]bool)
	for _, w := range words {
		wordSet[w] = true
	}

	for lang, commonWords := range wordLists {
		for _, word := range commonWords {
			if wordSet[word] {
				scores[lang]++
			}
		}
	}

	return scores
}

// tokenize splits text into lowercase words
func tokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// hasVietnameseChars checks for Vietnamese-specific diacritics
func hasVietnameseChars(text string) bool {
	// Vietnamese tones: ă, â, đ, ê, ô, ơ, ư and their tone marks
	vietChars := "ăâđêôơưảạấầẩẫậắằẳẵặảẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữự"
	count := 0
	for _, r := range text {
		if strings.ContainsRune(vietChars, r) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// hasTurkishChars checks for Turkish-specific characters
func hasTurkishChars(text string) bool {
	// ğ, ı (dotless i), ş, ç (also in other langs but common combination)
	turkishChars := "ğışçöüİ"
	count := 0
	for _, r := range text {
		if strings.ContainsRune(turkishChars, r) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// hasPolishChars checks for Polish-specific characters
func hasPolishChars(text string) bool {
	// ą, ć, ę, ł, ń, ó, ś, ź, ż
	polishChars := "ąćęłńśźż"
	count := 0
	for _, r := range text {
		if strings.ContainsRune(polishChars, r) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// hasSwedishChars checks for Swedish-specific characters
func hasSwedishChars(text string) bool {
	// å, ä, ö
	swedishChars := "åäö"
	count := 0
	for _, r := range text {
		if strings.ContainsRune(swedishChars, r) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

// extractTrigrams extracts character trigrams from text
func extractTrigrams(text string) map[string]int {
	trigrams := make(map[string]int)
	runes := []rune(text)

	for i := 0; i < len(runes)-2; i++ {
		// Skip non-letter sequences
		if !unicode.IsLetter(runes[i]) || !unicode.IsLetter(runes[i+1]) || !unicode.IsLetter(runes[i+2]) {
			continue
		}
		trigram := string(runes[i : i+3])
		trigrams[trigram]++
	}

	return trigrams
}

// scoreTrigrams scores trigrams against language profiles
func scoreTrigrams(trigrams map[string]int) map[string]float64 {
	scores := make(map[string]float64)

	// English-specific trigrams (avoid shared ones with Romance languages)
	enTrigrams := map[string]float64{
		"the": 15, "ing": 10, "tion": 8, "ght": 8, "ould": 7,
		"tha": 7, "nth": 6, "thi": 6, "ith": 6, "fth": 5,
		"his": 5, "whi": 5, "whe": 5, "hin": 5, "hat": 5,
		"ome": 4, "ble": 4, "eve": 4, "nce": 4, "ake": 4,
	}

	// German-specific trigrams
	deTrigrams := map[string]float64{
		"sch": 15, "ich": 12, "ein": 10, "und": 10, "der": 9,
		"die": 9, "ung": 8, "cht": 8, "den": 7, "ber": 7,
		"eit": 7, "auf": 6, "aus": 6, "mit": 6, "das": 6,
		"ist": 6, "gen": 5, "nen": 5, "ern": 5, "ach": 5,
	}

	// French-specific trigrams
	frTrigrams := map[string]float64{
		"ait": 12, "eux": 10, "ais": 9, "ous": 9, "oir": 8,
		"eau": 8, "lle": 7, "que": 7, "tre": 7, "our": 6,
		"eur": 6, "aux": 6, "ent": 6, "pas": 6, "ien": 5,
		"eme": 5, "omm": 5, "est": 5, "sse": 5, "rai": 5,
	}

	// Spanish-specific trigrams
	esTrigrams := map[string]float64{
		"ado": 12, "ión": 10, "los": 9, "las": 9, "nte": 8,
		"aci": 8, "que": 7, "ndo": 7, "sta": 6, "ent": 6,
		"tra": 6, "una": 5, "ara": 5, "era": 5, "ido": 5,
	}

	// Portuguese-specific trigrams
	ptTrigrams := map[string]float64{
		"ção": 15, "ões": 12, "ndo": 8, "ado": 8, "nte": 7,
		"que": 7, "uma": 6, "ara": 6, "dos": 6, "das": 6,
		"por": 5, "com": 5, "ser": 5, "ais": 5, "ica": 5,
	}

	// Italian-specific trigrams
	itTrigrams := map[string]float64{
		"zio": 15, "gli": 12, "tti": 10, "ell": 9, "lla": 8,
		"ato": 8, "che": 7, "one": 7, "ita": 6, "ere": 6,
		"nti": 6, "ess": 5, "per": 5, "ono": 5, "ano": 5,
	}

	// Dutch-specific trigrams
	nlTrigrams := map[string]float64{
		"aar": 12, "oor": 10, "een": 9, "het": 9, "van": 8,
		"ijn": 8, "erd": 7, "aan": 7, "wor": 6, "cht": 6,
		"eer": 6, "ond": 5, "gel": 5, "ste": 5, "oel": 5,
	}

	// Score each language
	langProfiles := map[string]map[string]float64{
		LangEnglish:    enTrigrams,
		LangGerman:     deTrigrams,
		LangFrench:     frTrigrams,
		LangSpanish:    esTrigrams,
		LangPortuguese: ptTrigrams,
		LangItalian:    itTrigrams,
		LangDutch:      nlTrigrams,
	}

	for lang, profile := range langProfiles {
		var score float64
		for trigram, count := range trigrams {
			if weight, ok := profile[trigram]; ok {
				score += weight * float64(count)
			}
		}
		scores[lang] = score
	}

	return scores
}

// DetectMultiple returns multiple possible languages sorted by confidence.
// Useful when text might be mixed-language.
func DetectMultiple(text string, maxResults int) []Result {
	if len(text) < 3 {
		return nil
	}

	counts := countScripts(text)
	total := counts.total()
	if total == 0 {
		return nil
	}

	var results []Result

	// Add results for each significant script
	threshold := float64(total) * 0.05 // At least 5% of characters

	if float64(counts.hangul) >= threshold {
		results = append(results, Result{
			Lang:       LangKorean,
			Confidence: float64(counts.hangul) / float64(total),
			Script:     "Hangul",
		})
	}

	if float64(counts.hiragana+counts.katakana) >= threshold {
		results = append(results, Result{
			Lang:       LangJapanese,
			Confidence: float64(counts.hiragana+counts.katakana) / float64(total),
			Script:     "Kana",
		})
	}

	if float64(counts.han) >= threshold {
		// If kana present, it's Japanese; otherwise Chinese
		if counts.hiragana+counts.katakana > 0 {
			// Already added Japanese above
		} else {
			results = append(results, Result{
				Lang:       LangChinese,
				Confidence: float64(counts.han) / float64(total),
				Script:     "Han",
			})
		}
	}

	if float64(counts.cyrillic) >= threshold {
		results = append(results, Result{
			Lang:       LangRussian,
			Confidence: float64(counts.cyrillic) / float64(total),
			Script:     "Cyrillic",
		})
	}

	if float64(counts.latin) >= threshold {
		r := detectLatinLanguage(text, float64(counts.latin)/float64(total))
		results = append(results, r)
	}

	// Sort by confidence
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Confidence > results[i].Confidence {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// IsLanguage checks if text is primarily in the specified language.
func IsLanguage(text, lang string) bool {
	detected := Detect(text)
	return detected == lang
}

// GetScript returns the dominant script in the text.
func GetScript(text string) string {
	counts := countScripts(text)
	script, _ := counts.dominant()
	return script
}
