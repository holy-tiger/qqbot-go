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

// SessionPersister defines the interface for persisting WebSocket session state.
type SessionPersister interface {
	Load(accountID, expectedAppID string) *SessionData
	Save(data SessionData)
	UpdateLastSeq(accountID string, lastSeq int)
}

// SessionData represents a persistent WebSocket session state.
type SessionData struct {
	SessionID        string
	LastSeq          int
	IntentLevelIndex int
	AccountID        string
	AppID            string
}

// gatewayHTTPTimeout is the timeout for gateway URL fetch requests.
const gatewayHTTPTimeout = 30 * time.Second

// heartbeatAckFactor is the multiplier on heartbeat interval for ACK timeout.
const heartbeatAckFactor = 2

// Gateway manages the WebSocket connection to the QQ Bot gateway.
type Gateway struct {
	accountID    string
	client       *api.APIClient
	apiBase      string // overridable for testing
	token        string // pre-set token, used by getGatewayURL
	conn         *websocket.Conn
	mu           sync.Mutex
	sessionID    string
	lastSeq      int
	intentIndex  int // P0-1: now always accessed under muConn
	queue        *MessageQueue
	reconnect    ReconnectConfig
	eventHandler EventHandler
	connected    bool
	didConnect   bool // P0-3: set true on READY/RESUMED, checked by connectLoop
	muConn       sync.Mutex
	cancel       context.CancelFunc
	sessionStore SessionPersister
	appID        string
	httpClient   *http.Client // P0-4: dedicated client with timeout

	// Heartbeat
	heartbeatTimer  *time.Timer
	heartbeatAck    chan struct{}
	heartbeatTimeout *time.Timer // P1-6: ACK timeout timer

	// Synchronization for Connect returning after READY
	readyCh   chan struct{} // closed when READY or RESUMED is received
	readyOnce *sync.Once   // protects readyCh from double-close
}

// NewGateway creates a new Gateway instance.
func NewGateway(accountID string, client *api.APIClient, handler EventHandler) *Gateway {
	return &Gateway{
		accountID:   accountID,
		client:      client,
		apiBase:     api.APIBase,
		intentIndex: 0,
		queue:       NewMessageQueue(defaultMaxConcurrentUsers, defaultPerUserQueueSize),
		reconnect:   DefaultReconnectConfig,
		eventHandler: handler,
		heartbeatAck: make(chan struct{}, 1),
		readyCh:     make(chan struct{}),
		readyOnce:   &sync.Once{},
		httpClient:  &http.Client{Timeout: gatewayHTTPTimeout}, // P0-4
	}
}

// SetSessionStore configures session persistence for the gateway.
func (g *Gateway) SetSessionStore(store SessionPersister, appID string) {
	g.sessionStore = store
	g.appID = appID
}

// HasSessionStore returns whether a session store is configured.
func (g *Gateway) HasSessionStore() bool {
	return g.sessionStore != nil
}

// Connect establishes the WebSocket connection with intent fallback and reconnection.
// It blocks until READY is received (or context cancelled), then returns nil.
// The read loop continues running in the background.
func (g *Gateway) Connect(ctx context.Context) error {
	ctx, g.cancel = context.WithCancel(ctx)
	g.queue.Start()

	// Reset readyCh and readyOnce
	g.mu.Lock()
	g.readyCh = make(chan struct{})
	g.readyOnce = &sync.Once{}
	g.mu.Unlock()

	// Load persisted session state
	if g.sessionStore != nil {
		state := g.sessionStore.Load(g.accountID, g.appID)
		if state != nil {
			g.muConn.Lock()
			g.sessionID = state.SessionID
			g.lastSeq = state.LastSeq
			g.intentIndex = state.IntentLevelIndex // P0-1: under muConn
			g.muConn.Unlock()
			log.Printf("[gateway:%s] restored session (seq=%d, intent=%d)", g.accountID, g.lastSeq, g.intentIndex)
		}
	}

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
	var disconnectTimes []time.Time // P1-9: track disconnect times for ShouldQuickStop

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

		// P1-9: check for rapid disconnects
		if g.reconnect.ShouldQuickStop(disconnectTimes) {
			log.Printf("[gateway:%s] quick disconnect detected, stopping reconnect", g.accountID)
			return
		}

		err := g.connectOnce(ctx)
		if err != nil {
			log.Printf("[gateway:%s] connection attempt failed: %v", g.accountID, err)
		}

		// P0-3: if we successfully connected before disconnecting, reset the counter
		g.muConn.Lock()
		wasConnected := g.didConnect
		if wasConnected {
			reconnectAttempts = 0
			disconnectTimes = nil
		}
		g.didConnect = false
		g.muConn.Unlock()

		if wasConnected {
			disconnectTimes = append(disconnectTimes, time.Now())
		}

		// Disconnected (or intent fallback needed)
		g.setConnected(false)
		g.stopHeartbeat()
		g.stopHeartbeatTimeout() // P1-6

		// P1-9: use RateLimitDelay for rate-limited reconnects
		delay := g.reconnect.GetDelay(reconnectAttempts)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			if !wasConnected {
				reconnectAttempts++
			}
		}
	}
}

