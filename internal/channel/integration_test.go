package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// testHarness sets up a real MCP server (our ChannelServer's MCPServer) connected
// to a real MCP client via io.Pipe, simulating the production stdio transport.
type testHarness struct {
	ctx        context.Context
	cancel     context.CancelFunc
	cs         *ChannelServer
	mcpClient  *client.Client
	transport  transport.Interface
	toServer   *io.PipeWriter // inject client→server messages (e.g. permission notifications)
}

func newTestHarness(t *testing.T, qqbotAPI string) *testHarness {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	cfg := Config{
		WebhookPort: 0,
		QQBotAPI:    qqbotAPI,
		Account:     "test-acct",
	}
	cs := newChannelServer(cfg)

	// Set up pipes for bidirectional communication
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	// Start the MCP server in a goroutine using StdioServer.Listen
	// (required for session registration and SendNotificationToAllClients).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stdioServer := server.NewStdioServer(cs.MCPServer())
		_ = stdioServer.Listen(ctx, serverReader, serverWriter)
	}()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Create client transport over the pipes
	logBuf := &bytes.Buffer{}
	clientTransport := transport.NewIO(clientReader, clientWriter, io.NopCloser(logBuf))
	if err := clientTransport.Start(ctx); err != nil {
		t.Fatalf("transport.Start: %v", err)
	}

	mcpClient := client.NewClient(clientTransport)

	// Start the client to register the notification handler bridge.
	// Client.Start() calls transport.SetNotificationHandler which bridges
	// transport-level notifications to client.OnNotification handlers.
	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("client.Start: %v", err)
	}

	// Initialize the MCP connection
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		t.Fatalf("client.Initialize: %v", err)
	}

	return &testHarness{
		ctx:       ctx,
		cancel:    cancel,
		cs:        cs,
		mcpClient: mcpClient,
		transport: clientTransport,
		toServer:  clientWriter,
	}
}

func (h *testHarness) close() {
	h.transport.Close()
	h.cancel()
	time.Sleep(50 * time.Millisecond)
}

// --- Integration Tests ---

func TestIntegration_ServerInitialization(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	// If we got here, the server initialized successfully with our custom
	// MCPServer (WithExperimental, WithToolCapabilities, WithInstructions).
	// Verify it responds to ListTools.
	result, err := h.mcpClient.ListTools(h.ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result.Tools))
	}
}

func TestIntegration_ListTools(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	result, err := h.mcpClient.ListTools(h.ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result.Tools))
	}

	// Check reply tool
	var replyTool *mcp.Tool
	for i := range result.Tools {
		if result.Tools[i].Name == "reply" {
			replyTool = &result.Tools[i]
			break
		}
	}
	if replyTool == nil {
		t.Fatal("missing reply tool")
	}
	if replyTool.Name != "reply" {
		t.Errorf("expected tool name %q, got %q", "reply", replyTool.Name)
	}

	// Verify tool schema has chat_id, text, media_type, and media_url properties
	if len(replyTool.InputSchema.Properties) != 4 {
		t.Errorf("expected 4 properties, got %d", len(replyTool.InputSchema.Properties))
	}
	if _, ok := replyTool.InputSchema.Properties["chat_id"]; !ok {
		t.Error("missing chat_id parameter")
	}
	if _, ok := replyTool.InputSchema.Properties["text"]; !ok {
		t.Error("missing text parameter")
	}
	if len(replyTool.InputSchema.Required) != 2 {
		t.Errorf("expected 2 required params, got %d", len(replyTool.InputSchema.Required))
	}
}

func TestIntegration_CallReplyTool_Success(t *testing.T) {
	var receivedPath, receivedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedBody = body["content"]
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	req := mcp.CallToolRequest{}
	req.Params.Name = "reply"
	req.Params.Arguments = map[string]any{
		"chat_id": "c2c:o_user1",
		"text":    "hello from test",
	}

	result, err := h.mcpClient.CallTool(h.ctx, req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	wantPath := "/api/v1/accounts/test-acct/c2c/o_user1/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
	if receivedBody != "hello from test" {
		t.Errorf("body = %q, want %q", receivedBody, "hello from test")
	}
}

func TestIntegration_CallReplyTool_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	req := mcp.CallToolRequest{}
	req.Params.Name = "reply"
	req.Params.Arguments = map[string]any{
		"chat_id": "c2c:o_u",
		"text":    "fail",
	}

	result, err := h.mcpClient.CallTool(h.ctx, req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for HTTP 500")
	}
}

