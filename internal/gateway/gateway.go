package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openclaw/qqbot/internal/api"
	"github.com/openclaw/qqbot/internal/types"
)

// EventHandler is called when a message event is received from the gateway.
type EventHandler func(accountID string, eventType string, payload []byte)

// Gateway manages the WebSocket connection to the QQ Bot gateway.
type Gateway struct {
	accountID    string
	client       *api.APIClient
	apiBase      string // overridable for testing
	token        string // pre-set token, used by getGatewayURL
	conn         *websocket.Conn
	mu           sync.Mutex
	intentIndex  int
	sessionID    string
	lastSeq      int
	queue        *MessageQueue
	reconnect    ReconnectConfig
	eventHandler EventHandler
	connected    bool
	muConn       sync.Mutex
	cancel       context.CancelFunc

	// Heartbeat
	heartbeatTimer *time.Timer
	heartbeatAck   chan struct{}

	// Synchronization for Connect returning after READY
	readyCh chan struct{} // closed when READY or RESUMED is received
}

// NewGateway creates a new Gateway instance.
func NewGateway(accountID string, client *api.APIClient, handler EventHandler) *Gateway {
	return &Gateway{
		accountID:    accountID,
		client:       client,
		apiBase:      api.APIBase,
		intentIndex:  0,
		queue:        NewMessageQueue(defaultMaxConcurrentUsers, defaultPerUserQueueSize),
		reconnect:    DefaultReconnectConfig,
		eventHandler: handler,
		heartbeatAck: make(chan struct{}, 1),
		readyCh:      make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection with intent fallback and reconnection.
// It blocks until READY is received (or context cancelled), then returns nil.
// The read loop continues running in the background.
func (g *Gateway) Connect(ctx context.Context) error {
	ctx, g.cancel = context.WithCancel(ctx)
	g.queue.Start()

	// Reset readyCh
	g.mu.Lock()
	g.readyCh = make(chan struct{})
	g.mu.Unlock()

	go g.connectLoop(ctx)

	// Wait for READY or context cancellation
	select {
	case <-g.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// connectLoop implements the main connect-reconnect loop.
func (g *Gateway) connectLoop(ctx context.Context) {
	var reconnectAttempts int

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if reconnectAttempts >= g.reconnect.MaxAttempts {
			log.Printf("[gateway:%s] max reconnect attempts (%d) reached", g.accountID, g.reconnect.MaxAttempts)
			return
		}

		err := g.connectOnce(ctx)
		if err != nil {
			log.Printf("[gateway:%s] connection attempt failed: %v", g.accountID, err)
		}

		// Disconnected (or intent fallback needed)
		g.setConnected(false)
		g.stopHeartbeat()

		select {
		case <-ctx.Done():
			return
		case <-time.After(g.reconnect.GetDelay(reconnectAttempts)):
			reconnectAttempts++
		}
	}
}

// connectOnce performs a single connection attempt with intent fallback.
func (g *Gateway) connectOnce(ctx context.Context) error {
	wsURL, token, err := g.getGatewayURLAndToken(ctx)
	if err != nil {
		return fmt.Errorf("get gateway URL: %w", err)
	}

	for levelIdx := g.intentIndex; levelIdx < len(types.DefaultIntentLevels); levelIdx++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		g.intentIndex = levelIdx
		err := g.tryConnect(ctx, wsURL, token, levelIdx)
		if err == nil {
			return nil // connected successfully
		}

		log.Printf("[gateway:%s] Intent level %d failed: %v, trying next", g.accountID, levelIdx, err)
	}

	return fmt.Errorf("all intent levels failed")
}

// tryConnect attempts WebSocket connection at a given intent level.
func (g *Gateway) tryConnect(ctx context.Context, wsURL, token string, levelIdx int) error {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	g.muConn.Lock()
	g.conn = conn
	g.muConn.Unlock()

	// Wait for Hello
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read hello: %w", err)
	}

	var helloPayload types.WSPayload
	if err := json.Unmarshal(msg, &helloPayload); err != nil {
		return fmt.Errorf("parse hello: %w", err)
	}

	if helloPayload.Op != OpHello {
		return fmt.Errorf("expected Hello (op=10), got op=%d", helloPayload.Op)
	}

	var helloData struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := json.Unmarshal(helloPayload.Data, &helloData); err != nil {
		return fmt.Errorf("parse hello data: %w", err)
	}
	heartbeatInterval := time.Duration(helloData.HeartbeatInterval) * time.Millisecond

	// Send Identify or Resume
	if g.sessionID != "" {
		resumeData := struct {
			Op int           `json:"op"`
			D  ResumeParams `json:"d"`
		}{
			Op: OpResume,
			D: ResumeParams{
				Token:     "QQBot " + token,
				SessionID: g.sessionID,
				Seq:       g.lastSeq,
			},
		}
		if err := conn.WriteJSON(resumeData); err != nil {
			return fmt.Errorf("send resume: %w", err)
		}
	} else {
		intentLevel := types.DefaultIntentLevels[levelIdx]
		identifyData := struct {
			Op int            `json:"op"`
			D  IdentifyParams `json:"d"`
		}{
			Op: OpIdentify,
			D: IdentifyParams{
				Token:   "QQBot " + token,
				Intents: intentLevel.Intents,
				Shard:   []int{0, 1},
			},
		}
		if err := conn.WriteJSON(identifyData); err != nil {
			return fmt.Errorf("send identify: %w", err)
		}
	}

	// Start heartbeat
	g.stopHeartbeat()
	g.heartbeatAck = make(chan struct{}, 1)
	g.startHeartbeat(heartbeatInterval)

	// Read loop (blocks until disconnection)
	return g.readLoop(ctx)
}

// readLoop processes incoming WebSocket messages. Blocks until disconnect or context cancellation.
func (g *Gateway) readLoop(ctx context.Context) error {
	conn := g.getConn()
	if conn == nil {
		return fmt.Errorf("no connection")
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			g.setConnected(false)
			return fmt.Errorf("read message: %w", err)
		}

		var payload types.WSPayload
		if err := json.Unmarshal(msg, &payload); err != nil {
			log.Printf("[gateway:%s] parse error: %v", g.accountID, err)
			continue
		}

		if payload.Seq != nil {
			g.muConn.Lock()
			g.lastSeq = *payload.Seq
			g.muConn.Unlock()
		}

		g.handlePayload(ctx, &payload)
	}
}

