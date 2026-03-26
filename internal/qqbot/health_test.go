package qqbot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/types"
)

func TestHealthResponse_Basic(t *testing.T) {
	mgr, _ := NewBotManager(t.TempDir())
	mgr.AddAccount(types.ResolvedQQBotAccount{
		AccountID:    "test-acct",
		AppID:        "app-123",
		ClientSecret: "secret",
		Enabled:      true,
	})

	hs := NewHealthServer(mgr, "0.1.0")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	hs.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content type, got %s", ct)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", resp.Status)
	}
	if resp.Version != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %s", resp.Version)
	}
	if resp.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}

	if len(resp.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(resp.Accounts))
	}
	if resp.Accounts[0].ID != "test-acct" {
		t.Errorf("expected account ID 'test-acct', got %s", resp.Accounts[0].ID)
	}
	if resp.Accounts[0].Connected {
		t.Error("account should not be connected")
	}
}

func TestHealthResponse_NoAccounts(t *testing.T) {
	mgr, _ := NewBotManager(t.TempDir())
	hs := NewHealthServer(mgr, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	hs.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HealthResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
	if len(resp.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(resp.Accounts))
	}
}

func TestHealthResponse_NilManager(t *testing.T) {
	hs := NewHealthServer(nil, "0.1.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	hs.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HealthResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Status != "ok" {
		t.Errorf("expected ok, got %s", resp.Status)
	}
}

func TestHealthResponse_Uptime(t *testing.T) {
	mgr, _ := NewBotManager(t.TempDir())
	hs := NewHealthServer(mgr, "0.1.0")
	hs.startTime = time.Now().Add(-5 * time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	hs.mux.ServeHTTP(rec, req)

	var resp HealthResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestHealthResponse_MethodNotAllowed(t *testing.T) {
	mgr, _ := NewBotManager(t.TempDir())
	hs := NewHealthServer(mgr, "0.1.0")

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()
	hs.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
