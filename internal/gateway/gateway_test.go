package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openclaw/qqbot/internal/api"
	"github.com/openclaw/qqbot/internal/types"
)

// upgrader for test WebSocket server
var testUpgrader = websocket.Upgrader{}

// mockGatewayServer creates a test WS server that simulates the QQ Bot gateway.
type mockGatewayServer struct {
	server       *httptest.Server
	url          string
	mu           sync.Mutex
	identifyRecv atomic.Int32
	helloSent    atomic.Bool
	readySent    atomic.Bool
	ackedSeq     atomic.Int32

	// Config
	heartbeatInterval int // ms
	// Behavior
	sendHelloOnConnect bool
	sendReadyOnIdentify bool
	sessionID          string
}

func newMockGatewayServer(t *testing.T) *mockGatewayServer {
	t.Helper()
	m := &mockGatewayServer{
		heartbeatInterval:   41250,
		sendHelloOnConnect:  true,
		sendReadyOnIdentify: true,
		sessionID:           "test-session-abc",
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		m.handleConnection(t, conn)
	}))

	m.url = "ws" + strings.TrimPrefix(m.server.URL, "http")
	return m
}

func (m *mockGatewayServer) handleConnection(t *testing.T, conn *websocket.Conn) {
	if m.sendHelloOnConnect {
		hello := map[string]interface{}{
			"op": 10,
			"d": map[string]interface{}{
				"heartbeat_interval": m.heartbeatInterval,
			},
		}
		if err := conn.WriteJSON(hello); err != nil {
			t.Logf("write hello: %v", err)
			return
		}
		m.helloSent.Store(true)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(msg, &payload); err != nil {
			continue
		}

		op, _ := payload["op"].(float64)
		data := payload["d"] // "d" can be a map or a number (for heartbeat)

		switch int(op) {
	case OpIdentify:
		m.identifyRecv.Add(1)
		if m.sendReadyOnIdentify {
			ready := map[string]interface{}{
				"op": 0,
				"t":  EventReady,
				"d": map[string]interface{}{
					"session_id": m.sessionID,
					"user":       map[string]interface{}{"id": "bot-user-id"},
				},
				"s": 1,
			}
				if err := conn.WriteJSON(ready); err != nil {
					t.Logf("write ready: %v", err)
					return
				}
				m.readySent.Store(true)
			}

		case OpResume:
			// Send RESUMED event
			resumed := map[string]interface{}{
				"op": 0,
				"t":  EventResumed,
				"d":  map[string]interface{}{},
				"s":  10,
			}
			conn.WriteJSON(resumed)

	case OpHeartbeat:
		if seq, ok := data.(float64); ok {
			m.ackedSeq.Store(int32(seq))
		}
		ack := map[string]interface{}{
			"op": OpHeartbeatACK,
		}
		conn.WriteJSON(ack)
		}
	}
}

func (m *mockGatewayServer) Close() {
	m.server.Close()
}

// mockAPIServer creates a test HTTP server that mocks the QQ Bot API.
func mockAPIServer(t *testing.T, wsURL string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gateway" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"url": wsURL})
			return
		}
		http.NotFound(w, r)
	}))
}

// newTestGateway creates a Gateway with a mock API server.
func newTestGateway(t *testing.T, wsURL string) (*Gateway, *httptest.Server) {
	t.Helper()

	apiSrv := mockAPIServer(t, wsURL)

	client := api.NewAPIClient(func(c *api.APIClient) {
		// Use test API server URL
	})
	// Override apiBase by setting directly
	client.Init(context.Background(), "test-app-id", "test-secret")

	var receivedAccountID atomic.Value
	var receivedEventType atomic.Value
	var receivedPayload atomic.Value

	gw := NewGateway("test-account", client, func(accountID string, eventType string, payload []byte) {
		receivedAccountID.Store(accountID)
		receivedEventType.Store(eventType)
		receivedPayload.Store(payload)
	})
	gw.token = "test-token" // skip real token fetch

	// We need to set the apiBase to our test server
	t.Cleanup(func() {
		apiSrv.Close()
	})

	return gw, apiSrv
}

func TestNewGateway(t *testing.T) {
	gw := NewGateway("acc-1", api.NewAPIClient(), func(string, string, []byte) {})
	if gw == nil {
		t.Fatal("NewGateway returned nil")
	}
	if gw.accountID != "acc-1" {
		t.Errorf("accountID: got %q, want %q", gw.accountID, "acc-1")
	}
	if gw.IsConnected() {
		t.Error("new gateway should not be connected")
	}
}

