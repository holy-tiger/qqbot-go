package qqbot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/config"
	"github.com/openclaw/qqbot/internal/outbound"
	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

// ---------------------------------------------------------------------------
// mockMessageSender implements outbound.MessageSender
// ---------------------------------------------------------------------------

type mockMessageSender struct {
	mu sync.Mutex

	c2cCalls            []mockC2CCall
	groupCalls          []mockGroupCall
	channelCalls        []mockChannelCall
	proactiveC2CCalls   []mockProactiveC2CCall
	proactiveGroupCalls []mockProactiveGroupCall
	imageCalls          []mockImageCall
	groupImageCalls     []mockGroupImageCall
	voiceCalls          []mockVoiceCall
	groupVoiceCalls     []mockGroupVoiceCall
	videoCalls          []mockVideoCall
	groupVideoCalls     []mockGroupVideoCall
	fileCalls           []mockFileCall
	groupFileCalls      []mockGroupFileCall
	err                 error
}

type mockC2CCall struct{ openid, content, msgID string }
type mockGroupCall struct{ groupOpenID, content, msgID string }
type mockChannelCall struct{ channelID, content, msgID string }
type mockProactiveC2CCall struct{ openid, content string }
type mockProactiveGroupCall struct{ groupOpenID, content string }
type mockImageCall struct{ openid, imageURL, msgID string }
type mockGroupImageCall struct{ groupOpenID, imageURL, msgID string }
type mockVoiceCall struct{ openid, voiceBase64, msgID string }
type mockGroupVoiceCall struct{ groupOpenID, voiceBase64, msgID string }
type mockVideoCall struct{ openid, videoURL, videoBase64, msgID string }
type mockGroupVideoCall struct{ groupOpenID, videoURL, videoBase64, msgID string }
type mockFileCall struct{ openid, fileBase64, fileURL, msgID, fileName string }
type mockGroupFileCall struct{ groupOpenID, fileBase64, fileURL, msgID, fileName string }

func newMockMessageSender() *mockMessageSender {
	return &mockMessageSender{}
}

func (m *mockMessageSender) SendC2CMessage(_ context.Context, openid, content, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.c2cCalls = append(m.c2cCalls, mockC2CCall{openid, content, msgID})
	return types.MessageResponse{ID: "resp-" + msgID}, m.err
}

func (m *mockMessageSender) SendGroupMessage(_ context.Context, groupOpenID, content, msgID string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupCalls = append(m.groupCalls, mockGroupCall{groupOpenID, content, msgID})
	return types.MessageResponse{ID: "resp-" + msgID}, m.err
}

func (m *mockMessageSender) SendChannelMessage(_ context.Context, channelID, content, msgID string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channelCalls = append(m.channelCalls, mockChannelCall{channelID, content, msgID})
	return types.MessageResponse{ID: "resp-" + msgID}, m.err
}

func (m *mockMessageSender) SendProactiveC2CMessage(_ context.Context, openid, content string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proactiveC2CCalls = append(m.proactiveC2CCalls, mockProactiveC2CCall{openid, content})
	return types.MessageResponse{ID: "resp-pro-c2c-" + openid}, m.err
}

func (m *mockMessageSender) SendProactiveGroupMessage(_ context.Context, groupOpenID, content string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.proactiveGroupCalls = append(m.proactiveGroupCalls, mockProactiveGroupCall{groupOpenID, content})
	return types.MessageResponse{ID: "resp-pro-grp-" + groupOpenID}, m.err
}

func (m *mockMessageSender) SendC2CImageMessage(_ context.Context, openid, imageURL, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.imageCalls = append(m.imageCalls, mockImageCall{openid, imageURL, msgID})
	return types.MessageResponse{ID: "resp-img-" + msgID}, m.err
}

func (m *mockMessageSender) SendGroupImageMessage(_ context.Context, groupOpenID, imageURL, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupImageCalls = append(m.groupImageCalls, mockGroupImageCall{groupOpenID, imageURL, msgID})
	return types.MessageResponse{ID: "resp-grp-img-" + msgID}, m.err
}

