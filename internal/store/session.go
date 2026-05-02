package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

const sessionExpireTime = 5 * 60 * 1000 // 5 minutes

// SessionState represents a persistent WebSocket session state.
type SessionState struct {
	SessionID        string `json:"session_id"`
	LastSeq          int    `json:"last_seq"`
	LastConnectedAt  int64  `json:"last_connected_at"`
	IntentLevelIndex int    `json:"intent_level_index"`
	AccountID        string `json:"account_id"`
	SavedAt          int64  `json:"saved_at"`
	AppID            string `json:"app_id,omitempty"`
}

// SessionStore manages persistent WebSocket session state per account using SQLite.
type SessionStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewSessionStore creates a new store backed by the shared DB.
func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{db: db.SQLDB()}
}

// Load loads session state for an account. Returns nil if not found, expired, or appID mismatch.
func (s *SessionStore) Load(accountID, expectedAppID string) *SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state SessionState
	err := s.db.QueryRow(`SELECT session_id, last_seq, last_connected_at,
		intent_level_index, account_id, saved_at, app_id
		FROM sessions WHERE account_id = ?`, accountID).Scan(
		&state.SessionID, &state.LastSeq, &state.LastConnectedAt,
		&state.IntentLevelIndex, &state.AccountID, &state.SavedAt, &state.AppID)
	if err != nil {
		return nil
	}

	// Check expiry.
	now := time.Now().UnixMilli()
	if now-state.SavedAt > sessionExpireTime {
		s.db.Exec(`DELETE FROM sessions WHERE account_id = ?`, accountID)
		return nil
	}

	// Check appID mismatch.
	if expectedAppID != "" && state.AppID != "" && state.AppID != expectedAppID {
		s.db.Exec(`DELETE FROM sessions WHERE account_id = ?`, accountID)
		return nil
	}

	if state.SessionID == "" {
		return nil
	}

	return &state
}

// Save persists session state immediately.
func (s *SessionStore) Save(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	state.SavedAt = now

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		state.AccountID, state.SessionID, state.LastSeq,
		state.LastConnectedAt, state.IntentLevelIndex, state.AppID, state.SavedAt)
	if err != nil {
		fmt.Printf("[store] Session Save: %v\n", err)
	}
}

// Clear removes session state for an account.
func (s *SessionStore) Clear(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Exec(`DELETE FROM sessions WHERE account_id = ?`, accountID)
}

// UpdateLastSeq updates the last sequence number for an account.
// P2-11: uses context with timeout to avoid indefinite blocking.
func (s *SessionStore) UpdateLastSeq(accountID string, lastSeq int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seq = ? WHERE account_id = ? AND session_id != ''`,
		lastSeq, accountID)
	if err != nil || res == nil {
		return
	}
	n, _ := res.RowsAffected()
	_ = n // update may affect 0 rows if no session exists
}

// GetAll returns all saved session states.
func (s *SessionStore) GetAll() []SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT session_id, last_seq, last_connected_at,
		intent_level_index, account_id, saved_at, app_id FROM sessions`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var sessions []SessionState
	for rows.Next() {
		var state SessionState
		if err := rows.Scan(&state.SessionID, &state.LastSeq, &state.LastConnectedAt,
			&state.IntentLevelIndex, &state.AccountID, &state.SavedAt, &state.AppID); err != nil {
			continue
		}
		sessions = append(sessions, state)
	}
	return sessions
}

// CleanupExpired removes expired session entries. Returns count of cleaned entries.
func (s *SessionStore) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UnixMilli() - sessionExpireTime
	res, err := s.db.Exec(`DELETE FROM sessions WHERE saved_at < ?`, cutoff)
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return int(n)
}

// Flush is a no-op for SQLite backend.
func (s *SessionStore) Flush() {}

// Close is a no-op for SQLite backend.
func (s *SessionStore) Close() {}

// SessionStats returns the number of stored sessions.
func (s *SessionStore) SessionStats() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count)
	return count
}
