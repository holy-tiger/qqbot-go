package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	silkSampleRate = 24000
	silkChannels   = 1
	silkBitDepth   = 16
)

// IsFFmpegAvailable checks whether ffmpeg is installed and accessible.
func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// IsSilkBytes checks whether raw bytes contain the SILK v3 magic header.
// It also handles the AMR wrapper header ("#!AMR\n") that QQ may prepend.
func IsSilkBytes(data []byte) bool {
	if len(data) < 9 {
		return false
	}
	// Check for raw SILK header
	if string(data[:9]) == "#!SILK_V3" {
		return true
	}
	// Check for AMR-wrapped SILK: "#!AMR\n" followed by "#!SILK_V3"
	const amrHeader = "#!AMR\n"
	amrLen := len(amrHeader)
	if len(data) >= amrLen+9 && string(data[:amrLen]) == amrHeader {
		if string(data[amrLen:amrLen+9]) == "#!SILK_V3" {
			return true
		}
	}
	return false
}

// IsSilkFile checks whether a file at the given path is a SILK audio file.
func IsSilkFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return IsSilkBytes(data)
}

// DecodeSilkToWav converts a SILK file to WAV format using ffmpeg.
func DecodeSilkToWav(silkPath, wavPath string) error {
	if _, err := os.Stat(silkPath); os.IsNotExist(err) {
		return fmt.Errorf("silk input file not found: %s", silkPath)
	}

	// Ensure output directory exists
	if dir := filepath.Dir(wavPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", silkPath,
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", silkSampleRate),
		"-ac", fmt.Sprintf("%d", silkChannels),
		"-acodec", "pcm_s16le",
		"pipe:1",
	)

	pcmData, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg decode failed: %w", err)
	}

	wavData := PCMToWAV(pcmData, silkSampleRate, silkChannels, silkBitDepth)
	return os.WriteFile(wavPath, wavData, 0644)
}

// EncodeWavToSilk converts a WAV file to SILK format using ffmpeg.
func EncodeWavToSilk(wavPath, silkPath string) error {
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		return fmt.Errorf("wav input file not found: %s", wavPath)
	}

	// Ensure output directory exists
	if dir := filepath.Dir(silkPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", wavPath,
		"-ar", fmt.Sprintf("%d", silkSampleRate),
		"-ac", fmt.Sprintf("%d", silkChannels),
		"-c:a", "libopus",
		"-b:a", "24000",
		silkPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg encode failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// SilkToWavBytes converts SILK audio bytes to WAV bytes.
func SilkToWavBytes(silkData []byte) ([]byte, error) {
	if len(silkData) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}

	// Create temp files for ffmpeg
	silkFile, err := os.CreateTemp("", "silk-input-*.silk")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp silk file: %w", err)
	}
	defer os.Remove(silkFile.Name())

	wavFile, err := os.CreateTemp("", "silk-output-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp wav file: %w", err)
	}
	defer os.Remove(wavFile.Name())

	// Write silk data to temp file
	if _, err := silkFile.Write(silkData); err != nil {
		silkFile.Close()
		return nil, fmt.Errorf("failed to write silk data: %w", err)
	}
	silkFile.Close()
	wavFile.Close()

	if err := DecodeSilkToWav(silkFile.Name(), wavFile.Name()); err != nil {
		return nil, err
	}

	return os.ReadFile(wavFile.Name())
}

// WavToSilkBytes converts WAV audio bytes to SILK bytes.
func WavToSilkBytes(wavData []byte) ([]byte, error) {
	if len(wavData) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}

	// Create temp files for ffmpeg
	wavFile, err := os.CreateTemp("", "wav-input-*.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp wav file: %w", err)
	}
	defer os.Remove(wavFile.Name())

	silkFile, err := os.CreateTemp("", "silk-output-*.silk")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp silk file: %w", err)
	}
	defer os.Remove(silkFile.Name())

	// Write wav data to temp file
	if _, err := wavFile.Write(wavData); err != nil {
		wavFile.Close()
		return nil, fmt.Errorf("failed to write wav data: %w", err)
	}
	wavFile.Close()
	silkFile.Close()

	if err := EncodeWavToSilk(wavFile.Name(), silkFile.Name()); err != nil {
		return nil, err
	}

	return os.ReadFile(silkFile.Name())
}

// PCMToWAV wraps raw PCM data in a WAV file header.
func PCMToWAV(pcmData []byte, sampleRate, channels, bitsPerSample int) []byte {
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	dataSize := len(pcmData)
	headerSize := 44
	fileSize := headerSize + dataSize

	buf := make([]byte, fileSize)

	// RIFF header
	copy(buf[0:4], "RIFF")
	le32(buf[4:8], uint32(fileSize-8))
	copy(buf[8:12], "WAVE")

	// fmt sub-chunk
	copy(buf[12:16], "fmt ")
	le32(buf[16:20], 16)           // sub-chunk size
	le16(buf[20:22], 1)            // PCM format
	le16(buf[22:24], uint16(channels))
	le32(buf[24:28], uint32(sampleRate))
	le32(buf[28:32], uint32(byteRate))
	le16(buf[32:34], uint16(blockAlign))
	le16(buf[34:36], uint16(bitsPerSample))

	// data sub-chunk
	copy(buf[36:40], "data")
	le32(buf[40:44], uint32(dataSize))
	copy(buf[headerSize:], pcmData)

	return buf
}

func le16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func le32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