func metaValue(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func TestIntegration_SendNotificationToClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	notifCh := make(chan mcp.JSONRPCNotification, 1)
	h.mcpClient.OnNotification(func(notification mcp.JSONRPCNotification) {
		notifCh <- notification
	})

	// Send notification via MCP server (simulating pushNotification)
	h.cs.mcp.SendNotificationToAllClients("notifications/claude/channel", map[string]any{
		"content": "test message from QQ",
		"meta": map[string]string{
			"source":  "qq",
			"sender":  "o_user123",
			"chat_id": "c2c:o_user123",
		},
	})

	select {
	case notif := <-notifCh:
		if notif.Method != "notifications/claude/channel" {
			t.Errorf("method = %q, want %q", notif.Method, "notifications/claude/channel")
		}
		content, ok := notif.Params.AdditionalFields["content"].(string)
		if !ok || content != "test message from QQ" {
			t.Errorf("content = %v", notif.Params.AdditionalFields["content"])
		}
		meta, ok := notif.Params.AdditionalFields["meta"].(map[string]any)
		if !ok {
			t.Fatalf("meta type assertion failed: %T", notif.Params.AdditionalFields["meta"])
		}
		if metaValue(meta, "source") != "qq" || metaValue(meta, "sender") != "o_user123" || metaValue(meta, "chat_id") != "c2c:o_user123" {
			t.Errorf("meta = %v", meta)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestIntegration_FullWebhookToNotificationPipeline(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	h.cs.config.WebhookPort = 18799
	defer h.close()

	notifCh := make(chan mcp.JSONRPCNotification, 1)
	h.mcpClient.OnNotification(func(notification mcp.JSONRPCNotification) {
		if notification.Method == "notifications/claude/channel" {
			notifCh <- notification
		}
	})

	// Start webhook server
	webhookCtx, webhookCancel := context.WithCancel(h.ctx)
	defer webhookCancel()
	go h.cs.startWebhookServer(webhookCtx)
	time.Sleep(100 * time.Millisecond)

	// Send a webhook request simulating a QQ C2C message
	webhookBody := `{
		"account_id": "test-acct",
		"event_type": "C2C_MESSAGE_CREATE",
		"timestamp": "2026-01-01T00:00:00Z",
		"data": {
			"content": "hello from QQ user",
			"author": {"user_openid": "o_testuser", "id": "xxx", "union_openid": "xxx"},
			"id": "msg1",
			"timestamp": "2026-01-01T00:00:00Z"
		}
	}`
	resp, err := http.Post("http://127.0.0.1:18799/webhook", "application/json",
		bytes.NewReader([]byte(webhookBody)))
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook returned %d", resp.StatusCode)
	}

	select {
	case notif := <-notifCh:
		content, _ := notif.Params.AdditionalFields["content"].(string)
		if content != "hello from QQ user" {
			t.Errorf("content = %q, want %q", content, "hello from QQ user")
		}
		meta, _ := notif.Params.AdditionalFields["meta"].(map[string]any)
		if metaValue(meta, "sender") != "o_testuser" || metaValue(meta, "chat_id") != "c2c:o_testuser" {
			t.Errorf("meta = %v", meta)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook->notification")
	}
}

func TestIntegration_FullReplyPipeline(t *testing.T) {
	var receivedPath, receivedBody string
	var callCount atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		receivedPath = r.URL.Path
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		receivedBody = body["content"]
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	// Step 1: Simulate QQ user sends a message via notification
	notifCh := make(chan mcp.JSONRPCNotification, 1)
	h.mcpClient.OnNotification(func(notification mcp.JSONRPCNotification) {
		notifCh <- notification
	})

	h.cs.mcp.SendNotificationToAllClients("notifications/claude/channel", map[string]any{
		"content": "what is 2+2?",
		"meta": map[string]string{
			"source":  "qq",
			"sender":  "o_mathuser",
			"chat_id": "c2c:o_mathuser",
		},
	})

	// Step 2: Client receives the notification
	select {
	case notif := <-notifCh:
		content, _ := notif.Params.AdditionalFields["content"].(string)
		if content != "what is 2+2?" {
			t.Fatalf("unexpected content: %q", content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}

	// Step 3: Client calls reply tool
	req := mcp.CallToolRequest{}
	req.Params.Name = "reply"
	req.Params.Arguments = map[string]any{
		"chat_id": "c2c:o_mathuser",
		"text":    "2+2 = 4",
	}

	result, err := h.mcpClient.CallTool(h.ctx, req)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	// Step 4: Verify the reply was sent to qqbot API
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount.Load())
	}
	wantPath := "/api/v1/accounts/test-acct/c2c/o_mathuser/messages"
	if receivedPath != wantPath {
		t.Errorf("path = %q, want %q", receivedPath, wantPath)
	}
	if receivedBody != "2+2 = 4" {
		t.Errorf("body = %q, want %q", receivedBody, "2+2 = 4")
	}
}

// TestIntegration_PermissionRelay tests the full permission relay flow:
// 1. QQ message arrives via ForwardMessage (sets chat_id tracking)
// 2. CodeBuddy Code sends a permission_request notification to the MCP server
// 3. The server forwards the request to QQ via sender.Send()
func TestIntegration_PermissionRelay(t *testing.T) {
	type permissionCall struct {
		path string
		text string
	}
	callCh := make(chan permissionCall, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		callCh <- permissionCall{path: r.URL.Path, text: body["content"]}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	h := newTestHarness(t, ts.URL)
	defer h.close()

	// Step 1: Simulate a QQ user sending a message (sets chat_id tracking)
	h.cs.ForwardMessage("o_permuser", "c2c:o_permuser", "do something dangerous")

	// Step 2: Simulate CodeBuddy Code sending a permission_request notification.
	// In MCP, client→server notifications are sent as JSON-RPC over the transport.
	// We inject a raw JSON-RPC notification through the pipe.
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/claude/channel/permission_request",
		"params": map[string]any{
			"request_id":    "abcde",
			"tool_name":     "Bash",
			"description":   "Execute shell command",
			"input_preview": "rm -rf /tmp/test",
		},
	}
	notifJSON, _ := json.Marshal(notif)
	notifJSON = append(notifJSON, '\n')
	h.toServer.Write(notifJSON)

	// Step 3: Wait for the permission request to be forwarded to QQ via HTTP
	select {
	case call := <-callCh:
		wantPath := "/api/v1/accounts/test-acct/c2c/o_permuser/messages"
		if call.path != wantPath {
			t.Errorf("path = %q, want %q", call.path, wantPath)
		}
		// Verify content includes request_id and reply instructions
		if !contains(call.text, "abcde") {
			t.Errorf("text should contain request_id 'abcde', got %q", call.text)
		}
		if !contains(call.text, "yes abcde") {
			t.Errorf("text should contain 'yes abcde', got %q", call.text)
		}
		if !contains(call.text, "no abcde") {
			t.Errorf("text should contain 'no abcde', got %q", call.text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission request to be forwarded to QQ")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
