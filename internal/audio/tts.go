package audio

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// DefaultTTSVoice is the default Edge TTS voice.
const DefaultTTSVoice = "zh-CN-XiaoxiaoNeural"

// TTSConfig holds configuration for the TTS provider.
type TTSConfig struct {
	Voice string // Edge TTS voice name, e.g. "zh-CN-XiaoxiaoNeural"
}

// TTSProvider provides text-to-speech via edge-tts CLI.
type TTSProvider struct {
	voice string
}

// NewTTSProvider creates a new TTSProvider.
func NewTTSProvider(config TTSConfig) *TTSProvider {
	voice := config.Voice
	if voice == "" {
		voice = DefaultTTSVoice
	}
	return &TTSProvider{voice: voice}
}

// IsEdgeTTSAvailable checks if the edge-tts CLI is installed.
func IsEdgeTTSAvailable() bool {
	_, err := exec.LookPath("edge-tts")
	return err == nil
}

// Synthesize converts text to MP3 audio bytes using edge-tts.
func (p *TTSProvider) Synthesize(text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("tts: text must not be empty")
	}
	if !IsEdgeTTSAvailable() {
		return nil, fmt.Errorf("tts: edge-tts not installed (pip install edge-tts)")
	}

	dir, err := os.MkdirTemp("", "tts-*")
	if err != nil {
		return nil, fmt.Errorf("tts: create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	outPath := filepath.Join(dir, "output.mp3")
	// P2-10: use CommandContext with timeout, like silk.go/convert.go
	ttsCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ttsCtx, "edge-tts",
		"--voice", p.voice,
		"--text", text,
		"--write-media", outPath,
	)
	cmd.Stderr = nil // suppress edge-tts progress output

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tts: edge-tts failed: %w", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("tts: read output: %w", err)
	}
	return data, nil
}

// SynthesizeToFile converts text to an audio file using edge-tts.
func (p *TTSProvider) SynthesizeToFile(text, outPath string) error {
	if text == "" {
		return fmt.Errorf("tts: text must not be empty")
	}
	if !IsEdgeTTSAvailable() {
		return fmt.Errorf("tts: edge-tts not installed (pip install edge-tts)")
	}

	// P2-10: use CommandContext with timeout
	ttsCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ttsCtx, "edge-tts",
		"--voice", p.voice,
		"--text", text,
		"--write-media", outPath,
	)
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tts: edge-tts failed: %w", err)
	}
	return nil
}

// SynthesizeToSilkBase64 converts text to MP3 audio encoded as Base64.
// Edge TTS outputs MP3 directly, which QQ Bot API accepts for voice messages.
func (p *TTSProvider) SynthesizeToSilkBase64(text string) (string, error) {
	mp3Data, err := p.Synthesize(text)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(mp3Data), nil
}
