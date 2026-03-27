package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleReply_C2C(t *testing.T) {
	var receivedPath, receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedBody = body["content"]
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: ts.URL},
	}

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: ts.URL},
	}

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: ts.URL},
	}

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: ts.URL},
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "dm:54321", "text": "hi dm"}

	result, err := cs.handleReply(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	// dm uses /channels/ API path (same as channel)
	wantPath := "/api/v1/accounts/acct1/channels/54321/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
}

func TestHandleReply_InvalidChatID(t *testing.T) {
	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"},
	}

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
	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"},
	}

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: ts.URL},
	}

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
	cs := &ChannelServer{
		config: Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:1"},
	}

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
