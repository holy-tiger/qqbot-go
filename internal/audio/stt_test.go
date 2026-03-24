package audio

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewSTTProvider(t *testing.T) {
	cfg := STTConfig{
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		Model:   "whisper-1",
	}
	p := NewSTTProvider(cfg)
	if p == nil {
		t.Fatal("NewSTTProvider returned nil")
	}
	if p.config.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL mismatch: got %q, want %q", p.config.BaseURL, cfg.BaseURL)
	}
	if p.config.APIKey != cfg.APIKey {
		t.Errorf("APIKey mismatch: got %q, want %q", p.config.APIKey, cfg.APIKey)
	}
	if p.config.Model != cfg.Model {
		t.Errorf("Model mismatch: got %q, want %q", p.config.Model, cfg.Model)
	}
}

func TestSTTProvider_Transcribe(t *testing.T) {
	// Create a mock server that returns a transcription response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify endpoint
		if !strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			t.Errorf("expected /audio/transcriptions, got %s", r.URL.Path)
		}

		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("expected 'Bearer test-api-key', got %q", auth)
		}

		// Verify multipart form contains file and model
		reader, err := r.MultipartReader()
		if err != nil {
			t.Errorf("failed to read multipart: %v", err)
			return
		}

		var foundFile, foundModel bool
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("error reading part: %v", err)
				return
			}

			name := part.FormName()
			if name == "file" {
				foundFile = true
			}
			if name == "model" {
				data, _ := io.ReadAll(part)
				if string(data) != "whisper-1" {
					t.Errorf("model = %q, want %q", string(data), "whisper-1")
				}
				foundModel = true
			}
		}

		if !foundFile {
			t.Error("expected file field in multipart form")
		}
		if !foundModel {
			t.Error("expected model field in multipart form")
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": "Hello, world!"})
	}))
	defer server.Close()

	cfg := STTConfig{
		BaseURL: strings.TrimSuffix(server.URL, "/"),
		APIKey:  "test-api-key",
		Model:   "whisper-1",
	}
	provider := NewSTTProvider(cfg)

	// Create a temp audio file for testing
	tmpDir := t.TempDir()
	audioPath := tmpDir + "/test.wav"
	if err := writeFileWithCleanup(t, audioPath, []byte("fake wav data")); err != nil {
		t.Fatal(err)
	}

	result, err := provider.Transcribe(audioPath)
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	if result != "Hello, world!" {
		t.Errorf("Transcribe() = %q, want %q", result, "Hello, world!")
	}
}

func TestSTTProvider_Transcribe_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := STTConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "whisper-1",
	}
	provider := NewSTTProvider(cfg)

	tmpDir := t.TempDir()
	audioPath := tmpDir + "/test.wav"
	writeFileWithCleanup(t, audioPath, []byte("fake data"))

	_, err := provider.Transcribe(audioPath)
	if err == nil {
		t.Error("expected error for server 500 response")
	}
}

func TestSTTProvider_Transcribe_EmptyText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": ""})
	}))
	defer server.Close()

	cfg := STTConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "whisper-1",
	}
	provider := NewSTTProvider(cfg)

	tmpDir := t.TempDir()
	audioPath := tmpDir + "/test.wav"
	writeFileWithCleanup(t, audioPath, []byte("fake data"))

	result, err := provider.Transcribe(audioPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSTTProvider_Transcribe_NonExistentFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"text": "should not reach"})
	}))
	defer server.Close()

	cfg := STTConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "whisper-1",
	}
	provider := NewSTTProvider(cfg)

	_, err := provider.Transcribe("/nonexistent/file.wav")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestSTTProvider_Transcribe_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := STTConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "whisper-1",
	}
	provider := NewSTTProvider(cfg)

	tmpDir := t.TempDir()
	audioPath := tmpDir + "/test.wav"
	writeFileWithCleanup(t, audioPath, []byte("fake data"))

	_, err := provider.Transcribe(audioPath)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

// Helper to write a file with cleanup registered via t.Cleanup
func writeFileWithCleanup(t *testing.T, path string, data []byte) error {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	t.Cleanup(func() { os.Remove(path) })
	return nil
}

// _ imports to ensure multipart is used in tests
var _ = multipart.NewReader
