package qqbot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

func makeResolvedAccount(accountID, appID string) types.ResolvedQQBotAccount {
	return types.ResolvedQQBotAccount{
		AccountID:    accountID,
		AppID:        appID,
		ClientSecret: "test-secret",
		Enabled:      true,
	}
}

func TestNewBotManager(t *testing.T) {
	dir := t.TempDir()
	m, err := NewBotManager(dir)
	if err != nil {
		t.Fatalf("NewBotManager failed: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil BotManager")
	}
	if m.dataDir != dir {
		t.Errorf("expected dataDir %q, got %q", dir, m.dataDir)
	}
	if m.started {
		t.Error("expected started to be false")
	}
	if len(m.GetAllStatuses()) != 0 {
		t.Error("expected no statuses for new manager")
	}
}

func TestAddAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acct := m.GetAccount("acct1")
	if acct == nil {
		t.Fatal("expected account to exist")
	}
	if acct.ID != "acct1" {
		t.Errorf("expected ID %q, got %q", "acct1", acct.ID)
	}
	if acct.Config.AppID != "app123" {
		t.Errorf("expected AppID %q, got %q", "app123", acct.Config.AppID)
	}
	if acct.DataDir != dir {
		t.Errorf("expected DataDir %q, got %q", dir, acct.DataDir)
	}
	if acct.Client == nil {
		t.Error("expected Client to be initialized")
	}
	if acct.Gateway == nil {
		t.Error("expected Gateway to be initialized")
	}
	if acct.Outbound == nil {
		t.Error("expected Outbound to be initialized")
	}
	if acct.Proactive == nil {
		t.Error("expected Proactive to be initialized")
	}
	if acct.KnownUsers == nil {
		t.Error("expected KnownUsers to be initialized")
	}
	if acct.RefIndex == nil {
		t.Error("expected RefIndex to be initialized")
	}
	if acct.Sessions == nil {
		t.Error("expected Sessions to be initialized")
	}
}

func TestAddAccount_Duplicate(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("first AddAccount failed: %v", err)
	}

	err = m.AddAccount(account)
	if err == nil {
		t.Fatal("expected error for duplicate account")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddAccount_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "")
	err := m.AddAccount(account)
	if err == nil {
		t.Fatal("expected error for empty appID")
	}
	if !strings.Contains(err.Error(), "appId") {
		t.Errorf("expected appId-related error, got: %v", err)
	}
}

func TestRemoveAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	err = m.RemoveAccount("acct1")
	if err != nil {
		t.Fatalf("RemoveAccount failed: %v", err)
	}

	if m.GetAccount("acct1") != nil {
		t.Error("expected account to be removed")
	}
}

func TestRemoveAccount_Unknown(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	err := m.RemoveAccount("nonexistent")
	if err == nil {
		t.Fatal("expected error for removing unknown account")
	}
}

func TestStop_BeforeStart(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.Stop()

	if m.started {
		t.Error("expected started to be false after stop")
	}
}

func TestGetAllStatuses(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.AddAccount(makeResolvedAccount("acct1", "app123"))
	m.AddAccount(makeResolvedAccount("acct2", "app456"))

	statuses := m.GetAllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, s := range statuses {
		if s.Connected {
			t.Errorf("account %s should not be connected before start", s.ID)
		}
	}

	ids := map[string]bool{}
	for _, s := range statuses {
		ids[s.ID] = true
	}
	if !ids["acct1"] || !ids["acct2"] {
		t.Error("expected both account IDs in statuses")
	}
}

func TestSendC2C_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	err := m.SendC2C(context.Background(), "nonexistent", "openid1", "hello", "msg1")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf("expected 'unknown account' error, got: %v", err)
	}
}

func TestSendGroup_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	err := m.SendGroup(context.Background(), "nonexistent", "group1", "hello", "msg1")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf("expected 'unknown account' error, got: %v", err)
	}
}

