package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// testServer creates a ChannelServer with a mock HTTP backend for testing.
func testServer(t *testing.T, handler http.HandlerFunc) (*ChannelServer, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := Config{Account: "acct1", QQBotAPI: ts.URL}
	cs := newChannelServer(cfg)
	return cs, ts
}

func TestHandleReply_C2C(t *testing.T) {
	var receivedPath, receivedBody string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedBody = body["content"]
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_user1", "text": "hello"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	wantPath := "/api/v1/accounts/acct1/c2c/o_user1/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
	if receivedBody != "hello" {
		t.Errorf("body = %q, want %q", receivedBody, "hello")
	}
}

func TestHandleReply_Group(t *testing.T) {
	var receivedPath string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "group:o_group1", "text": "hi group"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	wantPath := "/api/v1/accounts/acct1/groups/o_group1/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
}

func TestHandleReply_Channel(t *testing.T) {
	var receivedPath string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "channel:12345", "text": "hi channel"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	wantPath := "/api/v1/accounts/acct1/channels/12345/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
}

func TestHandleReply_DM(t *testing.T) {
	var receivedPath string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "dm:54321", "text": "hi dm"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	wantPath := "/api/v1/accounts/acct1/channels/54321/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
}

func TestHandleReply_InvalidChatID(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	tests := []struct {
		name   string
		chatID string
	}{
		{"no colon", "invalid"},
		{"just prefix", "c2c"},
		{"empty target", "c2c:"},
		{"unknown type", "unknown:id123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"chat_id": tt.chatID, "text": "hi"}

			result, err := cs.handleReply(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected error result")
			}
		})
	}
}

func TestHandleReply_MissingParams(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	// Missing text
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:x"}
	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing text")
	}

	// Missing chat_id
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"text": "hi"}
	result, err = cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestHandleReply_HTTPError(t *testing.T) {
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_u", "text": "fail"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for HTTP 500")
	}
}

func TestHandleReply_ConnectionRefused(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:1"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_u", "text": "fail"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for connection refused")
	}
}

// --- Media reply tests ---

func TestBuildMediaPath(t *testing.T) {
	tests := []struct {
		name      string
		chatType  string
		mediaType string
		targetID  string
		want      string
		wantErr   bool
	}{
		{"c2c image", "c2c", "image", "oid", "/api/v1/accounts/acct/c2c/oid/images", false},
		{"c2c file", "c2c", "file", "oid", "/api/v1/accounts/acct/c2c/oid/files", false},
		{"c2c voice", "c2c", "voice", "oid", "/api/v1/accounts/acct/c2c/oid/voice", false},
		{"c2c video", "c2c", "video", "oid", "/api/v1/accounts/acct/c2c/oid/videos", false},
		{"group image", "group", "image", "gid", "/api/v1/accounts/acct/groups/gid/images", false},
		{"channel image", "channel", "image", "id", "", true},
		{"dm video", "dm", "video", "id", "", true},
		{"invalid media", "c2c", "audio", "id", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildMediaPath("acct", tt.chatType, tt.targetID, tt.mediaType)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRequestBody(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		mediaType string
		mediaURL  string
		wantKey   string
		wantVal   string
	}{
		{"text only", "hello", "", "", "content", "hello"},
		{"image", "caption", "image", "http://img.png", "image_url", "http://img.png"},
		{"file", "desc", "file", "http://f.zip", "file_url", "http://f.zip"},
		{"voice", "tts text", "voice", "base64data", "voice_base64", "base64data"},
		{"video", "desc", "video", "http://v.mp4", "video_url", "http://v.mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := buildRequestBody(tt.text, tt.mediaType, tt.mediaURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var m map[string]string
			json.Unmarshal(body, &m)
			if m[tt.wantKey] != tt.wantVal {
				t.Errorf("%s = %q, want %q", tt.wantKey, m[tt.wantKey], tt.wantVal)
			}
		})
	}
}

func TestHandleReply_Image(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "c2c:o_user1",
		"text":       "a cute cat",
		"media_type": "image",
		"media_url":  "http://example.com/cat.png",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	if receivedPath != "/api/v1/accounts/acct1/c2c/o_user1/images" {
		t.Errorf("path = %q", receivedPath)
	}
	if receivedBody["image_url"] != "http://example.com/cat.png" {
		t.Errorf("image_url = %q", receivedBody["image_url"])
	}
	if receivedBody["content"] != "a cute cat" {
		t.Errorf("content = %q", receivedBody["content"])
	}
}

func TestHandleReply_Image_Group(t *testing.T) {
	var receivedPath string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "group:o_grp1",
		"text":       "group photo",
		"media_type": "image",
		"media_url":  "http://example.com/photo.jpg",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	if receivedPath != "/api/v1/accounts/acct1/groups/o_grp1/images" {
		t.Errorf("path = %q", receivedPath)
	}
}

func TestHandleReply_Media_UnsupportedChatType(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "channel:12345",
		"text":       "photo",
		"media_type": "image",
		"media_url":  "http://example.com/img.png",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for media on channel chat type")
	}
}

func TestHandleReply_Media_MissingURL(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "c2c:o_u",
		"text":       "photo",
		"media_type": "image",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing media_url")
	}
}

func TestHandleReply_Media_InvalidType(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "c2c:o_u",
		"text":       "data",
		"media_type": "audio",
		"media_url":  "http://x",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid media_type")
	}
}

func TestHandleReply_File(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":    "group:o_grp1",
		"text":       "project docs",
		"media_type": "file",
		"media_url":  "http://example.com/docs.pdf",
	}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	if receivedPath != "/api/v1/accounts/acct1/groups/o_grp1/files" {
		t.Errorf("path = %q", receivedPath)
	}
	if receivedBody["file_url"] != "http://example.com/docs.pdf" {
		t.Errorf("file_url = %q", receivedBody["file_url"])
	}
}
