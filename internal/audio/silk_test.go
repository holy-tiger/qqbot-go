package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFFmpegAvailable(t *testing.T) {
	// We don't know if ffmpeg is installed in the test environment,
	// just verify the function doesn't panic and returns a bool.
	result := IsFFmpegAvailable()
	t.Logf("IsFFmpegAvailable() = %v", result)
	if result != true && result != false {
		t.Fatalf("expected bool, got %T", result)
	}
}

func TestIsSilkFile(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    bool
	}{
		{
			name: "valid SILK v3 header",
			data: []byte("#!SILK_V3"),
			want: true,
		},
		{
			name: "SILK header with extra data",
			data: []byte("#!SILK_V3\x00\x01\x02\x03"),
			want: true,
		},
		{
			name: "empty file",
			data: []byte{},
			want: false,
		},
		{
			name: "too short",
			data: []byte("#!SILK"),
			want: false,
		},
		{
			name: "random data",
			data: []byte("\x00\x01\x02\x03\x04\x05\x06\x07"),
			want: false,
		},
		{
			name: "WAV header",
			data: []byte("RIFF\x00\x00\x00\x00WAVEfmt "),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.silk")
			if err := os.WriteFile(path, tt.data, 0644); err != nil {
				t.Fatal(err)
			}
			got := IsSilkFile(path)
			if got != tt.want {
				t.Errorf("IsSilkFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsSilkFile_NonExistent(t *testing.T) {
	got := IsSilkFile("/nonexistent/path/to/file.silk")
	if got {
		t.Error("IsSilkFile should return false for non-existent file")
	}
}

func TestIsSilkBytes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid SILK v3", []byte("#!SILK_V3"), true},
		{"SILK with AMR header", []byte("#!AMR\n#!SILK_V3"), true},
		{"empty", []byte{}, false},
		{"too short", []byte("#!SILK"), false},
		{"random", []byte("\xff\xfe\xfd\xfc"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSilkBytes(tt.data)
			if got != tt.want {
				t.Errorf("IsSilkBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSilkToWavBytes_EmptyInput(t *testing.T) {
	_, err := SilkToWavBytes(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestWavToSilkBytes_EmptyInput(t *testing.T) {
	_, err := WavToSilkBytes(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestDecodeSilkToWav_NonExistent(t *testing.T) {
	err := DecodeSilkToWav("/nonexistent/file.silk", "/tmp/output.wav")
	if err == nil {
		t.Error("expected error for non-existent input file")
	}
}

func TestEncodeWavToSilk_NonExistent(t *testing.T) {
	err := EncodeWavToSilk("/nonexistent/file.wav", "/tmp/output.silk")
	if err == nil {
		t.Error("expected error for non-existent input file")
	}
}

func TestPCMToWAV(t *testing.T) {
	// Create a small PCM buffer (1 second of silence at 24000 Hz, 16-bit mono)
	sampleRate := 24000
	pcmData := make([]byte, sampleRate*2) // 2 bytes per sample, 1 second

	wavData := PCMToWAV(pcmData, sampleRate, 1, 16)

	// Verify WAV header
	if len(wavData) < 44 {
		t.Fatalf("WAV data too short: %d bytes", len(wavData))
	}
	if string(wavData[0:4]) != "RIFF" {
		t.Errorf("expected RIFF header, got %q", string(wavData[0:4]))
	}
	if string(wavData[8:12]) != "WAVE" {
		t.Errorf("expected WAVE format, got %q", string(wavData[8:12]))
	}
	if string(wavData[36:40]) != "data" {
		t.Errorf("expected data chunk, got %q", string(wavData[36:40]))
	}

	// WAV header should be 44 bytes + pcm data
	expectedLen := 44 + len(pcmData)
	if len(wavData) != expectedLen {
		t.Errorf("WAV length = %d, want %d", len(wavData), expectedLen)
	}
}