func TestBroadcast_NoUsers(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	sent, errs := m.Broadcast(context.Background(), "nonexistent", "hello")
	if sent != 0 {
		t.Errorf("expected 0 sent, got %d", sent)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "unknown account") {
		t.Errorf("expected 'unknown account' error, got: %v", errs[0])
	}
}

func TestMultiAccountIsolation(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account1 := makeResolvedAccount("acct1", "app123")
	account2 := makeResolvedAccount("acct2", "app456")

	m.AddAccount(account1)
	m.AddAccount(account2)

	acct1 := m.GetAccount("acct1")
	acct2 := m.GetAccount("acct2")

	if acct1 == nil || acct2 == nil {
		t.Fatal("expected both accounts to exist")
	}

	// Verify they have different stores
	if acct1.KnownUsers == acct2.KnownUsers {
		t.Error("accounts should have separate KnownUsers stores")
	}
	if acct1.RefIndex == acct2.RefIndex {
		t.Error("accounts should have separate RefIndex stores")
	}
	if acct1.Sessions == acct2.Sessions {
		t.Error("accounts should have separate Session stores")
	}
	if acct1.Client == acct2.Client {
		t.Error("accounts should have separate API clients")
	}
	if acct1.Gateway == acct2.Gateway {
		t.Error("accounts should have separate Gateways")
	}
	if acct1.Outbound == acct2.Outbound {
		t.Error("accounts should have separate Outbound handlers")
	}
	if acct1.Proactive == acct2.Proactive {
		t.Error("accounts should have separate Proactive managers")
	}

	// Record a user in account1's store
	acct1.KnownUsers.Record(store.KnownUser{
		AccountID: "acct1",
		OpenID:    "user1",
		Type:      "c2c",
	})

	// Verify account2 doesn't see it
	users := acct2.KnownUsers.List(store.ListOptions{AccountID: "acct2", Type: "c2c"})
	if len(users) != 0 {
		t.Errorf("account2 should not see account1's users, got %d users", len(users))
	}

	// Verify account1 sees it
	users = acct1.KnownUsers.List(store.ListOptions{AccountID: "acct1", Type: "c2c"})
	if len(users) != 1 {
		t.Errorf("account1 should see its own user, got %d users", len(users))
	}
}

func TestStop_FlushesStores(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	m.AddAccount(account)

	acct := m.GetAccount("acct1")
	acct.KnownUsers.Record(store.KnownUser{
		AccountID: "acct1",
		OpenID:    "user1",
		Type:      "c2c",
	})
	acct.RefIndex.Set("ref1", store.RefIndexEntry{
		Content:  "test content",
		SenderID: "user1",
	})

	m.Stop()

	// Verify SQLite database was written
	dbPath := filepath.Join(dir, "qqbot.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected qqbot.db to exist after Stop")
	}
}

func TestStart_NoAccounts(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("Start with no accounts should not error: %v", err)
	}
	if !m.started {
		t.Error("expected started to be true")
	}
	m.Stop()
}