// connectOnce performs a single connection attempt with intent fallback.
func (g *Gateway) connectOnce(ctx context.Context) error {
	wsURL, token, err := g.getGatewayURLAndToken(ctx)
	if err != nil {
		return fmt.Errorf("get gateway URL: %w", err)
	}

	// P0-1: read intentIndex under muConn
	g.muConn.Lock()
	startIdx := g.intentIndex
	g.muConn.Unlock()

	for levelIdx := startIdx; levelIdx < len(types.DefaultIntentLevels); levelIdx++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// P0-1: write intentIndex under muConn
		g.muConn.Lock()
		g.intentIndex = levelIdx
		g.muConn.Unlock()

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

	// P0-2: close old connection before assigning new one
	g.muConn.Lock()
	if g.conn != nil {
		g.conn.Close()
	}
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
	g.muConn.Lock()
	sessionID := g.sessionID
	lastSeq := g.lastSeq
	g.muConn.Unlock()

	if sessionID != "" {
		resumeData := struct {
			Op int           `json:"op"`
			D  ResumeParams `json:"d"`
		}{
			Op: OpResume,
			D: ResumeParams{
				Token:     "QQBot " + token,
				SessionID: sessionID,
				Seq:       lastSeq,
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

	// Start heartbeat with ACK timeout monitoring
	g.stopHeartbeat()
	g.stopHeartbeatTimeout()
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

		// Persist seq periodically (every message is cheap for SQLite)
		g.muConn.Lock()
		seq := g.lastSeq
		g.muConn.Unlock()
		if g.sessionStore != nil {
			g.sessionStore.UpdateLastSeq(g.accountID, seq)
		}
	}
}

// handlePayload dispatches a received WebSocket payload.
func (g *Gateway) handlePayload(ctx context.Context, payload *types.WSPayload) {
	switch payload.Op {
	case OpDispatch:
		g.handleDispatch(payload)

	case OpHeartbeatACK:
		g.stopHeartbeatTimeout() // P1-6: ACK received, cancel timeout
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
			// P0-1: write intentIndex under muConn
			g.muConn.Lock()
			g.sessionID = ""
			g.lastSeq = 0
			if g.intentIndex < len(types.DefaultIntentLevels)-1 {
				g.intentIndex++
			}
			g.muConn.Unlock()
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
			g.didConnect = true // P0-3: mark that we had a successful connection
			g.muConn.Unlock()
		}
		g.saveSession()
		g.signalReady()

	case EventResumed:
		g.muConn.Lock()
		g.connected = true
		g.didConnect = true // P0-3: mark that we had a successful connection
		g.muConn.Unlock()
		g.signalReady()

	case EventC2CMessage, EventGroupMessage, EventGuildMessage, EventGuildDM:
		if g.eventHandler != nil {
			g.muConn.Lock()
			seq := g.lastSeq
			g.muConn.Unlock()
			g.queue.Enqueue(&MessageItem{
				ID:        fmt.Sprintf("event-%s-%d", eventName, seq),
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
	once := g.readyOnce
	ch := g.readyCh
	g.mu.Unlock()
	once.Do(func() {
		close(ch)
	})
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
		log.Printf("[gateway:%s] heartbeat write failed: %v", g.accountID, err)
		return
	}

	// P1-6: start ACK timeout — if no ACK within 2x interval, force disconnect
	g.startHeartbeatTimeout(interval * heartbeatAckFactor)

	g.mu.Lock()
	g.heartbeatTimer = time.AfterFunc(interval, func() {
		g.sendHeartbeat(interval)
	})
	g.mu.Unlock()
}

// startHeartbeatTimeout starts a timer that forces disconnect if no ACK is received.
func (g *Gateway) startHeartbeatTimeout(timeout time.Duration) {
	g.mu.Lock()
	g.heartbeatTimeout = time.AfterFunc(timeout, func() {
		log.Printf("[gateway:%s] heartbeat ACK timeout, forcing disconnect", g.accountID)
		conn := g.getConn()
		if conn != nil {
			conn.Close()
		}
	})
	g.mu.Unlock()
}

// stopHeartbeatTimeout stops the heartbeat ACK timeout timer.
func (g *Gateway) stopHeartbeatTimeout() {
	g.mu.Lock()
	if g.heartbeatTimeout != nil {
		g.heartbeatTimeout.Stop()
		g.heartbeatTimeout = nil
	}
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
	g.stopHeartbeatTimeout()
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

	// P0-4: use dedicated httpClient with timeout instead of http.DefaultClient
	resp, err := g.httpClient.Do(req)
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

// saveSession persists current session state if a store is configured.
func (g *Gateway) saveSession() {
	if g.sessionStore == nil {
		return
	}
	g.muConn.Lock()
	data := SessionData{
		SessionID:        g.sessionID,
		LastSeq:          g.lastSeq,
		IntentLevelIndex: g.intentIndex, // P0-1: already under muConn
		AccountID:        g.accountID,
		AppID:            g.appID,
	}
	g.muConn.Unlock()
	g.sessionStore.Save(data)
}
