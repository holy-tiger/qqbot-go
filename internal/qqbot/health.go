package qqbot

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

// HealthResponse is the JSON response from the health check endpoint.
type HealthResponse struct {
	Status    string           `json:"status"`
	Uptime    string           `json:"uptime"`
	Version   string           `json:"version"`
	Accounts  []AccountHealth  `json:"accounts,omitempty"`
	Timestamp string           `json:"timestamp"`
}

// AccountHealth represents the health of a single account.
type AccountHealth struct {
	ID          string `json:"id"`
	Connected   bool   `json:"connected"`
	TokenStatus string `json:"token_status,omitempty"`
	Error       string `json:"error,omitempty"`
}

// HealthServer provides an HTTP health check endpoint.
type HealthServer struct {
	mux       *http.ServeMux
	server    *http.Server
	listener  net.Listener
	mu        sync.RWMutex
	statuses  []AccountHealth
	manager   *BotManager
	startTime time.Time
}

// NewHealthServer creates a new health check server.
func NewHealthServer(mgr *BotManager, version string) *HealthServer {
	hs := &HealthServer{
		manager:   mgr,
		startTime: time.Now(),
		mux:       http.NewServeMux(),
	}
	hs.mux.HandleFunc("/health", hs.handleHealth)
	hs.mux.HandleFunc("/healthz", hs.handleHealth)
	hs.server = &http.Server{
		Handler: hs.mux,
	}
	return hs
}

// Addr returns the listener address (non-nil only after Start).
func (hs *HealthServer) Addr() string {
	if hs.listener != nil {
		return hs.listener.Addr().String()
	}
	return ""
}

// Start begins serving the health endpoint on the given address.
func (hs *HealthServer) Start(addr string) error {
	hs.server.Addr = addr
	l, err := hs.listen(addr)
	if err != nil {
		return err
	}
	hs.listener = l
	go hs.server.Serve(l)
	return nil
}

// listen creates a TCP listener on the given address.
func (hs *HealthServer) listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// Stop shuts down the health server.
func (hs *HealthServer) Stop() {
	if hs.server != nil {
		hs.server.Close()
	}
}

// handleHealth serves the health check JSON response.
func (hs *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resp := HealthResponse{
		Status:    "ok",
		Uptime:    time.Since(hs.startTime).Truncate(time.Second).String(),
		Version:   "0.1.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if hs.manager != nil {
		statuses := hs.manager.GetAllStatuses()
		resp.Accounts = make([]AccountHealth, len(statuses))
		for i, s := range statuses {
			resp.Accounts[i] = AccountHealth{
				ID:        s.ID,
				Connected: s.Connected,
				Error:     s.Error,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
