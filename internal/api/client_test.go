package api

import (
	"context"
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

func TestNewAPIClient_Defaults(t *testing.T) {
	client := NewAPIClient()
	if client == nil {
		t.Fatal("expected non-nil APIClient")
	}
	if client.markdownSupport != false {
		t.Fatal("expected markdown support to be false by default")
	}
	if client.onMessageSent != nil {
		t.Fatal("expected no message sent hook by default")
	}
	if client.tokenCache == nil {
		t.Fatal("expected token cache to be initialized")
	}
	if client.httpClient == nil {
		t.Fatal("expected http client to be initialized")
	}
}

func TestNewAPIClient_WithMarkdownSupport(t *testing.T) {
	client := NewAPIClient(WithMarkdownSupport(true))
	if !client.markdownSupport {
		t.Fatal("expected markdown support to be true")
	}
	client2 := NewAPIClient(WithMarkdownSupport(false))
	if client2.markdownSupport {
		t.Fatal("expected markdown support to be false")
	}
}

func TestNewAPIClient_WithMessageSentHook(t *testing.T) {
	called := false
	client := NewAPIClient(WithMessageSentHook(func(refIdx string, meta types.OutboundMeta) {
		called = true
	}))
	if client.onMessageSent == nil {
		t.Fatal("expected message sent hook to be set")
	}
	client.onMessageSent("test-ref", types.OutboundMeta{Text: "hello"})
	if !called {
		t.Fatal("expected hook to be called")
	}
}

func TestAPIClient_InitAndClose(t *testing.T) {
	client := NewAPIClient()
	ctx, cancel := context.WithCancel(context.Background())
	client.Init(ctx, "app1", "secret1")
	// Should not panic
	cancel()
	client.Close()
}

func TestAPIClient_Init_SetsCredentials(t *testing.T) {
	client := NewAPIClient()
	ctx := context.Background()
	client.Init(ctx, "test-app", "test-secret")
	if client.appID != "test-app" {
		t.Fatalf("expected appID test-app, got %q", client.appID)
	}
	if client.clientSecret != "test-secret" {
		t.Fatalf("expected clientSecret test-secret, got %q", client.clientSecret)
	}
	client.Close()
}

func TestGetNextMsgSeq(t *testing.T) {
	seq := getNextMsgSeq("msg1")
	if seq < 0 || seq > 65535 {
		t.Fatalf("expected seq in range 0-65535, got %d", seq)
	}
	// Multiple calls should return values in valid range
	for i := 0; i < 100; i++ {
		s := getNextMsgSeq("test")
		if s < 0 || s > 65535 {
			t.Fatalf("iteration %d: seq %d out of range", i, s)
		}
	}
}
