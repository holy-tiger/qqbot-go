package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestSessionStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 1,
		AccountID:        "acct1",
		AppID:            "app1",
	}
	s.Save(state)

	loaded := s.Load("acct1", "app1")
	if loaded == nil {
		t.Fatal("expected to load session")
	}
	if loaded.SessionID != "sess123" {
		t.Errorf("expected session_id=sess123, got %s", loaded.SessionID)
	}
	if loaded.LastSeq != 42 {
		t.Errorf("expected last_seq=42, got %d", loaded.LastSeq)
	}
}

func TestSessionStore_LoadNotExist(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	loaded := s.Load("nonexistent", "")
	if loaded != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestSessionStore_LoadExpired(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	}
	s.Save(state)

	// Manually backdate savedAt to simulate expiry
	fp := filepath.Join(dir, "session-acct1.json")
	data, _ := os.ReadFile(fp)
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	m["saved_at"] = time.Now().UnixMilli() - (5*60*1000 + 1) // 5m1s ago
	updated, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(fp, updated, 0644)

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil for expired session")
	}

	// Expired file should be deleted
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Error("expected expired session file to be deleted")
	}
}

func TestSessionStore_AppIDMismatch(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
		AppID:            "old_app",
	}
	s.Save(state)

	loaded := s.Load("acct1", "new_app")
	if loaded != nil {
		t.Error("expected nil for appID mismatch")
	}

	// File should be deleted
	fp := filepath.Join(dir, "session-acct1.json")
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Error("expected session file to be deleted on appID mismatch")
	}
}

func TestSessionStore_NoAppIDCheck(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
		AppID:            "app1",
	}
	s.Save(state)

	// Don't pass expectedAppID - should load regardless
	loaded := s.Load("acct1", "")
	if loaded == nil {
		t.Fatal("expected to load session without appID check")
	}
	if loaded.SessionID != "sess123" {
		t.Errorf("expected sess123, got %s", loaded.SessionID)
	}
}

func TestSessionStore_SaveWithThrottle(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	}
	s.Save(state)

	// File might not exist immediately (throttled), but should exist after Close/Flush
	s.Flush()
	fp := filepath.Join(dir, "session-acct1.json")
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("expected file to exist after flush: %v", err)
	}

	var loaded SessionState
	data, _ := os.ReadFile(fp)
	json.Unmarshal(data, &loaded)
	if loaded.SessionID != "sess123" {
		t.Errorf("expected sess123, got %s", loaded.SessionID)
	}
}

func TestSessionStore_ThrottledWrites(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	// Rapid saves should be throttled
	for i := 0; i < 10; i++ {
		s.Save(SessionState{
			SessionID:        "sess123",
			LastSeq:          i,
			LastConnectedAt:  now,
			IntentLevelIndex: 0,
			AccountID:        "acct1",
		})
	}

	s.Flush()

	// Should have the last state
	loaded := s.Load("acct1", "")
	if loaded == nil {
		t.Fatal("expected to load session")
	}
	if loaded.LastSeq != 9 {
		t.Errorf("expected last_seq=9, got %d", loaded.LastSeq)
	}
}

func TestSessionStore_Clear(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	}
	s.Save(state)
	s.Flush()

	s.Clear("acct1")

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil after clear")
	}
}

func TestSessionStore_UpdateLastSeq(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	}
	s.Save(state)
	s.Flush()

	s.UpdateLastSeq("acct1", 100)
	s.Flush()

	loaded := s.Load("acct1", "")
	if loaded == nil {
		t.Fatal("expected to load after updateLastSeq")
	}
	if loaded.LastSeq != 100 {
		t.Errorf("expected last_seq=100, got %d", loaded.LastSeq)
	}
}

func TestSessionStore_GetAll(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Save(SessionState{SessionID: "s1", LastSeq: 1, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct1"})
	s.Save(SessionState{SessionID: "s2", LastSeq: 2, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct2"})
	s.Flush()

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(all))
	}
}

func TestSessionStore_CleanupExpired(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	now := time.Now().UnixMilli()
	// Save a valid session
	s.Save(SessionState{SessionID: "valid", LastSeq: 1, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct_valid"})
	s.Flush()

	// Manually create an expired session file
	expired := SessionState{SessionID: "expired", LastSeq: 2, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct_expired", SavedAt: now - (5*60*1000 + 1)}
	data, _ := json.MarshalIndent(expired, "", "  ")
	safeID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString("acct_expired", "_")
	os.WriteFile(filepath.Join(dir, "session-"+safeID+".json"), data, 0644)

	cleaned := s.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleaned)
	}

	// Valid session should still exist
	loaded := s.Load("acct_valid", "")
	if loaded == nil {
		t.Error("expected valid session to still exist")
	}
}

func TestSessionStore_SafeFilename(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	state := SessionState{
		SessionID:        "sess",
		LastSeq:          1,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct/with@special#chars",
	}
	s.Save(state)
	s.Flush()

	// File should be created with sanitized name
	safeID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString("acct/with@special#chars", "_")
	fp := filepath.Join(dir, "session-"+safeID+".json")
	if _, err := os.Stat(fp); err != nil {
		t.Errorf("expected file with sanitized name: %v", err)
	}

	// Should still load correctly
	loaded := s.Load("acct/with@special#chars", "")
	if loaded == nil {
		t.Fatal("expected to load session with special chars in accountID")
	}
}

func TestSessionStore_InvalidSessionData(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)
	defer s.Close()

	// Write invalid session data (missing sessionId)
	safeID := "acct1"
	os.WriteFile(filepath.Join(dir, "session-"+safeID+".json"), []byte(`{"session_id":"","last_seq":null,"account_id":"acct1","saved_at":`+fmt.Sprintf("%d", time.Now().UnixMilli())+`}`), 0644)

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil for invalid session data")
	}
}
