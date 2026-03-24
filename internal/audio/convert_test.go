package audio

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0:00"},
		{5, "0:05"},
		{59, "0:59"},
		{60, "1:00"},
		{61, "1:01"},
		{90, "1:30"},
		{359, "5:59"},
		{360, "6:00"},
		{3661, "61:01"},
		{1.5, "0:01"},
		{59.9, "0:59"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestIsAudioFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.silk", true},
		{"test.slk", true},
		{"test.amr", true},
		{"test.wav", true},
		{"test.mp3", true},
		{"test.ogg", true},
		{"test.opus", true},
		{"test.aac", true},
		{"test.flac", true},
		{"test.m4a", true},
		{"test.wma", true},
		{"test.pcm", true},
		{"test.SILK", true},
		{"test.MP3", true},
		{"test.txt", false},
		{"test.jpg", false},
		{"test.png", false},
		{"test", false},
		{"test.wav.mp3", true}, // last extension wins
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsAudioFile(tt.path)
			if got != tt.want {
				t.Errorf("IsAudioFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestAudioFileToBase64(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.wav")
	content := []byte("RIFFfakeWAVEfmt data\x00\x01\x02")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := AudioFileToBase64(path)
	if err != nil {
		t.Fatalf("AudioFileToBase64 failed: %v", err)
	}

	// Verify the base64 decodes back to original content
	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	if string(decoded) != string(content) {
		t.Errorf("decoded content mismatch")
	}
}

func TestAudioFileToBase64_NonExistent(t *testing.T) {
	_, err := AudioFileToBase64("/nonexistent/file.wav")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestBase64ToAudioFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "output.wav")
	content := []byte("test audio data")
	b64 := base64.StdEncoding.EncodeToString(content)

	err := Base64ToAudioFile(b64, path)
	if err != nil {
		t.Fatalf("Base64ToAudioFile failed: %v", err)
	}

	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(read) != string(content) {
		t.Errorf("file content mismatch: got %q, want %q", read, content)
	}
}

func TestBase64ToAudioFile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "original.mp3")
	dstPath := filepath.Join(tmpDir, "copy.mp3")
	original := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC}

	if err := os.WriteFile(srcPath, original, 0644); err != nil {
		t.Fatal(err)
	}

	b64, err := AudioFileToBase64(srcPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := Base64ToAudioFile(b64, dstPath); err != nil {
		t.Fatal(err)
	}

	read, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(read) != string(original) {
		t.Errorf("round-trip mismatch: got %x, want %x", read, original)
	}
}

func TestConvertAudio_NonExistent(t *testing.T) {
	err := ConvertAudio("/nonexistent/input.wav", "/tmp/output.mp3", "mp3")
	if err == nil {
		t.Error("expected error for non-existent input")
	}
}

func TestGetAudioDuration_NonExistent(t *testing.T) {
	_, err := GetAudioDuration("/nonexistent/file.wav")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
