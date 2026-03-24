package audio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// STTConfig holds the configuration for the speech-to-text provider.
type STTConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// sttResponse represents the JSON response from the OpenAI-compatible STT API.
type sttResponse struct {
	Text string `json:"text"`
}

// STTProvider provides speech-to-text functionality via an OpenAI-compatible API.
type STTProvider struct {
	config STTConfig
	client *http.Client
}

// NewSTTProvider creates a new STTProvider with the given configuration.
func NewSTTProvider(config STTConfig) *STTProvider {
	return &STTProvider{
		config: config,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Transcribe sends an audio file to the STT API and returns the transcribed text.
func (s *STTProvider) Transcribe(audioPath string) (string, error) {
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", fmt.Errorf("audio file not found: %s", audioPath)
	}

	// Prepare multipart form body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	audioFile, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	if err := writer.WriteField("model", s.config.Model); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build request
	url := fmt.Sprintf("%s/audio/transcriptions", s.config.BaseURL)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.APIKey))

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("STT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("STT failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result sttResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode STT response: %w", err)
	}

	return result.Text, nil
}
