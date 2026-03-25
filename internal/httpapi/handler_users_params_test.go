package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openclaw/qqbot/internal/store"
)

func TestAPI_ListUsers_QueryParams(t *testing.T) {
	var capturedOpts store.ListOptions
	mock := &mockBotAPI{
		accountID: "test",
		onListUsers: func(accountID string, opts store.ListOptions) []store.KnownUser {
			capturedOpts = opts
			return nil
		},
	}
	srv := NewAPIServer(mock, NewWebhookDispatcher())

	// Test with all query params
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/accounts/test/users?type=c2c&active_within=86400000&limit=10&sort_by=lastSeenAt&sort_order=asc", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if capturedOpts.Type != "c2c" {
		t.Errorf("expected type=c2c, got %q", capturedOpts.Type)
	}
	if capturedOpts.ActiveWithin != 86400000 {
		t.Errorf("expected active_within=86400000, got %d", capturedOpts.ActiveWithin)
	}
	if capturedOpts.Limit != 10 {
		t.Errorf("expected limit=10, got %d", capturedOpts.Limit)
	}
	if capturedOpts.SortBy != "lastSeenAt" {
		t.Errorf("expected sort_by=lastSeenAt, got %q", capturedOpts.SortBy)
	}
	if capturedOpts.SortOrder != "asc" {
		t.Errorf("expected sort_order=asc, got %q", capturedOpts.SortOrder)
	}
}