func TestStart_AlreadyStarted(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.AddAccount(makeResolvedAccount("acct1", "app123"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := m.Start(ctx)
	if err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	// Brief sleep to let the background gateway Connect goroutine start
	// before we call Stop, avoiding a data race on Gateway.cancel.
	time.Sleep(50 * time.Millisecond)

	err = m.Start(ctx)
	if err == nil {
		t.Fatal("expected error when starting already started manager")
	}

	m.Stop()
}

func TestStop_WithStartedManager(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.AddAccount(makeResolvedAccount("acct1", "app123"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := m.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Brief sleep to let the background gateway Connect goroutine start
	// before we call Stop, avoiding a data race on Gateway.cancel.
	time.Sleep(50 * time.Millisecond)

	m.Stop()

	if m.started {
		t.Error("expected started to be false after stop")
	}

	m.Stop()
}

func TestSchedulerInitialized(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	m.AddAccount(account)

	acct := m.GetAccount("acct1")
	if acct.Scheduler == nil {
		t.Error("expected Scheduler to be initialized")
	}

	jobID := acct.Scheduler.AddReminder(proactive.ReminderJob{
		Content:       "test reminder",
		TargetType:    "c2c",
		TargetAddress: "openid1",
		AccountID:     "acct1",
	})
	if jobID == "" {
		t.Error("expected non-empty job ID")
	}
}

func TestEventHandlers(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	m.AddAccount(account)

	acct := m.GetAccount("acct1")

	c2cPayload := `{"author":{"id":"user1","user_openid":"openid1"},"content":"hello","id":"msg1","timestamp":"2025-01-01T00:00:00Z"}`
	if acct.eventHandler == nil {
		t.Fatal("expected eventHandler to be set")
	}
	acct.eventHandler("acct1", "C2C_MESSAGE_CREATE", []byte(c2cPayload))

	users := acct.KnownUsers.List(store.ListOptions{AccountID: "acct1", Type: "c2c"})
	if len(users) != 1 {
		t.Fatalf("expected 1 known user after C2C event, got %d", len(users))
	}
	if users[0].OpenID != "openid1" {
		t.Errorf("expected openid %q, got %q", "openid1", users[0].OpenID)
	}

	groupPayload := `{"author":{"id":"user2","member_openid":"member1"},"content":"hi","id":"msg2","timestamp":"2025-01-01T00:00:00Z","group_id":"grp1","group_openid":"grp_openid1"}`
	acct.eventHandler("acct1", "GROUP_AT_MESSAGE_CREATE", []byte(groupPayload))

	groupUsers := acct.KnownUsers.List(store.ListOptions{AccountID: "acct1", Type: "group"})
	if len(groupUsers) != 1 {
		t.Fatalf("expected 1 known group user after group event, got %d", len(groupUsers))
	}
	if groupUsers[0].GroupOpenID != "grp_openid1" {
		t.Errorf("expected groupOpenID %q, got %q", "grp_openid1", groupUsers[0].GroupOpenID)
	}
}

func TestSendC2C_WithValidAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	m.AddAccount(account)

	err := m.SendC2C(context.Background(), "acct1", "openid1", "hello", "msg1")
	if err != nil && strings.Contains(err.Error(), "unknown account") {
		t.Error("should not get 'unknown account' for valid account ID")
	}
}

func TestSendGroup_WithValidAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccount("acct1", "app123")
	m.AddAccount(account)

	err := m.SendGroup(context.Background(), "acct1", "group1", "hello", "msg1")
	if err != nil && strings.Contains(err.Error(), "unknown account") {
		t.Error("should not get 'unknown account' for valid account ID")
	}
}

func TestGetAllStatuses_WithAccounts(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.AddAccount(makeResolvedAccount("acct1", "app123"))
	m.AddAccount(makeResolvedAccount("acct2", "app456"))

	statuses := m.GetAllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, s := range statuses {
		if s.ID != "acct1" && s.ID != "acct2" {
			t.Errorf("unexpected account ID %q", s.ID)
		}
		if s.Connected {
			t.Errorf("account %s should not be connected", s.ID)
		}
		if s.Error != "" {
			t.Errorf("account %s should have no error, got %q", s.ID, s.Error)
		}
	}
}

func TestSendMedia_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)
	ctx := context.Background()

	// All media send methods should return "unknown account" for nonexistent account
	methods := []struct {
		name string
		fn   func() error
	}{
		{"SendImage", func() error { return m.SendImage(ctx, "no", "c2c", "o1", "url", "", "") }},
		{"SendVoice", func() error { return m.SendVoice(ctx, "no", "c2c", "o1", "b64", "", "") }},
		{"SendVideo", func() error { return m.SendVideo(ctx, "no", "c2c", "o1", "url", "", "", "") }},
		{"SendFile", func() error { return m.SendFile(ctx, "no", "c2c", "o1", "", "url", "f", "") }},
		{"SendChannel", func() error { return m.SendChannel(ctx, "no", "ch1", "hi", "") }},
		{"SendProactiveC2C", func() error { return m.SendProactiveC2C(ctx, "no", "o1", "hi") }},
		{"SendProactiveGroup", func() error { return m.SendProactiveGroup(ctx, "no", "g1", "hi") }},
	}

	for _, tc := range methods {
		err := tc.fn()
		if err == nil || !strings.Contains(err.Error(), "unknown account") {
			t.Errorf("%s: expected 'unknown account' error, got: %v", tc.name, err)
		}
	}
}

