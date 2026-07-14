package tts

import (
	"bytes"
	"os/exec"
)

type Engine interface {
	Synthesize(text, voice string) ([]byte, error)
	Name() string
}

// EdgeTTS uses Microsoft Edge TTS via edge-tts CLI
type EdgeTTS struct{}

func (e *EdgeTTS) Name() string { return "edge-tts" }

func (e *EdgeTTS) Synthesize(text, voice string) ([]byte, error) {
	if voice == "" {
		voice = "ko-KR-SunHiNeural"
	}

	cmd := exec.Command("edge-tts", "--voice", voice, "--text", text, "--write-media", "/dev/stdout")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Say uses macOS built-in say command
type MacSay struct{}

func (m *MacSay) Name() string { return "say" }

func (m *MacSay) Synthesize(text, voice string) ([]byte, error) {
	if voice == "" {
		voice = "Yuna" // Korean voice on macOS
	}

	// Create temp file for output
	cmd := exec.Command("say", "-v", voice, "-o", "/tmp/mimir-tts.aiff", text)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Convert to MP3
	convert := exec.Command("ffmpeg", "-y", "-i", "/tmp/mimir-tts.aiff", "-f", "mp3", "-")
	var out bytes.Buffer
	convert.Stdout = &out

	if err := convert.Run(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// NoTTS returns empty audio (for text-only delivery)
type NoTTS struct{}

func (n *NoTTS) Name() string { return "none" }

func (n *NoTTS) Synthesize(text, voice string) ([]byte, error) {
	return nil, nil
}

// GetEngine returns the best available TTS engine
func GetEngine(name string) Engine {
	switch name {
	case "edge-tts":
		return &EdgeTTS{}
	case "say":
		return &MacSay{}
	case "none", "":
		return &NoTTS{}
	default:
		// Try edge-tts first, fallback to none
		if _, err := exec.LookPath("edge-tts"); err == nil {
			return &EdgeTTS{}
		}
		return &NoTTS{}
	}
}
