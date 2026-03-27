package channel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChannelServer_PushNotification_NilFunc(t *testing.T) {
	// pushNotification is a function field, if not set it's nil
	cs := &ChannelServer{}
	if cs.pushNotification != nil {
		t.Fatal("expected nil pushNotification on fresh ChannelServer")
	}
	// In the actual Run() flow, pushNotification is always set.
	// Here we verify the nil state before Run() is called.
}

func TestChannelServer_PushNotification_WithMock(t *testing.T) {
	var received struct {
		source  string
		sender  string
		chatID  string
		content string
	}
	cs := &ChannelServer{
		pushNotification: func(source, sender, chatID, content string) {
			received.source = source
			received.sender = sender
			received.chatID = chatID
			received.content = content
		},
	}

	cs.pushNotification("qq", "o_user", "c2c:o_user", "hello")

	if received.source != "qq" {
		t.Errorf("source = %q, want %q", received.source, "qq")
	}
	if received.sender != "o_user" {
		t.Errorf("sender = %q, want %q", received.sender, "o_user")
	}
	if received.chatID != "c2c:o_user" {
		t.Errorf("chatID = %q, want %q", received.chatID, "c2c:o_user")
	}
	if received.content != "hello" {
		t.Errorf("content = %q, want %q", received.content, "hello")
	}
}

func TestWebhookServer_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		srv := &http.Server{Addr: "127.0.0.1:0", Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	err := <-errCh
	if err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

func TestWebhookServer_HealthEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"ok":true}` {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{}
	if cfg.WebhookPort != 0 {
		t.Errorf("default WebhookPort = %d, want 0", cfg.WebhookPort)
	}
	if cfg.QQBotAPI != "" {
		t.Errorf("default QQBotAPI = %q, want empty", cfg.QQBotAPI)
	}
	if cfg.Account != "" {
		t.Errorf("default Account = %q, want empty", cfg.Account)
	}
}
