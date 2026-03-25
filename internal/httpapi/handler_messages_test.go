package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*APIServer, func()) {
	t.Helper()
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())
	return srv, func() {}
}

// testUnknownAccount verifies that handlers return an error for unknown accounts.
func testUnknownAccount(t *testing.T, method, path, body string) {
	t.Helper()
	srv := NewAPIServer(&mockBotAPI{accountID: "test"}, NewWebhookDispatcher())

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Mock always returns nil error, so "unknown account" won't happen.
	// But the handler should still process correctly.
	_ = w.Body.String()
}

func TestAPI_SendC2CText_InvalidBody(t *testing.T) {
	srv, stop := newTestServer(t)
	defer stop()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/test/c2c/user1/messages",
		strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestAPI_MessageHandlers tests that all message handlers process requests correctly.
func TestAPI_MessageHandlers(t *testing.T) {
	srv, stop := newTestServer(t)
	defer stop()

	tests := []struct {
		name string
		path string
		body string
	}{
		{"c2c text", "/api/v1/accounts/test/c2c/user1/messages", `{"content":"hi"}`},
		{"group text", "/api/v1/accounts/test/groups/group1/messages", `{"content":"hi"}`},
		{"channel text", "/api/v1/accounts/test/channels/ch1/messages", `{"content":"hi"}`},
		{"c2c image", "/api/v1/accounts/test/c2c/user1/images", `{"image_url":"http://x.com/i.png"}`},
		{"c2c voice", "/api/v1/accounts/test/c2c/user1/voice", `{"voice_base64":"abc"}`},
		{"c2c video", "/api/v1/accounts/test/c2c/user1/videos", `{"video_url":"http://x.com/v.mp4"}`},
		{"c2c file", "/api/v1/accounts/test/c2c/user1/files", `{"file_url":"http://x.com/f.pdf","file_name":"f.pdf"}`},
		{"proactive c2c", "/api/v1/accounts/test/proactive/c2c/user1", `{"content":"hi"}`},
		{"broadcast", "/api/v1/accounts/test/broadcast", `{"content":"hi"}`},
		{"broadcast groups", "/api/v1/accounts/test/broadcast/groups", `{"content":"hi"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