func TestBroadcastToGroups_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	sent, errs := m.BroadcastToGroups(context.Background(), "no", "hi")
	if sent != 0 {
		t.Errorf("expected 0 sent, got %d", sent)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "unknown account") {
		t.Errorf("expected 'unknown account' error, got: %v", errs)
	}
}

func TestListUsers_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	users := m.ListUsers("no", store.ListOptions{})
	if users != nil {
		t.Errorf("expected nil for unknown account, got %v", users)
	}
}

func TestGetUserStats_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	stats := m.GetUserStats("no")
	if stats != (store.UserStats{}) {
		t.Errorf("expected empty stats for unknown account, got %+v", stats)
	}
}

func TestClearUsers_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	n := m.ClearUsers("no")
	if n != 0 {
		t.Errorf("expected 0 for unknown account, got %d", n)
	}
}

func TestReminderOps_UnknownAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	_, err := m.AddReminder(proactive.ReminderJob{AccountID: "no"})
	if err == nil || !strings.Contains(err.Error(), "unknown account") {
		t.Errorf("expected 'unknown account' error, got: %v", err)
	}

	if m.CancelReminder("no", "rem-1") {
		t.Error("CancelReminder should return false for unknown account")
	}

	reminders := m.GetReminders("no")
	if reminders != nil {
		t.Errorf("expected nil reminders for unknown account, got %v", reminders)
	}
}

func TestListAccountIDs(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	m.AddAccount(makeResolvedAccount("acct1", "app123"))
	m.AddAccount(makeResolvedAccount("acct2", "app456"))

	ids := m.ListAccountIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["acct1"] || !idSet["acct2"] {
		t.Error("expected both account IDs in list")
	}
}

func TestAddReminder_ValidAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)
	m.AddAccount(makeResolvedAccount("acct1", "app123"))

	jobID, err := m.AddReminder(proactive.ReminderJob{
		AccountID:     "acct1",
		Content:       "test",
		TargetType:    "c2c",
		TargetAddress: "openid1",
	})
	if err != nil {
		t.Fatalf("AddReminder failed: %v", err)
	}
	if jobID == "" {
		t.Fatal("expected non-empty job ID")
	}

	reminders := m.GetReminders("acct1")
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}

	if m.CancelReminder("acct1", jobID) != true {
		t.Error("expected CancelReminder to return true")
	}

	if len(m.GetReminders("acct1")) != 0 {
		t.Error("expected no reminders after cancel")
	}
}

func TestClearUsers_ValidAccount(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)
	m.AddAccount(makeResolvedAccount("acct1", "app123"))

	acct := m.GetAccount("acct1")
	acct.KnownUsers.Record(store.KnownUser{
		AccountID: "acct1",
		OpenID:    "user1",
		Type:      "c2c",
	})
	acct.KnownUsers.Record(store.KnownUser{
		AccountID: "acct1",
		OpenID:    "user2",
		Type:      "c2c",
	})

	n := m.ClearUsers("acct1")
	if n != 2 {
		t.Errorf("expected 2 cleared, got %d", n)
	}

	users := m.ListUsers("acct1", store.ListOptions{Type: "c2c"})
	if len(users) != 0 {
		t.Errorf("expected 0 users after clear, got %d", len(users))
	}
}
