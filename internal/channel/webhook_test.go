package channel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

// --- stripAtMention tests ---

func TestStripAtMention(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"mention format", "<@!123456> 你好", "你好"},
		{"mention no trailing content", "<@!123456>", ""},
		{"at nickname with space", "@Bot 你好", "你好"},
		{"at nickname with null byte", "@Bot\x00你好", "你好"},
		{"at nickname with null byte and space after", "@Bot\x00 你好", "你好"},
		{"plain text unchanged", "你好", "你好"},
		{"at nickname no separator before space", "@Bot你 好", "好"},
		{"empty string", "", ""},
		{"just at sign", "@", "@"},
		{"mention without closing bracket", "<@!123456 你好", "<@!123456 你好"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAtMention(tt.input)
			if got != tt.want {
				t.Errorf("stripAtMention(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- handleWebhook tests ---

func newTestChannelServer() *ChannelServer {
	return &ChannelServer{
		config: Config{Account: "default", QQBotAPI: "http://127.0.0.1:9090"},
	}
}

func TestHandleWebhook_MethodNotAllowed(t *testing.T) {
	cs := newTestChannelServer()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	cs := newTestChannelServer()
	req := httptest.NewRequest(http.MethodPost, "/webhook",
		bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleWebhook_IgnoresUnhandledEventTypes(t *testing.T) {
	cs := newTestChannelServer()
	event := webhookEvent{EventType: "GUILD_MEMBER_ADD", Data: json.RawMessage(`{}`)}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleWebhook_C2CMessage(t *testing.T) {
	cs := newTestChannelServer()

	var captured struct {
		source  string
		sender  string
		chatID  string
		content string
	}
	cs.pushNotification = func(source, sender, chatID, content string) {
		captured.source = source
		captured.sender = sender
		captured.chatID = chatID
		captured.content = content
	}

	msg := types.C2CMessageEvent{
		Content: "hello",
		Author:  types.C2CAuthor{UserOpenID: "o_user123"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "C2C_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if captured.source != "qq" {
		t.Errorf("expected source %q, got %q", "qq", captured.source)
	}
	if captured.sender != "o_user123" {
		t.Errorf("expected sender %q, got %q", "o_user123", captured.sender)
	}
	if captured.chatID != "c2c:o_user123" {
		t.Errorf("expected chatID %q, got %q", "c2c:o_user123", captured.chatID)
	}
	if captured.content != "hello" {
		t.Errorf("expected content %q, got %q", "hello", captured.content)
	}
}

func TestHandleWebhook_C2CMessageWithAttachments(t *testing.T) {
	cs := newTestChannelServer()

	var capturedContent string
	cs.pushNotification = func(_, _, _, content string) {
		capturedContent = content
	}

	msg := types.C2CMessageEvent{
		Content: "看看",
		Author:  types.C2CAuthor{UserOpenID: "o_u1"},
		Attachments: []types.MessageAttachment{
			{ContentType: "image", URL: "https://example.com/img.png"},
		},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "C2C_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	want := "看看\n[图片: https://example.com/img.png]"
	if capturedContent != want {
		t.Errorf("got %q, want %q", capturedContent, want)
	}
}

func TestHandleWebhook_GroupMessage(t *testing.T) {
	cs := newTestChannelServer()

	var captured struct {
		sender string
		chatID string
	}
	cs.pushNotification = func(_, sender, chatID, _ string) {
		captured.sender = sender
		captured.chatID = chatID
	}

	msg := types.GroupMessageEvent{
		Content:     "@Bot 帮我看一下",
		GroupOpenID: "o_group123",
		Author:      types.GroupAuthor{MemberOpenID: "o_member1"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "GROUP_AT_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if captured.sender != "o_member1" {
		t.Errorf("expected sender %q, got %q", "o_member1", captured.sender)
	}
	if captured.chatID != "group:o_group123" {
		t.Errorf("expected chatID %q, got %q", "group:o_group123", captured.chatID)
	}
}

func TestHandleWebhook_GroupMessage_MentionStripped(t *testing.T) {
	cs := newTestChannelServer()

	var capturedContent string
	cs.pushNotification = func(_, _, _, content string) {
		capturedContent = content
	}

	msg := types.GroupMessageEvent{
		Content: "<@!botid> 帮我看一下这个bug",
		Author:  types.GroupAuthor{MemberOpenID: "o_m1"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "GROUP_AT_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if capturedContent != "帮我看一下这个bug" {
		t.Errorf("got %q, want mention stripped", capturedContent)
	}
}

func TestHandleWebhook_GuildMessage(t *testing.T) {
	cs := newTestChannelServer()

	var captured struct {
		sender string
		chatID string
	}
	cs.pushNotification = func(_, sender, chatID, _ string) {
		captured.sender = sender
		captured.chatID = chatID
	}

	msg := types.GuildMessageEvent{
		Content:   "<@!botid> hello",
		ChannelID: "12345678",
		Author:    types.GuildAuthor{ID: "author_id"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "GUILD_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if captured.sender != "author_id" {
		t.Errorf("expected sender %q, got %q", "author_id", captured.sender)
	}
	if captured.chatID != "channel:12345678" {
		t.Errorf("expected chatID %q, got %q", "channel:12345678", captured.chatID)
	}
}

func TestHandleWebhook_DirectMessage(t *testing.T) {
	cs := newTestChannelServer()

	var capturedChatID string
	cs.pushNotification = func(_, _, chatID, _ string) {
		capturedChatID = chatID
	}

	msg := types.GuildMessageEvent{
		Content:   "hello",
		ChannelID: "87654321",
		Author:    types.GuildAuthor{ID: "author_id"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "DIRECT_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	if capturedChatID != "dm:87654321" {
		t.Errorf("expected chatID %q, got %q", "dm:87654321", capturedChatID)
	}
}

func TestHandleWebhook_ConcurrentRequests(t *testing.T) {
	cs := newTestChannelServer()

	var count atomic.Int64
	cs.pushNotification = func(_, _, _, _ string) {
		count.Add(1)
	}

	msg := types.C2CMessageEvent{
		Content: "hello",
		Author:  types.C2CAuthor{UserOpenID: "o_u"},
	}
	data, _ := json.Marshal(msg)
	event := webhookEvent{EventType: "C2C_MESSAGE_CREATE", Data: data}
	body, _ := json.Marshal(event)

	const concurrency = 50
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			w := httptest.NewRecorder()
			cs.handleWebhook(w, req)
		}()
	}
	wg.Wait()

	if got := count.Load(); got != concurrency {
		t.Errorf("expected %d notifications, got %d", concurrency, got)
	}
}

func TestHandleWebhook_InvalidEventPayload(t *testing.T) {
	cs := newTestChannelServer()

	var notified bool
	cs.pushNotification = func(_, _, _, _ string) {
		notified = true
	}

	// Valid webhook envelope but invalid event data (wrong type for content field)
	event := webhookEvent{
		EventType: "C2C_MESSAGE_CREATE",
		Data:      json.RawMessage(`{"content": 123, "author": {"user_openid": "x"}}`),
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	w := httptest.NewRecorder()
	cs.handleWebhook(w, req)

	// Should return 200 (we acknowledged the webhook) but not notify
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if notified {
		t.Error("should not notify on invalid event payload")
	}
}

func TestHandleWebhook_AllEventTypes(t *testing.T) {
	tests := []struct {
		eventType string
		chatID    string
	}{
		{"C2C_MESSAGE_CREATE", "c2c:o_user"},
		{"GROUP_AT_MESSAGE_CREATE", "group:o_group"},
		{"GUILD_MESSAGE_CREATE", "channel:chid"},
		{"DIRECT_MESSAGE_CREATE", "dm:chid"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			cs := newTestChannelServer()
			var capturedChatID string
			cs.pushNotification = func(_, _, chatID, _ string) {
				capturedChatID = chatID
			}

			var eventData []byte
			switch tt.eventType {
			case "C2C_MESSAGE_CREATE":
				eventData, _ = json.Marshal(types.C2CMessageEvent{
					Content: "hi", Author: types.C2CAuthor{UserOpenID: "o_user"},
				})
			case "GROUP_AT_MESSAGE_CREATE":
				eventData, _ = json.Marshal(types.GroupMessageEvent{
					Content: "@Bot hi", GroupOpenID: "o_group",
					Author: types.GroupAuthor{MemberOpenID: "o_m"},
				})
			case "GUILD_MESSAGE_CREATE", "DIRECT_MESSAGE_CREATE":
				eventData, _ = json.Marshal(types.GuildMessageEvent{
					Content: "<@!b> hi", ChannelID: "chid",
					Author: types.GuildAuthor{ID: "aid"},
				})
			}

			event := webhookEvent{EventType: tt.eventType, Data: eventData}
			body, _ := json.Marshal(event)
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			w := httptest.NewRecorder()
			cs.handleWebhook(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if capturedChatID != tt.chatID {
				t.Errorf("expected chatID %q, got %q", tt.chatID, capturedChatID)
			}
		})
	}
}
