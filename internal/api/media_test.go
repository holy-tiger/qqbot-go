package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/types"
)

func newMediaTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "QQBot test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/users/user1/files"):
			// C2C media upload
			resp := map[string]interface{}{
				"file_uuid": "uuid-img-001",
				"file_info": "file_info_img_001",
				"ttl":       3600,
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 3 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-voice-001",
					"file_info": "file_info_voice_001",
					"ttl":       3600,
				}
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 4 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-file-001",
					"file_info": "file_info_file_001",
					"ttl":       3600,
				}
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 2 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-video-001",
					"file_info": "file_info_video_001",
					"ttl":       3600,
				}
			}
			json.NewEncoder(w).Encode(resp)

		case strings.HasPrefix(r.URL.Path, "/v2/groups/group1/files"):
			// Group media upload
			resp := map[string]interface{}{
				"file_uuid": "uuid-grp-img-001",
				"file_info": "file_info_grp_img_001",
				"ttl":       3600,
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 3 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-grp-voice-001",
					"file_info": "file_info_grp_voice_001",
					"ttl":       3600,
				}
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 4 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-grp-file-001",
					"file_info": "file_info_grp_file_001",
					"ttl":       3600,
				}
			}
			if ft, ok := body["file_type"].(float64); ok && ft == 2 {
				resp = map[string]interface{}{
					"file_uuid": "uuid-grp-video-001",
					"file_info": "file_info_grp_video_001",
					"ttl":       3600,
				}
			}
			json.NewEncoder(w).Encode(resp)

		case strings.HasPrefix(r.URL.Path, "/v2/users/user1/messages"):
			resp := map[string]interface{}{
				"id":        "media-msg-001",
				"timestamp": "2024-01-01T00:00:00Z",
				"ext_info":  map[string]string{"ref_idx": "ref-media-001"},
			}
			json.NewEncoder(w).Encode(resp)

		case strings.HasPrefix(r.URL.Path, "/v2/groups/group1/messages"):
			resp := map[string]interface{}{
				"id":        "grp-media-msg-001",
				"timestamp": "2024-01-01T00:00:00Z",
				"ext_info":  map[string]string{"ref_idx": "ref-grp-media-001"},
			}
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newMediaTestClient(server *httptest.Server, opts ...ClientOption) *APIClient {
	tc := NewTokenCache()
	client := NewAPIClient(opts...)
	client.tokenCache = tc
	client.apiBase = server.URL
	client.appID = "app1"
	client.clientSecret = "secret1"
	client.uploadCache = NewUploadCache(100)
	return client
}

func prefillTokenForMedia(t *testing.T, client *APIClient) {
	t.Helper()
	client.tokenCache.mu.Lock()
	client.tokenCache.cache["app1"] = &tokenEntry{
		token:     "test-token",
		expiresAt: time.Now().Add(1 * time.Hour),
		appID:     "app1",
	}
	client.tokenCache.mu.Unlock()
}

func TestUploadC2CMedia_WithURL(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.UploadC2CMedia(context.Background(), "user1", types.MediaFileTypeImage, "https://example.com/image.png", "", "image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FileInfo != "file_info_img_001" {
		t.Fatalf("expected file_info_img_001, got %q", resp.FileInfo)
	}
}

func TestUploadC2CMedia_WithFileData(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.UploadC2CMedia(context.Background(), "user1", types.MediaFileTypeImage, "", "base64data", "image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FileInfo != "file_info_img_001" {
		t.Fatalf("expected file_info_img_001, got %q", resp.FileInfo)
	}
}

func TestUploadGroupMedia(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.UploadGroupMedia(context.Background(), "group1", types.MediaFileTypeImage, "https://example.com/image.png", "", "image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FileInfo != "file_info_grp_img_001" {
		t.Fatalf("expected file_info_grp_img_001, got %q", resp.FileInfo)
	}
}

func TestUploadC2CMedia_CacheHit(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	// First upload
	resp1, err := client.UploadC2CMedia(context.Background(), "user1", types.MediaFileTypeImage, "", "base64data", "image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second upload with same data should hit cache (empty file_uuid indicates cache hit)
	resp2, err := client.UploadC2CMedia(context.Background(), "user1", types.MediaFileTypeImage, "", "base64data", "image.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp2.FileInfo != resp1.FileInfo {
		t.Fatalf("expected same file_info from cache, got %q vs %q", resp2.FileInfo, resp1.FileInfo)
	}
}

func TestSendC2CImageMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendC2CImageMessage(context.Background(), "user1", "https://example.com/image.png", "msg-id-1", "see image")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "media-msg-001" {
		t.Fatalf("expected media-msg-001, got %q", resp.ID)
	}
}

func TestSendGroupImageMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendGroupImageMessage(context.Background(), "group1", "https://example.com/image.png", "msg-id-1", "see image")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "grp-media-msg-001" {
		t.Fatalf("expected grp-media-msg-001, got %q", resp.ID)
	}
}

func TestSendC2CVoiceMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendC2CVoiceMessage(context.Background(), "user1", "voicebase64data", "msg-id-1", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "media-msg-001" {
		t.Fatalf("expected media-msg-001, got %q", resp.ID)
	}
}

func TestSendGroupVoiceMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendGroupVoiceMessage(context.Background(), "group1", "voicebase64data", "msg-id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "grp-media-msg-001" {
		t.Fatalf("expected grp-media-msg-001, got %q", resp.ID)
	}
}

func TestSendC2CFileMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendC2CFileMessage(context.Background(), "user1", "filebase64data", "", "msg-id-1", "doc.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "media-msg-001" {
		t.Fatalf("expected media-msg-001, got %q", resp.ID)
	}
}

func TestSendGroupFileMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendGroupFileMessage(context.Background(), "group1", "filebase64data", "", "msg-id-1", "doc.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "grp-media-msg-001" {
		t.Fatalf("expected grp-media-msg-001, got %q", resp.ID)
	}
}

func TestSendC2CVideoMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendC2CVideoMessage(context.Background(), "user1", "https://example.com/video.mp4", "", "msg-id-1", "see video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "media-msg-001" {
		t.Fatalf("expected media-msg-001, got %q", resp.ID)
	}
}

func TestSendGroupVideoMessage(t *testing.T) {
	server := newMediaTestServer()
	defer server.Close()

	client := newMediaTestClient(server)
	prefillTokenForMedia(t, client)

	resp, err := client.SendGroupVideoMessage(context.Background(), "group1", "https://example.com/video.mp4", "", "msg-id-1", "see video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "grp-media-msg-001" {
		t.Fatalf("expected grp-media-msg-001, got %q", resp.ID)
	}
}
