package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
)

// mockBotAPI implements BotAPI for testing.
type mockBotAPI struct {
	accountID    string
	cancelFound  bool
	onListUsers  func(accountID string, opts store.ListOptions) []store.KnownUser
}

func (m *mockBotAPI) GetAccount(id string) interface{ GetID() string; IsConnected() bool } {
	if id != m.accountID {
		return nil
	}
	return &mockAcctInfo{id: id}
}

type mockAcctInfo struct {
	id string
}

func (a *mockAcctInfo) GetID() string       { return a.id }
func (a *mockAcctInfo) IsConnected() bool   { return false }

func (m *mockBotAPI) GetAllStatuses() []interface{ GetID() string; IsConnected() bool } {
	return []interface{ GetID() string; IsConnected() bool }{&mockAcctInfo{id: m.accountID}}
}

func (m *mockBotAPI) SendC2C(ctx context.Context, accountID, openid, content, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendGroup(ctx context.Context, accountID, groupOpenID, content, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendChannel(ctx context.Context, accountID, channelID, content, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendImage(ctx context.Context, accountID, targetType, targetID, imageURL, content, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendVoice(ctx context.Context, accountID, targetType, targetID, voiceBase64, ttsText, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendVideo(ctx context.Context, accountID, targetType, targetID, videoURL, videoBase64, content, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendFile(ctx context.Context, accountID, targetType, targetID, fileBase64, fileURL, fileName, msgID string) error {
	return nil
}

func (m *mockBotAPI) SendProactiveC2C(ctx context.Context, accountID, openid, content string) error {
	return nil
}

func (m *mockBotAPI) SendProactiveGroup(ctx context.Context, accountID, groupOpenID, content string) error {
	return nil
}

func (m *mockBotAPI) Broadcast(ctx context.Context, accountID, content string) (int, []error) {
	return 0, nil
}

func (m *mockBotAPI) BroadcastToGroups(ctx context.Context, accountID, content string) (int, []error) {
	return 0, nil
}

func (m *mockBotAPI) ListUsers(accountID string, opts store.ListOptions) []store.KnownUser {
	if m.onListUsers != nil {
		return m.onListUsers(accountID, opts)
	}
	return nil
}

func (m *mockBotAPI) GetUserStats(accountID string) store.UserStats {
	return store.UserStats{}
}

func (m *mockBotAPI) ClearUsers(accountID string) int {
	return 0
}

func (m *mockBotAPI) AddReminder(job proactive.ReminderJob) (string, error) {
	return "rem-1", nil
}

func (m *mockBotAPI) CancelReminder(accountID, jobID string) bool {
	return m.cancelFound
}

func (m *mockBotAPI) GetReminders(accountID string) []proactive.ReminderJob {
	return nil
}

func TestAPI_ListAccounts(t *testing.T) {
	mock := &mockBotAPI{accountID: "test"}
	srv := NewAPIServer(mock, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := testJSON(t, w.Body.String())
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
}

func TestAPI_GetAccount(t *testing.T) {
	mock := &mockBotAPI{accountID: "test"}
	srv := NewAPIServer(mock, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/test", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := testJSON(t, w.Body.String())
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}
	data := resp["data"].(map[string]interface{})
	if data["id"] != "test" {
		t.Errorf("expected id=test, got %v", data["id"])
	}
}

func TestAPI_GetAccount_NotFound(t *testing.T) {
	mock := &mockBotAPI{accountID: "test"}
	srv := NewAPIServer(mock, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	resp := testJSON(t, w.Body.String())
	if resp["ok"] != false {
		t.Errorf("expected ok=false, got %v", resp["ok"])
	}
}