// handlePayload dispatches a received WebSocket payload.
func (g *Gateway) handlePayload(ctx context.Context, payload *types.WSPayload) {
	switch payload.Op {
	case OpDispatch:
		g.handleDispatch(payload)

	case OpHeartbeatACK:
		select {
		case g.heartbeatAck <- struct{}{}:
		default:
		}

	case OpReconnect:
		g.setConnected(false)
		conn := g.getConn()
		if conn != nil {
			conn.Close()
		}

	case OpInvalidSession:
		canResume := false
		if payload.Data != nil {
			json.Unmarshal(payload.Data, &canResume)
		}
		if !canResume {
			g.muConn.Lock()
			g.sessionID = ""
			g.lastSeq = 0
			g.muConn.Unlock()
			if g.intentIndex < len(types.DefaultIntentLevels)-1 {
				g.intentIndex++
			}
		}
		g.setConnected(false)
		conn := g.getConn()
		if conn != nil {
			conn.Close()
		}
	}
}

// handleDispatch processes a Dispatch (op=0) event.
func (g *Gateway) handleDispatch(payload *types.WSPayload) {
	if payload.Event == nil {
		return
	}
	eventName := *payload.Event

	switch eventName {
	case EventReady:
		var readyData struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal(payload.Data, &readyData); err == nil {
			g.muConn.Lock()
			g.sessionID = readyData.SessionID
			g.connected = true
			g.muConn.Unlock()
		}
		g.signalReady()

	case EventResumed:
		g.setConnected(true)
		g.signalReady()

	case EventC2CMessage, EventGroupMessage, EventGuildMessage, EventGuildDM:
		if g.eventHandler != nil {
			g.queue.Enqueue(&MessageItem{
				ID:        fmt.Sprintf("event-%s-%d", eventName, g.lastSeq),
				AccountID: g.accountID,
				EventType: eventName,
				Payload:   payload.Data,
				Handler: func() error {
					g.eventHandler(g.accountID, eventName, payload.Data)
					return nil
				},
			})
		}
	}
}