func (m *mockMessageSender) SendC2CVoiceMessage(_ context.Context, openid, voiceBase64, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voiceCalls = append(m.voiceCalls, mockVoiceCall{openid, voiceBase64, msgID})
	return types.MessageResponse{ID: "resp-voice-" + msgID}, m.err
}

func (m *mockMessageSender) SendGroupVoiceMessage(_ context.Context, groupOpenID, voiceBase64, msgID string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupVoiceCalls = append(m.groupVoiceCalls, mockGroupVoiceCall{groupOpenID, voiceBase64, msgID})
	return types.MessageResponse{ID: "resp-grp-voice-" + msgID}, m.err
}

func (m *mockMessageSender) SendC2CVideoMessage(_ context.Context, openid string, videoURL, videoBase64, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.videoCalls = append(m.videoCalls, mockVideoCall{openid, videoURL, videoBase64, msgID})
	return types.MessageResponse{ID: "resp-video-" + msgID}, m.err
}

func (m *mockMessageSender) SendGroupVideoMessage(_ context.Context, groupOpenID string, videoURL, videoBase64, msgID, _ string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupVideoCalls = append(m.groupVideoCalls, mockGroupVideoCall{groupOpenID, videoURL, videoBase64, msgID})
	return types.MessageResponse{ID: "resp-grp-video-" + msgID}, m.err
}

func (m *mockMessageSender) SendC2CFileMessage(_ context.Context, openid string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileCalls = append(m.fileCalls, mockFileCall{openid, fileBase64, fileURL, msgID, fileName})
	return types.MessageResponse{ID: "resp-file-" + msgID}, m.err
}

func (m *mockMessageSender) SendGroupFileMessage(_ context.Context, groupOpenID string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupFileCalls = append(m.groupFileCalls, mockGroupFileCall{groupOpenID, fileBase64, fileURL, msgID, fileName})
	return types.MessageResponse{ID: "resp-grp-file-" + msgID}, m.err
}

// ===========================================================================
// Section 1: Full Outbound Message Flow
// ===========================================================================

func TestIntegration_OutboundC2CFlow(t *testing.T) {
	mock := newMockMessageSender()
	handler := outbound.NewOutboundHandler(mock, "")

	ctx := context.Background()
	target := outbound.Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendText(ctx, target, "hello world", "msg001")
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.c2cCalls) != 1 {
		t.Fatalf("expected 1 C2C call, got %d", len(mock.c2cCalls))
	}
	if mock.c2cCalls[0].openid != "user123" {
		t.Errorf("expected openid user123, got %s", mock.c2cCalls[0].openid)
	}
	if mock.c2cCalls[0].content != "hello world" {
		t.Errorf("expected content 'hello world', got %s", mock.c2cCalls[0].content)
	}
	if mock.c2cCalls[0].msgID != "msg001" {
		t.Errorf("expected msgID msg001, got %s", mock.c2cCalls[0].msgID)
	}
}

func TestIntegration_OutboundGroupFlow(t *testing.T) {
	mock := newMockMessageSender()
	handler := outbound.NewOutboundHandler(mock, "")

	ctx := context.Background()
	target := outbound.Target{Type: "group", OpenID: "group456"}
	err := handler.SendText(ctx, target, "group hello", "msg002")
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.groupCalls) != 1 {
		t.Fatalf("expected 1 group call, got %d", len(mock.groupCalls))
	}
	if mock.groupCalls[0].groupOpenID != "group456" {
		t.Errorf("expected groupOpenID group456, got %s", mock.groupCalls[0].groupOpenID)
	}
	if mock.groupCalls[0].content != "group hello" {
		t.Errorf("expected content 'group hello', got %s", mock.groupCalls[0].content)
	}
	if mock.groupCalls[0].msgID != "msg002" {
		t.Errorf("expected msgID msg002, got %s", mock.groupCalls[0].msgID)
	}
}

