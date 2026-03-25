package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebhookDispatcher_Dispatch(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, _ := io.ReadAll(r.Body)
		var event WebhookEvent
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if event.EventType != "C2C_MESSAGE_CREATE" {
			t.Errorf("expected event type C2C_MESSAGE_CREATE, got %s", event.EventType)
		}
		if event.AccountID != "acct1" {
			t.Errorf("expected account_id acct1, got %s", event.AccountID)
		}
		if event.Data == nil {
			t.Error("expected non-nil data")
		}
		if event.Timestamp == "" {
			t.Error("expected non-empty timestamp")
		}
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher()
	d.SetURL("acct1", srv.URL)

	d.Dispatch("acct1", "C2C_MESSAGE_CREATE", json.RawMessage(`{"content":"hello"}`))

	// Wait for async delivery
	deadline := time.After(2 * time.Second)
	for received.Load() == 0 {
		select {
		case <-time.After(10 * time.Millisecond):
			// retry
		case <-deadline:
			t.Fatal("timed out waiting for webhook delivery")
		}
	}

	if received.Load() != 1 {
		t.Errorf("expected 1 delivery, got %d", received.Load())
	}
}

func TestWebhookDispatcher_NoURL(t *testing.T) {
	d := NewWebhookDispatcher()
	// Should not panic or block
	d.Dispatch("unknown", "SOME_EVENT", json.RawMessage(`{}`))
}

func TestWebhookDispatcher_ServerError(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher()
	d.SetURL("acct1", srv.URL)

	d.Dispatch("acct1", "TEST_EVENT", json.RawMessage(`{}`))

	// Wait for retries to complete
	time.Sleep(8 * time.Second) // 0 + 1s + 2s + 4s = ~7s

	if attempts.Load() != webhookMaxRetries {
		t.Errorf("expected %d attempts, got %d", webhookMaxRetries, attempts.Load())
	}
}

func TestWebhookDispatcher_AccountOverride(t *testing.T) {
	var hits map[string]int32 = map[string]int32{}

	mu := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var event WebhookEvent
		json.Unmarshal(body, &event)
		hits[event.AccountID]++
		mu <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher()
	d.SetURL("default", srv.URL)
	d.SetURL("special", "http://127.0.0.1:1/nonexistent") // will fail silently

	d.Dispatch("default", "EVENT", json.RawMessage(`{}`))
	<-mu

	d.Dispatch("special", "EVENT", json.RawMessage(`{}`))
	// Give time for the failed attempt
	time.Sleep(2 * time.Second)

	if hits["default"] != 1 {
		t.Errorf("expected default to get 1 hit, got %d", hits["default"])
	}
}

func TestWebhookEvent_JSONRoundTrip(t *testing.T) {
	event := WebhookEvent{
		AccountID: "acct1",
		EventType: "GROUP_AT_MESSAGE_CREATE",
		Timestamp: "2026-03-25T12:00:00Z",
		Data:      json.RawMessage(`{"content":"test"}`),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WebhookEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.AccountID != event.AccountID {
		t.Errorf("account_id mismatch")
	}
	if decoded.EventType != event.EventType {
		t.Errorf("event_type mismatch")
	}
	if string(decoded.Data) != string(event.Data) {
		t.Errorf("data mismatch")
	}
}
