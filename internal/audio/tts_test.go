package audio

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEdgeTTSAvailable(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}
}

func TestTTSProvider_Synthesize(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}

	provider := NewTTSProvider(TTSConfig{
		Voice: "zh-CN-XiaoxiaoNeural",
	})

	mp3Data, err := provider.Synthesize("你好，测试语音合成")
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}
	if len(mp3Data) == 0 {
		t.Fatal("expected non-empty MP3 data")
	}
	// MP3 files start with ID3 tag or sync word
	if mp3Data[0] != 0xFF && !strings.HasPrefix(string(mp3Data[:3]), "ID3") {
		t.Errorf("data doesn't look like MP3, first bytes: %x", mp3Data[:4])
	}
}

func TestTTSProvider_SynthesizeToSilkBase64(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}

	provider := NewTTSProvider(TTSConfig{
		Voice: "zh-CN-XiaoxiaoNeural",
	})

	mp3Base64, err := provider.SynthesizeToSilkBase64("测试语音 Base64 编码")
	if err != nil {
		t.Fatalf("SynthesizeToSilkBase64 failed: %v", err)
	}
	if mp3Base64 == "" {
		t.Fatal("expected non-empty Base64")
	}

	// Verify it's valid Base64
	decoded, err := base64.StdEncoding.DecodeString(mp3Base64)
	if err != nil {
		t.Fatalf("invalid Base64: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("decoded data is empty")
	}

	// Verify it's MP3 (ID3 tag or sync word)
	if decoded[0] != 0xFF && !strings.HasPrefix(string(decoded[:3]), "ID3") {
		t.Errorf("data doesn't look like MP3, first bytes: %x", decoded[:min(4, len(decoded))])
	}
}

func TestTTSProvider_EmptyText(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}

	provider := NewTTSProvider(TTSConfig{})

	_, err := provider.Synthesize("")
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestEdgeTTSAvailable_Check(t *testing.T) {
	// Just verify the check doesn't panic
	_ = IsEdgeTTSAvailable()
}

func TestTTSProvider_CustomVoice(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}

	provider := NewTTSProvider(TTSConfig{
		Voice: "en-US-JennyNeural",
	})

	mp3Data, err := provider.Synthesize("Hello, this is a test")
	if err != nil {
		t.Fatalf("Synthesize with en-US voice failed: %v", err)
	}
	if len(mp3Data) == 0 {
		t.Fatal("expected non-empty MP3 data")
	}
}

func TestTTSProvider_SynthesizeToFile(t *testing.T) {
	if !IsEdgeTTSAvailable() {
		t.Skip("edge-tts not installed")
	}

	dir := t.TempDir()
	provider := NewTTSProvider(TTSConfig{})

	outPath := filepath.Join(dir, "output.mp3")
	err := provider.SynthesizeToFile("文件输出测试", outPath)
	if err != nil {
		t.Fatalf("SynthesizeToFile failed: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