func TestIntegration_RateLimitFallback(t *testing.T) {
	mock := newMockMessageSender()
	handler := outbound.NewOutboundHandler(mock, "")
	ctx := context.Background()
	target := outbound.Target{Type: "c2c", OpenID: "user789"}
	msgID := "rate-msg-01"

	// Send 4 passive replies (the limit)
	for i := 0; i < 4; i++ {
		err := handler.SendText(ctx, target, "reply", msgID)
		if err != nil {
			t.Fatalf("reply %d failed: %v", i, err)
		}
	}

	// The 5th send should fall back to proactive (msgID gets cleared internally)
	err := handler.SendText(ctx, target, "fallback reply", msgID)
	if err != nil {
		t.Fatalf("fallback send failed: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.c2cCalls) != 4 {
		t.Errorf("expected 4 passive C2C calls, got %d", len(mock.c2cCalls))
	}
	if len(mock.proactiveC2CCalls) != 1 {
		t.Errorf("expected 1 proactive C2C call, got %d", len(mock.proactiveC2CCalls))
	} else {
		if mock.proactiveC2CCalls[0].openid != "user789" {
			t.Errorf("expected proactive to user789, got %s", mock.proactiveC2CCalls[0].openid)
		}
		if mock.proactiveC2CCalls[0].content != "fallback reply" {
			t.Errorf("expected proactive content 'fallback reply', got %s", mock.proactiveC2CCalls[0].content)
		}
	}
}

func TestIntegration_OutboundMediaTags(t *testing.T) {
	mock := newMockMessageSender()
	handler := outbound.NewOutboundHandler(mock, "")

	ctx := context.Background()
	target := outbound.Target{Type: "c2c", OpenID: "user-media"}
	text := "check this image:<qqimg>https://img.example.com/photo.jpg</qqimg>"
	err := handler.SendText(ctx, target, text, "msg-media-1")
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	// Should have a text part and an image part
	if len(mock.imageCalls) != 1 {
		t.Fatalf("expected 1 image call, got %d", len(mock.imageCalls))
	}
	if mock.imageCalls[0].imageURL != "https://img.example.com/photo.jpg" {
		t.Errorf("expected image URL https://img.example.com/photo.jpg, got %s", mock.imageCalls[0].imageURL)
	}
	if mock.imageCalls[0].openid != "user-media" {
		t.Errorf("expected openid user-media, got %s", mock.imageCalls[0].openid)
	}
	// The text before the tag should also be sent
	textSent := false
	for _, c := range mock.c2cCalls {
		if strings.Contains(c.content, "check this image") {
			textSent = true
			break
		}
	}
	if !textSent {
		t.Error("expected a C2C text call with text before the image tag")
	}
}

func TestIntegration_OutboundMixedContent(t *testing.T) {
	mock := newMockMessageSender()
	handler := outbound.NewOutboundHandler(mock, "")

	ctx := context.Background()
	target := outbound.Target{Type: "c2c", OpenID: "user-mix"}
	text := "hello<qqimg>https://img.example.com/a.png</qqimg><qqvoice>data:audio/wav;base64,UklGRiQAAABXQVZFZm10</qqvoice>world"
	err := handler.SendText(ctx, target, text, "msg-mix-1")
	if err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.imageCalls) != 1 {
		t.Fatalf("expected 1 image call, got %d", len(mock.imageCalls))
	}
	if len(mock.voiceCalls) != 1 {
		t.Fatalf("expected 1 voice call, got %d", len(mock.voiceCalls))
	}
	// Verify text parts were sent for "hello" and "world"
	textParts := 0
	for _, c := range mock.c2cCalls {
		if c.content == "hello" || c.content == "world" {
			textParts++
		}
	}
	if textParts < 2 {
		t.Errorf("expected at least 2 text part calls (hello, world), got %d", textParts)
	}
}

// ===========================================================================
// Section 2: Proactive Manager with Stores
// ===========================================================================

