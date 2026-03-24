package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/types"
)

func setupMessageTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if r.Header.Get("Authorization") != "QQBot test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
			return
		}

		// Read request body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Verify path and return appropriate response
		switch {
		case r.URL.Path == "/v2/users/user1/messages":
			resp := map[string]interface{}{
				"id":        "msg-001",
				"timestamp": "2024-01-01T00:00:00Z",
			}
			if r.Method == http.MethodPost {
				// Check msg_type for notification
				if msgType, ok := body["msg_type"].(float64); ok && msgType == 6 {
					resp["ext_info"] = map[string]string{"ref_idx": "ref-notify-001"}
				}
				// Add ext_info with ref_idx for regular messages to test callback
				if msgType, ok := body["msg_type"].(float64); ok && (msgType == 0 || msgType == 2 || msgType == 7) {
					resp["ext_info"] = map[string]string{"ref_idx": "ref-001"}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/channels/ch1/messages":
			resp := map[string]interface{}{
				"id":        "ch-msg-001",
				"timestamp": "2024-01-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/v2/groups/group1/messages":
			resp := map[string]interface{}{
				"id":        "grp-msg-001",
				"timestamp": "2024-01-01T00:00:00Z",
				"ext_info":  map[string]string{"ref_idx": "ref-group-001"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
		}
	}))
}

func newMessageTestClient(server *httptest.Server, opts ...ClientOption) *APIClient {
	tc := NewTokenCache()

	client := NewAPIClient(opts...)
	client.tokenCache = tc
	client.apiBase = server.URL
	client.appID = "app1"
	client.clientSecret = "secret1"

	return client
}

func prefillToken(t *testing.T, client *APIClient) {
	t.Helper()
	// Directly inject token into cache
	client.tokenCache.mu.Lock()
	client.tokenCache.cache["app1"] = &tokenEntry{
		token:     "test-token",
		expiresAt: time.Now().Add(1 * time.Hour),
		appID:     "app1",
	}
	client.tokenCache.mu.Unlock()
}

func TestSendC2CMessage_PlainText(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	resp, err := client.SendC2CMessage(context.Background(), "user1", "hello", "msg-id-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "msg-001" {
		t.Fatalf("expected msg-001, got %q", resp.ID)
	}
}

func TestSendC2CMessage_Markdown(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	var capturedBody map[string]interface{}
	captureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "QQBot test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		resp := map[string]interface{}{
			"id":        "msg-md-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-md"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer captureServer.Close()

	client := newMessageTestClient(captureServer, WithMarkdownSupport(true))
	prefillToken(t, client)

	_, err := client.SendC2CMessage(context.Background(), "user1", "**bold**", "msg-id-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify markdown body format
	if msgType, ok := capturedBody["msg_type"].(float64); !ok || msgType != 2 {
		t.Fatalf("expected msg_type=2 for markdown, got %v", capturedBody["msg_type"])
	}
	if md, ok := capturedBody["markdown"].(map[string]interface{}); !ok || md["content"] != "**bold**" {
		t.Fatalf("expected markdown.content, got %v", capturedBody["markdown"])
	}
}

func TestSendC2CMessage_PlainTextFormat(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	var capturedBody map[string]interface{}
	captureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		resp := map[string]interface{}{
			"id":        "msg-pt-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-pt"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer captureServer.Close()

	client := newMessageTestClient(captureServer, WithMarkdownSupport(false))
	prefillToken(t, client)

	_, err := client.SendC2CMessage(context.Background(), "user1", "hello", "msg-id-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgType, ok := capturedBody["msg_type"].(float64); !ok || msgType != 0 {
		t.Fatalf("expected msg_type=0 for plain text, got %v", capturedBody["msg_type"])
	}
	if content, ok := capturedBody["content"].(string); !ok || content != "hello" {
		t.Fatalf("expected content=hello, got %v", capturedBody["content"])
	}
}

func TestSendC2CMessage_MessageReference(t *testing.T) {
	var capturedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		resp := map[string]interface{}{
			"id":        "msg-ref-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-ref"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newMessageTestClient(server, WithMarkdownSupport(false))
	prefillToken(t, client)

	_, err := client.SendC2CMessage(context.Background(), "user1", "reply", "msg-id-1", "original-msg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msgID, ok := capturedBody["msg_id"].(string); !ok || msgID != "msg-id-1" {
		t.Fatalf("expected msg_id=msg-id-1, got %v", capturedBody["msg_id"])
	}
	if ref, ok := capturedBody["message_reference"].(map[string]interface{}); !ok || ref["message_id"] != "original-msg-id" {
		t.Fatalf("expected message_reference, got %v", capturedBody["message_reference"])
	}
}

func TestSendAndNotify_Callback(t *testing.T) {
	var capturedRefIdx string
	var capturedMeta types.OutboundMeta

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":        "msg-cb-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-callback-123"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newMessageTestClient(server, WithMessageSentHook(func(refIdx string, meta types.OutboundMeta) {
		capturedRefIdx = refIdx
		capturedMeta = meta
	}))
	prefillToken(t, client)

	_, err := client.SendC2CMessage(context.Background(), "user1", "test callback", "msg-id-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedRefIdx != "ref-callback-123" {
		t.Fatalf("expected refIdx ref-callback-123, got %q", capturedRefIdx)
	}
	if capturedMeta.Text != "test callback" {
		t.Fatalf("expected meta text 'test callback', got %q", capturedMeta.Text)
	}
}

func TestSendChannelMessage(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	resp, err := client.SendChannelMessage(context.Background(), "ch1", "hello channel", "msg-id-ch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "ch-msg-001" {
		t.Fatalf("expected ch-msg-001, got %q", resp.ID)
	}
}

func TestSendGroupMessage(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	resp, err := client.SendGroupMessage(context.Background(), "group1", "hello group", "msg-id-grp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "grp-msg-001" {
		t.Fatalf("expected grp-msg-001, got %q", resp.ID)
	}
}

func TestSendC2CInputNotify(t *testing.T) {
	server := setupMessageTestServer()
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	refIdx, err := client.SendC2CInputNotify(context.Background(), "user1", "msg-id-1", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if refIdx != "ref-notify-001" {
		t.Fatalf("expected ref-notify-001, got %q", refIdx)
	}
}

func TestSendProactiveC2CMessage(t *testing.T) {
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		resp := map[string]interface{}{
			"id":        "pro-c2c-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-pro-c2c"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	resp, err := client.SendProactiveC2CMessage(context.Background(), "user1", "proactive hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "pro-c2c-001" {
		t.Fatalf("expected pro-c2c-001, got %q", resp.ID)
	}
	if !called.Load() {
		t.Fatal("server was not called")
	}
}

func TestSendProactiveGroupMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id":        "pro-grp-001",
			"timestamp": "2024-01-01T00:00:00Z",
			"ext_info":  map[string]string{"ref_idx": "ref-pro-grp"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newMessageTestClient(server)
	prefillToken(t, client)

	resp, err := client.SendProactiveGroupMessage(context.Background(), "group1", "proactive group hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "pro-grp-001" {
		t.Fatalf("expected pro-grp-001, got %q", resp.ID)
	}
}
