package audio

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// audioExtensions contains file extensions recognized as audio formats.
var audioExtensions = map[string]bool{
	".silk": true, ".slk": true, ".amr": true, ".slac": true,
	".wav": true, ".mp3": true, ".ogg": true, ".opus": true,
	".aac": true, ".flac": true, ".m4a": true, ".wma": true,
	".pcm": true,
}

// IsAudioFile checks whether the file path has a recognized audio extension.
func IsAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return audioExtensions[ext]
}

// ConvertAudio converts an audio file from srcPath to dstFormat at dstPath using ffmpeg.
func ConvertAudio(srcPath, dstPath, dstFormat string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", srcPath)
	}

	if dir := filepath.Dir(dstPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", srcPath,
		dstPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg convert failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// AudioFileToBase64 reads a file and returns its contents as a base64-encoded string.
func AudioFileToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// Base64ToAudioFile decodes a base64 string and writes it to the specified path.
func Base64ToAudioFile(base64Data, path string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return os.WriteFile(path, data, 0644)
}

// GetAudioDuration returns the duration of an audio file in seconds using ffprobe.
func GetAudioDuration(path string) (float64, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, fmt.Errorf("file not found: %s", path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var duration float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// FormatDuration formats a duration in seconds to "M:SS" format.
func FormatDuration(seconds float64) string {
	totalSeconds := int(math.Floor(seconds))
	minutes := totalSeconds / 60
	secs := totalSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
