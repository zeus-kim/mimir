package briefing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zeus-kim/mimir/internal/tts"
)

// AudioBriefing represents a generated audio briefing
type AudioBriefing struct {
	Text      string    // Original text content
	AudioData []byte    // MP3 audio bytes
	Voice     string    // TTS voice used
	Duration  float64   // Estimated duration in seconds
	CreatedAt time.Time // Generation timestamp
	FilePath  string    // Path if saved to file
}

// AudioGenerator generates audio briefings from text
type AudioGenerator struct {
	Engine    tts.Engine
	Voice     string  // Default voice
	Rate      string  // Speech rate adjustment (e.g., "+10%", "-5%")
	OutputDir string  // Directory for saved files
}

// DefaultVoices for different languages
var DefaultVoices = map[string]string{
	"ko": "ko-KR-SunHiNeural",
	"en": "en-US-JennyNeural",
	"ja": "ja-JP-NanamiNeural",
	"zh": "zh-CN-XiaoxiaoNeural",
}

// NewAudioGenerator creates a new audio generator
func NewAudioGenerator(engine tts.Engine, outputDir string) *AudioGenerator {
	if outputDir == "" {
		home, _ := os.UserHomeDir()
		outputDir = filepath.Join(home, ".mine")
	}
	return &AudioGenerator{
		Engine:    engine,
		Voice:     DefaultVoices["ko"],
		Rate:      "+10%",
		OutputDir: outputDir,
	}
}

// Generate creates an audio briefing from text
func (g *AudioGenerator) Generate(text string) (*AudioBriefing, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text input")
	}

	// Synthesize audio
	audioData, err := g.synthesize(text)
	if err != nil {
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// Estimate duration (rough: ~150 chars per minute for Korean)
	duration := float64(len([]rune(text))) / 150.0 * 60.0

	return &AudioBriefing{
		Text:      text,
		AudioData: audioData,
		Voice:     g.Voice,
		Duration:  duration,
		CreatedAt: time.Now(),
	}, nil
}

// GenerateAndSave creates audio and saves to file
func (g *AudioGenerator) GenerateAndSave(text, filename string) (*AudioBriefing, error) {
	briefing, err := g.Generate(text)
	if err != nil {
		return nil, err
	}

	// Use default filename if not provided
	if filename == "" {
		filename = fmt.Sprintf("briefing_%s.mp3", time.Now().Format("20060102_150405"))
	}

	filePath := filepath.Join(g.OutputDir, filename)
	if err := g.SaveToFile(briefing, filePath); err != nil {
		return nil, err
	}

	briefing.FilePath = filePath
	return briefing, nil
}

// SaveToFile saves audio briefing to a file
func (g *AudioGenerator) SaveToFile(briefing *AudioBriefing, path string) error {
	if briefing.AudioData == nil || len(briefing.AudioData) == 0 {
		return fmt.Errorf("no audio data to save")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, briefing.AudioData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	briefing.FilePath = path
	return nil
}

// synthesize calls the TTS engine with rate adjustment
func (g *AudioGenerator) synthesize(text string) ([]byte, error) {
	// For EdgeTTS, we can pass rate via voice parameter or use edge-tts CLI directly
	// The tts.Engine interface is simple, so we handle rate adjustment here if needed
	if g.Engine.Name() == "edge-tts" && g.Rate != "" {
		return g.synthesizeEdgeTTSWithRate(text)
	}
	return g.Engine.Synthesize(text, g.Voice)
}

// synthesizeEdgeTTSWithRate uses edge-tts CLI with rate control
func (g *AudioGenerator) synthesizeEdgeTTSWithRate(text string) ([]byte, error) {
	args := []string{
		"--voice", g.Voice,
		"--rate", g.Rate,
		"--text", text,
		"--write-media", "-",
	}

	cmd := exec.Command("edge-tts", args...)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to engine without rate
		return g.Engine.Synthesize(text, g.Voice)
	}
	return output, nil
}

// SetVoice sets the voice for synthesis
func (g *AudioGenerator) SetVoice(voice string) {
	g.Voice = voice
}

// SetVoiceByLanguage sets voice based on language code
func (g *AudioGenerator) SetVoiceByLanguage(lang string) {
	if voice, ok := DefaultVoices[lang]; ok {
		g.Voice = voice
	}
}

// SetRate sets the speech rate adjustment
func (g *AudioGenerator) SetRate(rate string) {
	g.Rate = rate
}

// GenerateWithMacSay generates audio using macOS say command (fallback)
func GenerateWithMacSay(text, voice, outputPath string) error {
	if voice == "" {
		voice = "Yuna" // Korean voice on macOS
	}

	// Generate AIFF first
	aiffPath := outputPath + ".aiff"
	sayCmd := exec.Command("say", "-v", voice, "-o", aiffPath, text)
	if err := sayCmd.Run(); err != nil {
		return fmt.Errorf("say command failed: %w", err)
	}
	defer os.Remove(aiffPath)

	// Convert to MP3 using ffmpeg
	ffmpegCmd := exec.Command("ffmpeg", "-y", "-i", aiffPath, "-codec:a", "libmp3lame", "-qscale:a", "2", outputPath)
	if err := ffmpegCmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	return nil
}

// EstimateDuration estimates audio duration from text
// Returns duration in seconds based on average speaking rate
func EstimateDuration(text string, wordsPerMinute int) float64 {
	if wordsPerMinute <= 0 {
		wordsPerMinute = 150 // Default for Korean
	}
	// Count characters for CJK, words for others
	charCount := len([]rune(text))
	return float64(charCount) / float64(wordsPerMinute) * 60.0
}

// AvailableVoices lists commonly used Edge TTS voices
var AvailableVoices = []struct {
	Code   string
	Name   string
	Gender string
	Lang   string
}{
	{"ko-KR-SunHiNeural", "Sun-Hi", "Female", "Korean"},
	{"ko-KR-InJoonNeural", "In-Joon", "Male", "Korean"},
	{"en-US-JennyNeural", "Jenny", "Female", "English"},
	{"en-US-GuyNeural", "Guy", "Male", "English"},
	{"ja-JP-NanamiNeural", "Nanami", "Female", "Japanese"},
	{"zh-CN-XiaoxiaoNeural", "Xiaoxiao", "Female", "Chinese"},
}
