package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestSessionStore_SaveAndLoad(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
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
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	loaded := s.Load("nonexistent", "")
	if loaded != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestSessionStore_LoadExpired(t *testing.T) {
	db := openTestDB(t)
	s := NewSessionStore(db)
	defer s.Close()

	// Insert with expired savedAt
	oldTime := time.Now().UnixMilli() - (5*60*1000 + 1)
	db.SQLDB().Exec(`INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"acct1", "sess123", 42, time.Now().UnixMilli(), 0, "", oldTime)

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil for expired session")
	}
}

func TestSessionStore_AppIDMismatch(t *testing.T) {
	db := openTestDB(t)
	s := NewSessionStore(db)
	defer s.Close()

	now := time.Now().UnixMilli()
	db.SQLDB().Exec(`INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"acct1", "sess123", 42, now, 0, "old_app", now)

	loaded := s.Load("acct1", "new_app")
	if loaded != nil {
		t.Error("expected nil for appID mismatch")
	}
}

func TestSessionStore_NoAppIDCheck(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
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

	loaded := s.Load("acct1", "")
	if loaded == nil {
		t.Fatal("expected to load session without appID check")
	}
	if loaded.SessionID != "sess123" {
		t.Errorf("expected sess123, got %s", loaded.SessionID)
	}
}

func TestSessionStore_Clear(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	s.Save(SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	})

	s.Clear("acct1")

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil after clear")
	}
}

func TestSessionStore_UpdateLastSeq(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	s.Save(SessionState{
		SessionID:        "sess123",
		LastSeq:          42,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct1",
	})

	s.UpdateLastSeq("acct1", 100)

	loaded := s.Load("acct1", "")
	if loaded == nil {
		t.Fatal("expected to load after updateLastSeq")
	}
	if loaded.LastSeq != 100 {
		t.Errorf("expected last_seq=100, got %d", loaded.LastSeq)
	}
}

func TestSessionStore_UpdateLastSeq_NoSession(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	// UpdateLastSeq on non-existent session should not panic
	s.UpdateLastSeq("nonexistent", 100)
}

func TestSessionStore_GetAll(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Save(SessionState{SessionID: "s1", LastSeq: 1, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct1"})
	s.Save(SessionState{SessionID: "s2", LastSeq: 2, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct2"})

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(all))
	}
}

func TestSessionStore_CleanupExpired(t *testing.T) {
	db := openTestDB(t)
	s := NewSessionStore(db)
	defer s.Close()

	now := time.Now().UnixMilli()

	// Save a valid session
	s.Save(SessionState{SessionID: "valid", LastSeq: 1, LastConnectedAt: now, IntentLevelIndex: 0, AccountID: "acct_valid"})

	// Insert expired session directly
	oldTime := now - (5*60*1000 + 1)
	db.SQLDB().Exec(`INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"acct_expired", "expired", 2, now, 0, "", oldTime)

	cleaned := s.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleaned)
	}

	loaded := s.Load("acct_valid", "")
	if loaded == nil {
		t.Error("expected valid session to still exist")
	}
}

func TestSessionStore_InvalidSessionData(t *testing.T) {
	db := openTestDB(t)
	s := NewSessionStore(db)
	defer s.Close()

	// Insert session with empty session_id
	now := time.Now().UnixMilli()
	db.SQLDB().Exec(`INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, '', 0, 0, 0, '', ?)`,
		"acct1", now)

	loaded := s.Load("acct1", "")
	if loaded != nil {
		t.Error("expected nil for invalid session data (empty session_id)")
	}
}

func TestSessionStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1 := NewSessionStore(db1)
	s1.Save(SessionState{
		SessionID:        "sess-persist",
		LastSeq:          99,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 2,
		AccountID:        "acct-persist",
		AppID:            "app-persist",
	})
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := NewSessionStore(db2)
	loaded := s2.Load("acct-persist", "app-persist")
	if loaded == nil {
		t.Fatal("session not found after reopen")
	}
	if loaded.SessionID != "sess-persist" {
		t.Errorf("expected sess-persist, got %s", loaded.SessionID)
	}
	if loaded.LastSeq != 99 {
		t.Errorf("expected last_seq 99, got %d", loaded.LastSeq)
	}
}

func TestSessionStore_SpecialAccountID(t *testing.T) {
	s := NewSessionStore(openTestDB(t))
	defer s.Close()

	// Account ID with special characters should work in SQLite
	s.Save(SessionState{
		SessionID:        "sess",
		LastSeq:          1,
		LastConnectedAt:  time.Now().UnixMilli(),
		IntentLevelIndex: 0,
		AccountID:        "acct/with@special#chars",
	})

	loaded := s.Load("acct/with@special#chars", "")
	if loaded == nil {
		t.Fatal("expected to load session with special chars in accountID")
	}
	if loaded.SessionID != "sess" {
		t.Errorf("expected sess, got %s", loaded.SessionID)
	}
}

// Ensure SessionStore implements expected interfaces
var _ interface {
	Load(string, string) *SessionState
	Save(SessionState)
	Clear(string)
	UpdateLastSeq(string, int)
	GetAll() []SessionState
	CleanupExpired() int
	Flush()
	Close()
} = (*SessionStore)(nil)

// Ensure the db field is accessible for direct SQL in tests
var _ *sql.DB = (&SessionStore{}).db