func TestIntegration_ProactiveWithKnownUsers(t *testing.T) {
	userStore := store.NewKnownUsersStore(store.OpenTestDB(t))
	mock := newMockMessageSender()

	// Record 3 known C2C users
	userStore.Record(store.KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1"})
	userStore.Record(store.KnownUser{OpenID: "u2", Type: "c2c", AccountID: "acct1"})
	userStore.Record(store.KnownUser{OpenID: "u3", Type: "c2c", AccountID: "acct1"})
	// Record a group user (should not be broadcast to)
	userStore.Record(store.KnownUser{OpenID: "u4", Type: "group", GroupOpenID: "grp1", AccountID: "acct1"})

	mgr := proactive.NewProactiveManager(mock, userStore)

	ctx := context.Background()
	sent, errs := mgr.Broadcast(ctx, "acct1", "broadcast msg")
	if len(errs) > 0 {
		t.Fatalf("broadcast had errors: %v", errs)
	}
	if sent != 3 {
		t.Errorf("expected 3 messages sent, got %d", sent)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.proactiveC2CCalls) != 3 {
		t.Fatalf("expected 3 proactive C2C calls, got %d", len(mock.proactiveC2CCalls))
	}
	seen := map[string]bool{}
	for _, c := range mock.proactiveC2CCalls {
		seen[c.openid] = true
		if c.content != "broadcast msg" {
			t.Errorf("expected broadcast msg content, got %s", c.content)
		}
	}
	for _, id := range []string{"u1", "u2", "u3"} {
		if !seen[id] {
			t.Errorf("expected broadcast to user %s", id)
		}
	}
}

func TestIntegration_ProactiveUnknownUser(t *testing.T) {
	userStore := store.NewKnownUsersStore(store.OpenTestDB(t))
	mock := newMockMessageSender()

	mgr := proactive.NewProactiveManager(mock, userStore)

	ctx := context.Background()
	err := mgr.SendToUser(ctx, "acct1", "unknown-user", "hello")
	if err == nil {
		t.Fatal("expected error for unknown user, got nil")
	}
	if !strings.Contains(err.Error(), "unknown user") {
		t.Errorf("expected 'unknown user' in error, got: %v", err)
	}
}

func TestIntegration_ProactiveGroupBroadcast(t *testing.T) {
	userStore := store.NewKnownUsersStore(store.OpenTestDB(t))
	mock := newMockMessageSender()

	// Record 3 users in the same group (should deduplicate to 1 group message)
	userStore.Record(store.KnownUser{OpenID: "gu1", Type: "group", GroupOpenID: "grp-A", AccountID: "acct2"})
	userStore.Record(store.KnownUser{OpenID: "gu2", Type: "group", GroupOpenID: "grp-A", AccountID: "acct2"})
	userStore.Record(store.KnownUser{OpenID: "gu3", Type: "group", GroupOpenID: "grp-A", AccountID: "acct2"})
	// Another group
	userStore.Record(store.KnownUser{OpenID: "gu4", Type: "group", GroupOpenID: "grp-B", AccountID: "acct2"})
	// A user with empty groupOpenID (should be skipped)
	userStore.Record(store.KnownUser{OpenID: "gu5", Type: "group", GroupOpenID: "", AccountID: "acct2"})

	mgr := proactive.NewProactiveManager(mock, userStore)

	ctx := context.Background()
	sent, errs := mgr.BroadcastToGroup(ctx, "acct2", "group broadcast")
	if len(errs) > 0 {
		t.Fatalf("broadcast had errors: %v", errs)
	}
	if sent != 2 {
		t.Errorf("expected 2 unique group messages (grp-A dedup + grp-B), got %d", sent)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.proactiveGroupCalls) != 2 {
		t.Fatalf("expected 2 proactive group calls, got %d", len(mock.proactiveGroupCalls))
	}
}

func TestIntegration_SchedulerReminder(t *testing.T) {
	userStore := store.NewKnownUsersStore(store.OpenTestDB(t))
	mock := newMockMessageSender()

	mgr := proactive.NewProactiveManager(mock, userStore)
	sched := proactive.NewScheduler(mgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)

	// Add a reminder with NextRun in the past
	sched.AddReminder(proactive.ReminderJob{
		ID:            "rem-past",
		Content:       "wake up!",
		TargetType:    "c2c",
		TargetAddress: "user-sched",
		AccountID:     "acct-sched",
		Schedule:      "", // no recurrence
		NextRun:       time.Now().Add(-1 * time.Second),
	})

	// Wait for scheduler to fire (check interval is 100ms)
	time.Sleep(500 * time.Millisecond)

	sched.Stop()

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.proactiveC2CCalls) != 1 {
		t.Fatalf("expected 1 proactive C2C call from scheduler, got %d", len(mock.proactiveC2CCalls))
	}
	if mock.proactiveC2CCalls[0].openid != "user-sched" {
		t.Errorf("expected openid user-sched, got %s", mock.proactiveC2CCalls[0].openid)
	}
	if mock.proactiveC2CCalls[0].content != "wake up!" {
		t.Errorf("expected content 'wake up!', got %s", mock.proactiveC2CCalls[0].content)
	}
}

