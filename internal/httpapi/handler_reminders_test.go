package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPI_CreateReminder(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	body := `{"content":"remind me","target_type":"c2c","target_address":"user1","schedule":"@every 30m"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/test/reminders",
		strings.NewReader(body))
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
	if data["job_id"] == nil {
		t.Error("expected job_id in response")
	}
}

func TestAPI_CreateReminder_InvalidTarget(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	body := `{"content":"hi","target_type":"invalid","target_address":"user1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/test/reminders",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_CreateReminder_MissingAddress(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	body := `{"content":"hi","target_type":"c2c"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/test/reminders",
		strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_CancelReminder(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test", cancelFound: true}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/test/reminders/rem-1", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPI_CancelReminder_NotFound(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test", cancelFound: false}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/test/reminders/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAPI_ListReminders(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/test/reminders", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
