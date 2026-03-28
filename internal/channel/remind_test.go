package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleRemind_C2C(t *testing.T) {
	var receivedMethod, receivedPath string
	var receivedBody remindRequest
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse{
			OK: true,
			Data: &remindResponse{
				JobID:    "rem-123",
				NextRun:  "2026-03-28T10:00:00Z",
				Schedule: "@every 30m",
			},
		})
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id":  "c2c:o_user1",
		"text":     "该喝水了",
		"schedule": "@every 30m",
	}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", receivedMethod)
	}
	if receivedPath != "/api/v1/accounts/acct1/reminders" {
		t.Errorf("path = %q, want /api/v1/accounts/acct1/reminders", receivedPath)
	}
	if receivedBody.TargetType != "c2c" {
		t.Errorf("target_type = %q, want c2c", receivedBody.TargetType)
	}
	if receivedBody.TargetAddress != "o_user1" {
		t.Errorf("target_address = %q, want o_user1", receivedBody.TargetAddress)
	}
	if receivedBody.Content != "该喝水了" {
		t.Errorf("content = %q", receivedBody.Content)
	}
	if receivedBody.Schedule != "@every 30m" {
		t.Errorf("schedule = %q", receivedBody.Schedule)
	}
}

func TestHandleRemind_Group(t *testing.T) {
	var receivedBody remindRequest
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse{
			OK:   true,
			Data: &remindResponse{JobID: "rem-456"},
		})
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"chat_id": "group:o_group1",
		"text":    "开会时间到了",
	}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	if receivedBody.TargetType != "group" {
		t.Errorf("target_type = %q, want group", receivedBody.TargetType)
	}
	if receivedBody.TargetAddress != "o_group1" {
		t.Errorf("target_address = %q", receivedBody.TargetAddress)
	}
	if receivedBody.Schedule != "" {
		t.Errorf("schedule = %q, want empty for one-time", receivedBody.Schedule)
	}
}

func TestHandleRemind_UnsupportedChatType(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	tests := []struct {
		name   string
		chatID string
	}{
		{"channel", "channel:12345"},
		{"dm", "dm:54321"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{
				"chat_id": tt.chatID,
				"text":    "提醒",
			}

			result, err := cs.handleRemind(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected error for unsupported chat type")
			}
		})
	}
}

func TestHandleRemind_InvalidChatID(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "invalid", "text": "提醒"}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid chat_id")
	}
}

func TestHandleRemind_MissingParams(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	// Missing text
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:x"}
	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing text")
	}

	// Missing chat_id
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"text": "提醒"}
	result, err = cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestHandleRemind_HTTPError(t *testing.T) {
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_u", "text": "fail"}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for HTTP 500")
	}
}

func TestHandleRemind_ConnectionRefused(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:1"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_u", "text": "fail"}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for connection refused")
	}
}

func TestHandleRemind_ResultText(t *testing.T) {
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse{
			OK:   true,
			Data: &remindResponse{JobID: "rem-abc123"},
		})
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"chat_id": "c2c:o_u", "text": "test"}

	result, err := cs.handleRemind(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text != "reminded (job_id=rem-abc123)" {
		t.Errorf("result text = %q, want %q", text, "reminded (job_id=rem-abc123)")
	}
}

// --- cancel_reminder tests ---

func TestHandleCancelReminder(t *testing.T) {
	var receivedMethod, receivedPath string
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"data": map[string]string{"status": "cancelled"},
		})
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"job_id": "rem-123"}

	result, err := cs.handleCancelReminder(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].(mcp.TextContent).Text)
	}

	if receivedMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", receivedMethod)
	}
	wantPath := "/api/v1/accounts/acct1/reminders/rem-123"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if text != "cancelled" {
		t.Errorf("result text = %q, want %q", text, "cancelled")
	}
}

func TestHandleCancelReminder_MissingJobID(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:9090"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := cs.handleCancelReminder(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing job_id")
	}
}

func TestHandleCancelReminder_NotFound(t *testing.T) {
	cs, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"job_id": "rem-nonexistent"}

	result, err := cs.handleCancelReminder(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for 404")
	}
}

func TestHandleCancelReminder_ConnectionRefused(t *testing.T) {
	cs := newChannelServer(Config{Account: "acct1", QQBotAPI: "http://127.0.0.1:1"})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"job_id": "rem-123"}

	result, err := cs.handleCancelReminder(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for connection refused")
	}
}