func (g *Gateway) signalReady() {
	g.mu.Lock()
	ch := g.readyCh
	g.mu.Unlock()
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}

// startHeartbeat sends periodic heartbeats.
func (g *Gateway) startHeartbeat(interval time.Duration) {
	g.mu.Lock()
	g.heartbeatTimer = time.AfterFunc(interval, func() {
		g.sendHeartbeat(interval)
	})
	g.mu.Unlock()
}

func (g *Gateway) sendHeartbeat(interval time.Duration) {
	conn := g.getConn()
	if conn == nil {
		return
	}

	g.muConn.Lock()
	seq := g.lastSeq
	g.muConn.Unlock()

	data := map[string]interface{}{"op": OpHeartbeat, "d": seq}
	if err := conn.WriteJSON(data); err != nil {
		return
	}

	g.mu.Lock()
	g.heartbeatTimer = time.AfterFunc(interval, func() {
		g.sendHeartbeat(interval)
	})
	g.mu.Unlock()
}

// stopHeartbeat stops the heartbeat timer.
func (g *Gateway) stopHeartbeat() {
	g.mu.Lock()
	if g.heartbeatTimer != nil {
		g.heartbeatTimer.Stop()
		g.heartbeatTimer = nil
	}
	g.mu.Unlock()
}

// Close gracefully closes the gateway connection.
func (g *Gateway) Close() {
	g.stopHeartbeat()
	g.setConnected(false)
	g.muConn.Lock()
	if g.conn != nil {
		g.conn.Close()
		g.conn = nil
	}
	g.muConn.Unlock()
	g.queue.Stop()
	if g.cancel != nil {
		g.cancel()
	}
}

// IsConnected returns whether the gateway is currently connected.
func (g *Gateway) IsConnected() bool {
	g.muConn.Lock()
	defer g.muConn.Unlock()
	return g.connected
}

// GetSessionInfo returns the current session ID and last sequence number.
func (g *Gateway) GetSessionInfo() (sessionID string, lastSeq int) {
	g.muConn.Lock()
	defer g.muConn.Unlock()
	return g.sessionID, g.lastSeq
}

func (g *Gateway) setConnected(v bool) {
	g.muConn.Lock()
	g.connected = v
	g.muConn.Unlock()
}

func (g *Gateway) getConn() *websocket.Conn {
	g.muConn.Lock()
	defer g.muConn.Unlock()
	return g.conn
}

// getGatewayURLAndToken fetches the WebSocket gateway URL and access token from the API.
func (g *Gateway) getGatewayURLAndToken(ctx context.Context) (wsURL, token string, err error) {
	token = g.token
	if token == "" {
		token, err = g.client.GetAccessToken(ctx)
		if err != nil {
			return "", "", fmt.Errorf("get access token: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.apiBase+"/gateway", nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request gateway: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("parse gateway response: %w", err)
	}

	if result.URL == "" {
		return "", "", fmt.Errorf("empty gateway URL in response")
	}

	return result.URL, token, nil
}

// GetAccessToken exposes the token getter for the gateway.
func (g *Gateway) GetAccessToken(ctx context.Context) (string, error) {
	return g.client.GetAccessToken(ctx)
}