func TestConnectIdentify(t *testing.T) {
	mockSrv := newMockGatewayServer(t)
	defer mockSrv.Close()

	gw, apiSrv := newTestGateway(t, mockSrv.url)
	defer apiSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Override apiBase to use test server
	gw.apiBase = apiSrv.URL

	err := gw.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gw.Close()

	// Wait for identify
	time.Sleep(200 * time.Millisecond)

	if !gw.IsConnected() {
		t.Error("should be connected after Connect")
	}

	if mockSrv.identifyRecv.Load() < 1 {
		t.Error("server should have received at least 1 Identify")
	}

	sessID, seq := gw.GetSessionInfo()
	if sessID == "" {
		t.Error("sessionID should be set after READY")
	}
	if seq < 0 {
		t.Error("lastSeq should be >= 0 after READY")
	}
}

func TestConnectHeartbeat(t *testing.T) {
	mockSrv := newMockGatewayServer(t)
	mockSrv.heartbeatInterval = 50 // 50ms for fast testing
	defer mockSrv.Close()

	gw, apiSrv := newTestGateway(t, mockSrv.url)
	defer apiSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gw.apiBase = apiSrv.URL

	err := gw.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gw.Close()

	// Wait for at least one heartbeat to be ACK'd
	time.Sleep(200 * time.Millisecond)

	if mockSrv.ackedSeq.Load() == 0 {
		t.Error("expected at least one heartbeat ACK to be received with 50ms interval")
	}
}

func TestConnectEventDispatch(t *testing.T) {
	// Event dispatch is covered by TestEventRouting
	t.Skip("covered by TestEventRouting")
}

func TestIntentFallback(t *testing.T) {
	// Create a server that rejects the first Identify with InvalidSession,
	// then accepts the second with lower intents.
	var identifyCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send Hello
		conn.WriteJSON(map[string]interface{}{
			"op": 10,
			"d":  map[string]interface{}{"heartbeat_interval": 41250},
		})

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var payload map[string]interface{}
			json.Unmarshal(msg, &payload)
			op, _ := payload["op"].(float64)

			switch int(op) {
			case OpIdentify:
				count := identifyCount.Add(1)
				if count == 1 {
					// First identify: send InvalidSession (can't resume)
					conn.WriteJSON(map[string]interface{}{
						"op": OpInvalidSession,
						"d":  false,
					})
				} else {
					// Second identify: send READY
					conn.WriteJSON(map[string]interface{}{
						"op": 0,
						"t":  EventReady,
						"d":  map[string]interface{}{"session_id": "fallback-session"},
						"s":  1,
					})
				}
			case OpHeartbeat:
				conn.WriteJSON(map[string]interface{}{"op": OpHeartbeatACK})
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	apiSrv := mockAPIServer(t, wsURL)
	defer apiSrv.Close()

	client := api.NewAPIClient()
	client.Init(context.Background(), "test-app-id", "test-secret")

	handlerCalled := atomic.Bool{}
	gw := NewGateway("test-account", client, func(string, string, []byte) {
		handlerCalled.Store(true)
	})
	gw.token = "test-token"
	gw.apiBase = apiSrv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gw.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect with intent fallback: %v", err)
	}
	defer gw.Close()

	time.Sleep(300 * time.Millisecond)

	if identifyCount.Load() < 2 {
		t.Errorf("expected at least 2 identifies for fallback, got %d", identifyCount.Load())
	}
}

