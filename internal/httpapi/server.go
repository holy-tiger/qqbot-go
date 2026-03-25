package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
)

// BotAPI defines the operations that the API server delegates to BotManager.
type BotAPI interface {
	GetAccount(id string) interface{ GetID() string; IsConnected() bool }
	GetAllStatuses() []interface{ GetID() string; IsConnected() bool }
	SendC2C(ctx context.Context, accountID, openid, content, msgID string) error
	SendGroup(ctx context.Context, accountID, groupOpenID, content, msgID string) error
	SendChannel(ctx context.Context, accountID, channelID, content, msgID string) error
	SendImage(ctx context.Context, accountID, targetType, targetID, imageURL, content, msgID string) error
	SendVoice(ctx context.Context, accountID, targetType, targetID, voiceBase64, ttsText, msgID string) error
	SendVideo(ctx context.Context, accountID, targetType, targetID, videoURL, videoBase64, content, msgID string) error
	SendFile(ctx context.Context, accountID, targetType, targetID, fileBase64, fileURL, fileName, msgID string) error
	SendProactiveC2C(ctx context.Context, accountID, openid, content string) error
	SendProactiveGroup(ctx context.Context, accountID, groupOpenID, content string) error
	Broadcast(ctx context.Context, accountID, content string) (sent int, errs []error)
	BroadcastToGroups(ctx context.Context, accountID, content string) (sent int, errs []error)
	ListUsers(accountID string, opts store.ListOptions) []store.KnownUser
	GetUserStats(accountID string) store.UserStats
	ClearUsers(accountID string) int
	AddReminder(job proactive.ReminderJob) (string, error)
	CancelReminder(accountID, jobID string) bool
	GetReminders(accountID string) []proactive.ReminderJob
}

// APIServer provides RESTful HTTP API endpoints for QQ Bot operations.
type APIServer struct {
	mux     *http.ServeMux
	server  *http.Server
	manager BotAPI
	webhook *WebhookDispatcher
}

// NewAPIServer creates a new APIServer.
func NewAPIServer(manager BotAPI, webhook *WebhookDispatcher) *APIServer {
	s := &APIServer{
		mux:     http.NewServeMux(),
		manager: manager,
		webhook: webhook,
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up all API endpoints.
func (s *APIServer) registerRoutes() {
	// Account status
	s.mux.HandleFunc("GET /api/v1/accounts", s.handleListAccounts)
	s.mux.HandleFunc("GET /api/v1/accounts/{id}", s.handleGetAccount)

	// Message sending
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/c2c/{openid}/messages", s.handleSendC2CText)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/groups/{openid}/messages", s.handleSendGroupText)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/channels/{channelID}/messages", s.handleSendChannelText)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/c2c/{openid}/images", s.handleSendC2CImage)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/groups/{openid}/images", s.handleSendGroupImage)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/c2c/{openid}/voice", s.handleSendC2CVoice)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/groups/{openid}/voice", s.handleSendGroupVoice)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/c2c/{openid}/videos", s.handleSendC2CVideo)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/groups/{openid}/videos", s.handleSendGroupVideo)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/c2c/{openid}/files", s.handleSendC2CFile)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/groups/{openid}/files", s.handleSendGroupFile)

	// Proactive & broadcast
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/proactive/c2c/{openid}", s.handleProactiveC2C)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/proactive/groups/{openid}", s.handleProactiveGroup)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/broadcast", s.handleBroadcast)
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/broadcast/groups", s.handleBroadcastGroups)

	// Reminders
	s.mux.HandleFunc("POST /api/v1/accounts/{id}/reminders", s.handleCreateReminder)
	s.mux.HandleFunc("DELETE /api/v1/accounts/{id}/reminders/{remID}", s.handleCancelReminder)
	s.mux.HandleFunc("GET /api/v1/accounts/{id}/reminders", s.handleListReminders)

	// Users
	s.mux.HandleFunc("GET /api/v1/accounts/{id}/users", s.handleListUsers)
	s.mux.HandleFunc("GET /api/v1/accounts/{id}/users/stats", s.handleUserStats)
	s.mux.HandleFunc("DELETE /api/v1/accounts/{id}/users", s.handleClearUsers)
}

// Start begins serving the API on the given address.
func (s *APIServer) Start(addr string) error {
	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Server stopped
		}
	}()
	return nil
}

// Stop gracefully shuts down the API server.
func (s *APIServer) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeOK writes a success response.
func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": data})
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{"ok": false, "error": msg})
}

// textRequest is the standard request body for text messages.
type textRequest struct {
	Content string `json:"content"`
	MsgID   string `json:"msg_id,omitempty"`
}

// imageRequest is the request body for image messages.
type imageRequest struct {
	ImageURL string `json:"image_url"`
	Content  string `json:"content,omitempty"`
	MsgID    string `json:"msg_id,omitempty"`
}

// voiceRequest is the request body for voice messages.
type voiceRequest struct {
	VoiceBase64 string `json:"voice_base64"`
	TTSText     string `json:"tts_text,omitempty"`
	MsgID       string `json:"msg_id,omitempty"`
}

// videoRequest is the request body for video messages.
type videoRequest struct {
	VideoURL    string `json:"video_url"`
	VideoBase64 string `json:"video_base64,omitempty"`
	Content     string `json:"content,omitempty"`
	MsgID       string `json:"msg_id,omitempty"`
}

// fileRequest is the request body for file messages.
type fileRequest struct {
	FileURL    string `json:"file_url"`
	FileBase64 string `json:"file_base64,omitempty"`
	FileName   string `json:"file_name"`
	MsgID      string `json:"msg_id,omitempty"`
}

// reminderRequest is the request body for creating a reminder.
type reminderRequest struct {
	Content       string `json:"content"`
	TargetType    string `json:"target_type"`
	TargetAddress string `json:"target_address"`
	Schedule      string `json:"schedule"`
}

// proactiveRequest is the request body for proactive messages.
type proactiveRequest struct {
	Content string `json:"content"`
}