// ===========================================================================
// Section 3: Store Persistence
// ===========================================================================

func TestIntegration_StorePersistence_KnownUsers(t *testing.T) {
	dir := t.TempDir()

	// Phase 1: write users
	db1, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1 := store.NewKnownUsersStore(db1)
	s1.Record(store.KnownUser{OpenID: "persist-u1", Type: "c2c", AccountID: "acct-p", Nickname: "Alice"})
	s1.Record(store.KnownUser{OpenID: "persist-u2", Type: "c2c", AccountID: "acct-p", Nickname: "Bob"})
	db1.Close()

	// Phase 2: close and reopen
	db2, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := store.NewKnownUsersStore(db2)
	u1 := s2.Get("acct-p", "persist-u1", "c2c", "")
	if u1 == nil {
		t.Fatal("user persist-u1 not found after reopen")
	}
	if u1.Nickname != "Alice" {
		t.Errorf("expected nickname Alice, got %s", u1.Nickname)
	}
	u2 := s2.Get("acct-p", "persist-u2", "c2c", "")
	if u2 == nil {
		t.Fatal("user persist-u2 not found after reopen")
	}
	if u2.Nickname != "Bob" {
		t.Errorf("expected nickname Bob, got %s", u2.Nickname)
	}
}

func TestIntegration_StorePersistence_RefIndex(t *testing.T) {
	dir := t.TempDir()

	// Phase 1: write entries
	db1, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1 := store.NewRefIndexStore(db1)
	s1.Set("ref-1", store.RefIndexEntry{
		Content:   "hello ref",
		SenderID:  "bot",
		Timestamp: time.Now().UnixMilli(),
	})
	s1.Set("ref-2", store.RefIndexEntry{
		Content:   "world ref",
		SenderID:  "user",
		Timestamp: time.Now().UnixMilli(),
		Attachments: []store.RefAttachmentSummary{
			{Type: "image", Filename: "test.png"},
		},
	})
	db1.Close()

	// Phase 2: close and reopen
	db2, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := store.NewRefIndexStore(db2)
	e1 := s2.Get("ref-1")
	if e1 == nil {
		t.Fatal("ref-1 not found after reopen")
	}
	if e1.Content != "hello ref" {
		t.Errorf("expected content 'hello ref', got %s", e1.Content)
	}
	e2 := s2.Get("ref-2")
	if e2 == nil {
		t.Fatal("ref-2 not found after reopen")
	}
	if len(e2.Attachments) != 1 || e2.Attachments[0].Type != "image" {
		t.Errorf("expected 1 image attachment, got %+v", e2.Attachments)
	}
}

