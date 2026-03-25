package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPI_ListUsers(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/test/users", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPI_UserStats(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/test/users/stats", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := testJSON(t, w.Body.String())
	if resp["ok"] != true {
		t.Errorf("expected ok=true")
	}
}

func TestAPI_ClearUsers(t *testing.T) {
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/test/users", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