func TestResumeSession(t *testing.T) {
	// First connection: establish session
	mockSrv := newMockGatewayServer(t)
	defer mockSrv.Close()

	apiSrv := mockAPIServer(t, mockSrv.url)
	defer apiSrv.Close()

	client := api.NewAPIClient()
	client.Init(context.Background(), "test-app-id", "test-secret")

	gw := NewGateway("test-account", client, func(string, string, []byte) {})
	gw.token = "test-token"
	gw.apiBase = apiSrv.URL

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	err := gw.Connect(ctx1)
	if err != nil {
		t.Fatalf("First Connect: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	sessID, seq := gw.GetSessionInfo()
	cancel1()
	gw.Close()

	if sessID == "" {
		t.Fatal("should have session after first connection")
	}

	// Create a second server that expects Resume
	var resumeReceived atomic.Bool
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.WriteJSON(map[string]interface{}{
			"op": 10,
			"d":  map[string]interface{}{"heartbeat_interval": 41250},
		})

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var payload map[string]interface{}
			json.Unmarshal(msg, &payload)
			op, _ := payload["op"].(float64)

			switch int(op) {
			case OpResume:
				resumeReceived.Store(true)
				conn.WriteJSON(map[string]interface{}{
					"op": 0, "t": EventResumed, "d": map[string]interface{}{}, "s": seq + 1,
				})
			case OpIdentify:
				conn.WriteJSON(map[string]interface{}{
					"op": 0, "t": EventReady,
					"d": map[string]interface{}{"session_id": "new-session"}, "s": 1,
				})
			case OpHeartbeat:
				conn.WriteJSON(map[string]interface{}{"op": OpHeartbeatACK})
			}
		}
	}))
	defer srv2.Close()

	wsURL2 := "ws" + strings.TrimPrefix(srv2.URL, "http")
	apiSrv2 := mockAPIServer(t, wsURL2)
	defer apiSrv2.Close()

	client2 := api.NewAPIClient()
	client2.Init(context.Background(), "test-app-id", "test-secret")

	gw2 := NewGateway("test-account", client2, func(string, string, []byte) {})
	gw2.token = "test-token"
	gw2.sessionID = sessID
	gw2.lastSeq = seq
	gw2.apiBase = apiSrv2.URL

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	err = gw2.Connect(ctx2)
	if err != nil {
		t.Fatalf("Second Connect: %v", err)
	}
	defer gw2.Close()

	time.Sleep(200 * time.Millisecond)

	if !resumeReceived.Load() {
		t.Error("should have sent Resume for existing session")
	}
}

func TestEventRouting(t *testing.T) {
	var receivedEvent atomic.Value
	eventReceived := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Hello
		conn.WriteJSON(map[string]interface{}{
			"op": 10, "d": map[string]interface{}{"heartbeat_interval": 41250},
		})

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var payload map[string]interface{}
			json.Unmarshal(msg, &payload)
			op, _ := payload["op"].(float64)

			switch int(op) {
			case OpIdentify:
				conn.WriteJSON(map[string]interface{}{
					"op": 0, "t": EventReady,
					"d": map[string]interface{}{"session_id": "evt-session"}, "s": 1,
				})
				// Send a C2C message event
				c2cEvent := types.C2CMessageEvent{
					Author: types.C2CAuthor{ID: "user-1", UserOpenID: "open-1"},
					Content: "hello bot",
					ID:       "msg-001",
					Timestamp: "2024-01-01T00:00:00Z",
				}
				eventData, _ := json.Marshal(c2cEvent)
				conn.WriteJSON(map[string]interface{}{
					"op": 0, "t": EventC2CMessage,
					"d": json.RawMessage(eventData),
					"s": 2,
				})
			case OpHeartbeat:
				conn.WriteJSON(map[string]interface{}{"op": OpHeartbeatACK})
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	apiSrv := mockAPIServer(t, wsURL)
	defer apiSrv.Close()

	client := api.NewAPIClient()
	client.Init(context.Background(), "test-app-id", "test-secret")

	gw := NewGateway("test-account", client, func(accountID string, eventType string, payload []byte) {
		receivedEvent.Store(eventType)
		close(eventReceived)
	})
	gw.token = "test-token"
	gw.apiBase = apiSrv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := gw.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer gw.Close()

	select {
	case <-eventReceived:
		evType := receivedEvent.Load().(string)
		if evType != EventC2CMessage {
			t.Errorf("event type: got %q, want %q", evType, EventC2CMessage)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for event")
	}
}

func TestGatewayClose(t *testing.T) {
	mockSrv := newMockGatewayServer(t)
	defer mockSrv.Close()

	apiSrv := mockAPIServer(t, mockSrv.url)
	defer apiSrv.Close()

	client := api.NewAPIClient()
	client.Init(context.Background(), "test-app-id", "test-secret")

	gw := NewGateway("test-account", client, func(string, string, []byte) {})
	gw.token = "test-token"
	gw.apiBase = apiSrv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := gw.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	gw.Close()
	cancel()

	if gw.IsConnected() {
		t.Error("should not be connected after Close")
	}
}
