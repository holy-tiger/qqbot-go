package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tokenResponse mirrors the QQ Bot token API response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func newTokenTestServer(token string, expiresIn int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			AppID        string `json:"appId"`
			ClientSecret string `json:"clientSecret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := tokenResponse{AccessToken: token, ExpiresIn: expiresIn}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestTokenCache_New(t *testing.T) {
	tc := NewTokenCache()
	if tc == nil {
		t.Fatal("expected non-nil TokenCache")
	}
}

func TestTokenCache_GetAccessToken(t *testing.T) {
	server := newTokenTestServer("test-token-123", 7200)
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	token, err := tc.GetAccessToken(context.Background(), "app1", "secret1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Fatalf("expected test-token-123, got %q", token)
	}
}

func TestTokenCache_CacheHit(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := tokenResponse{AccessToken: "cached-token", ExpiresIn: 7200}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	// First call - should hit server
	token1, err := tc.GetAccessToken(context.Background(), "app1", "secret1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 != "cached-token" {
		t.Fatalf("expected cached-token, got %q", token1)
	}

	// Second call - should use cache
	token2, err := tc.GetAccessToken(context.Background(), "app1", "secret1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token2 != "cached-token" {
		t.Fatalf("expected cached-token, got %q", token2)
	}

	if callCount != 1 {
		t.Fatalf("expected 1 server call (cache hit), got %d", callCount)
	}
}

func TestTokenCache_SingleflightDedup(t *testing.T) {
	var callCount atomic.Int32
	var unblock chan struct{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Block until signaled to simulate slow server
		<-unblock
		resp := tokenResponse{AccessToken: "singleflight-token", ExpiresIn: 7200}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	unblock = make(chan struct{})
	tc := NewTokenCache()
	tc.tokenURL = server.URL

	var wg sync.WaitGroup
	results := make([]string, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, err := tc.GetAccessToken(context.Background(), "app1", "secret1")
			if err != nil {
				t.Errorf("unexpected error at %d: %v", idx, err)
				return
			}
			results[idx] = token
		}(i)
	}

	// Let goroutines queue up
	time.Sleep(50 * time.Millisecond)
	// Unblock the server
	close(unblock)

	wg.Wait()

	// All should get the same token
	for i, r := range results {
		if r != "singleflight-token" {
			t.Fatalf("goroutine %d: expected singleflight-token, got %q", i, r)
		}
	}
	// Only one actual HTTP call should have been made
	if got := callCount.Load(); got != 1 {
		t.Fatalf("expected 1 server call (singleflight), got %d", got)
	}
}

func TestTokenCache_Clear(t *testing.T) {
	server := newTokenTestServer("token-a", 7200)
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	token, err := tc.GetAccessToken(context.Background(), "app1", "secret1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "token-a" {
		t.Fatalf("expected token-a, got %q", token)
	}

	// Clear cache for app1
	tc.Clear("app1")

	// After clear, status should be none
	status := tc.GetStatus("app1")
	if status.Status != "none" {
		t.Fatalf("expected none status after clear, got %q", status.Status)
	}
}

func TestTokenCache_ClearAll(t *testing.T) {
	server := newTokenTestServer("token-b", 7200)
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	tc.GetAccessToken(context.Background(), "app1", "secret1")
	tc.GetAccessToken(context.Background(), "app2", "secret2")

	tc.ClearAll()

	if s := tc.GetStatus("app1"); s.Status != "none" {
		t.Fatalf("expected none for app1, got %q", s.Status)
	}
	if s := tc.GetStatus("app2"); s.Status != "none" {
		t.Fatalf("expected none for app2, got %q", s.Status)
	}
}

func TestTokenCache_GetStatus(t *testing.T) {
	server := newTokenTestServer("token-c", 7200)
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	// No cache
	status := tc.GetStatus("app1")
	if status.Status != "none" {
		t.Fatalf("expected none, got %q", status.Status)
	}

	// After fetching
	tc.GetAccessToken(context.Background(), "app1", "secret1")
	status = tc.GetStatus("app1")
	if status.Status != "valid" {
		t.Fatalf("expected valid, got %q", status.Status)
	}
}

func TestTokenCache_BackgroundRefresh(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		resp := tokenResponse{AccessToken: "refresh-token", ExpiresIn: 7200}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tc.StartBackgroundRefresh(ctx, "app1", "secret1")

	// Wait for initial fetch
	time.Sleep(200 * time.Millisecond)

	if got := callCount.Load(); got < 1 {
		t.Fatalf("expected at least 1 token fetch from background refresh, got %d", got)
	}

	status := tc.GetStatus("app1")
	if status.Status != "valid" {
		t.Fatalf("expected valid after background refresh, got %q", status.Status)
	}
}

func TestTokenCache_BackgroundRefreshStop(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		resp := tokenResponse{AccessToken: "refresh-token", ExpiresIn: 1}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tc := NewTokenCache()
	tc.tokenURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())

	tc.StartBackgroundRefresh(ctx, "app1", "secret1")
	time.Sleep(200 * time.Millisecond)

	countBefore := callCount.Load()
	cancel()
	time.Sleep(200 * time.Millisecond)

	// After cancel, count should not increase significantly
	// (background goroutine should have stopped)
	if got := callCount.Load(); got > int32(countBefore)+1 {
		t.Fatalf("background refresh did not stop promptly: before=%d after=%d", countBefore, got)
	}
}