func TestIntegration_StorePersistence_Session(t *testing.T) {
	dir := t.TempDir()

	// Phase 1: save session
	db1, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1 := store.NewSessionStore(db1)
	s1.Save(store.SessionState{
		SessionID:        "sess-123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 1,
		AccountID:        "acct-session",
		AppID:            "app-test",
	})
	db1.Close()

	// Phase 2: close and reopen, then load
	db2, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := store.NewSessionStore(db2)
	loaded := s2.Load("acct-session", "app-test")
	if loaded == nil {
		t.Fatal("session not found after reopen (note: session expires in 5 minutes)")
	}
	if loaded.SessionID != "sess-123" {
		t.Errorf("expected session_id sess-123, got %s", loaded.SessionID)
	}
	if loaded.LastSeq != 42 {
		t.Errorf("expected last_seq 42, got %d", loaded.LastSeq)
	}
	if loaded.IntentLevelIndex != 1 {
		t.Errorf("expected intent_level_index 1, got %d", loaded.IntentLevelIndex)
	}
}

// ===========================================================================
// Section 4: Config Loading and Account Resolution
// ===========================================================================

func TestIntegration_ConfigLoadMultiAccount(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfgContent := `qqbot:
  appId: "default-app-id"
  clientSecret: "default-secret"
  accounts:
    bot2:
      appId: "bot2-app-id"
      clientSecret: "bot2-secret"
    bot3:
      appId: "bot3-app-id"
      clientSecret: "bot3-secret"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	ids := config.ListAccountIDs(cfg)
	if len(ids) != 3 {
		t.Fatalf("expected 3 account IDs, got %d: %v", len(ids), ids)
	}

	defaultAcct := config.ResolveAccount(cfg, "default")
	if defaultAcct.AppID != "default-app-id" {
		t.Errorf("expected default AppID default-app-id, got %s", defaultAcct.AppID)
	}
	if defaultAcct.ClientSecret != "default-secret" {
		t.Errorf("expected default ClientSecret default-secret, got %s", defaultAcct.ClientSecret)
	}
	if defaultAcct.SecretSource != "config" {
		t.Errorf("expected secret source 'config', got %s", defaultAcct.SecretSource)
	}

	bot2 := config.ResolveAccount(cfg, "bot2")
	if bot2.AppID != "bot2-app-id" {
		t.Errorf("expected bot2 AppID bot2-app-id, got %s", bot2.AppID)
	}
	if bot2.ClientSecret != "bot2-secret" {
		t.Errorf("expected bot2 ClientSecret bot2-secret, got %s", bot2.ClientSecret)
	}

	bot3 := config.ResolveAccount(cfg, "bot3")
	if bot3.AppID != "bot3-app-id" {
		t.Errorf("expected bot3 AppID bot3-app-id, got %s", bot3.AppID)
	}
}

func TestIntegration_ConfigResolveWithEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config-env.yaml")
	cfgContent := `qqbot:
  appId: "env-test-app"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Set env vars
	t.Setenv("QQBOT_APP_ID", "env-override-app")
	t.Setenv("QQBOT_CLIENT_SECRET", "env-secret-value")

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	acct := config.ResolveAccount(cfg, "default")
	if acct.AppID != "env-test-app" {
		// AppID is set in config, so it should use config value, not env
		t.Errorf("expected AppID from config 'env-test-app', got %s", acct.AppID)
	}
	if acct.ClientSecret != "env-secret-value" {
		t.Errorf("expected ClientSecret from env 'env-secret-value', got %s", acct.ClientSecret)
	}
	if acct.SecretSource != "env" {
		t.Errorf("expected secret source 'env', got %s", acct.SecretSource)
	}

	// Test with empty config appId to verify env override
	cfg2Path := filepath.Join(dir, "config-no-appid.yaml")
	cfg2Content := `qqbot:
  name: "test"
`
	if err := os.WriteFile(cfg2Path, []byte(cfg2Content), 0644); err != nil {
		t.Fatalf("write config2: %v", err)
	}
	cfg2, err := config.LoadConfig(cfg2Path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	acct2 := config.ResolveAccount(cfg2, "default")
	if acct2.AppID != "env-override-app" {
		t.Errorf("expected AppID from env 'env-override-app', got %s", acct2.AppID)
	}
}
