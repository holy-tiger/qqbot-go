package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const (
	sessionExpireTime = 5 * 60 * 1000 // 5 minutes
	sessionSaveThrottle = 1000       // 1 second
)

// SessionState represents a persistent WebSocket session state.
type SessionState struct {
	SessionID         string `json:"session_id"`
	LastSeq           int    `json:"last_seq"`
	LastConnectedAt   int64  `json:"last_connected_at"`
	IntentLevelIndex  int    `json:"intent_level_index"`
	AccountID         string `json:"account_id"`
	SavedAt           int64  `json:"saved_at"`
	AppID             string `json:"app_id,omitempty"`
}

var safeIDRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// SessionStore manages persistent WebSocket session state per account.
type SessionStore struct {
	dir            string
	mu             sync.Mutex
	throttleTimers map[string]*time.Timer
	pendingStates  map[string]*SessionState
	lastSaveTimes  map[string]int64
}

// NewSessionStore creates a new store backed by dir.
func NewSessionStore(dir string) *SessionStore {
	return &SessionStore{
		dir:            dir,
		throttleTimers: make(map[string]*time.Timer),
		pendingStates:  make(map[string]*SessionState),
		lastSaveTimes:  make(map[string]int64),
	}
}

func (s *SessionStore) getSessionPath(accountID string) string {
	safeID := safeIDRe.ReplaceAllString(accountID, "_")
	return filepath.Join(s.dir, "session-"+safeID+".json")
}

func (s *SessionStore) ensureDir() {
	os.MkdirAll(s.dir, 0755)
}

// Load loads session state for an account. Returns nil if not found, expired, or appID mismatch.
func (s *SessionStore) Load(accountID, expectedAppID string) *SessionState {
	fp := s.getSessionPath(accountID)

	data, err := os.ReadFile(fp)
	if err != nil {
		return nil
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}

	// Check expiry
	now := time.Now().UnixMilli()
	if now-state.SavedAt > sessionExpireTime {
		os.Remove(fp)
		return nil
	}

	// Check appID mismatch
	if expectedAppID != "" && state.AppID != "" && state.AppID != expectedAppID {
		os.Remove(fp)
		return nil
	}

	// Validate required fields
	if state.SessionID == "" || state.LastSeq == 0 && state.SessionID != "" {
		// last_seq=0 is technically valid if session_id is set
		if state.SessionID == "" {
			return nil
		}
	}

	return &state
}

// Save persists session state with throttled writes.
func (s *SessionStore) Save(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountID := state.AccountID
	now := time.Now().UnixMilli()
	timeSinceLastSave := now - s.lastSaveTimes[accountID]

	if timeSinceLastSave >= sessionSaveThrottle {
		s.doSave(state)
		s.lastSaveTimes[accountID] = now
		s.pendingStates[accountID] = nil

		if timer, ok := s.throttleTimers[accountID]; ok {
			timer.Stop()
			delete(s.throttleTimers, accountID)
		}
	} else {
		s.pendingStates[accountID] = &state

		if _, ok := s.throttleTimers[accountID]; !ok {
			delay := sessionSaveThrottle - timeSinceLastSave
			s.throttleTimers[accountID] = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
				s.mu.Lock()
				defer s.mu.Unlock()

				if pending, ok := s.pendingStates[accountID]; ok && pending != nil {
					s.doSave(*pending)
					s.lastSaveTimes[accountID] = time.Now().UnixMilli()
					s.pendingStates[accountID] = nil
				}
				delete(s.throttleTimers, accountID)
			})
		}
	}
}

func (s *SessionStore) doSave(state SessionState) {
	s.ensureDir()

	state.SavedAt = time.Now().UnixMilli()
	data, _ := json.MarshalIndent(state, "", "  ")
	fp := s.getSessionPath(state.AccountID)
	os.WriteFile(fp, data, 0644)
}

// Clear removes session state for an account.
func (s *SessionStore) Clear(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timer, ok := s.throttleTimers[accountID]; ok {
		timer.Stop()
		delete(s.throttleTimers, accountID)
	}
	delete(s.pendingStates, accountID)

	fp := s.getSessionPath(accountID)
	os.Remove(fp)
}

// UpdateLastSeq updates the last sequence number for an account.
func (s *SessionStore) UpdateLastSeq(accountID string, lastSeq int) {
	existing := s.Load(accountID, "")
	if existing != nil && existing.SessionID != "" {
		existing.LastSeq = lastSeq
		s.Save(*existing)
	}
}

// GetAll returns all saved session states.
func (s *SessionStore) GetAll() []SessionState {
	s.ensureDir()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}

	var sessions []SessionState
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 8 && name[:8] == "session-" && len(name) > 5 && name[len(name)-5:] == ".json" {
			fp := filepath.Join(s.dir, name)
			data, err := os.ReadFile(fp)
			if err != nil {
				continue
			}
			var state SessionState
			if err := json.Unmarshal(data, &state); err != nil {
				continue
			}
			sessions = append(sessions, state)
		}
	}
	return sessions
}

// CleanupExpired removes expired session files. Returns count of cleaned files.
func (s *SessionStore) CleanupExpired() int {
	s.ensureDir()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0
	}

	now := time.Now().UnixMilli()
	cleaned := 0

	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 8 && name[:8] == "session-" && name[len(name)-5:] == ".json" {
			fp := filepath.Join(s.dir, name)
			data, err := os.ReadFile(fp)
			if err != nil {
				// Corrupted file, remove it
				os.Remove(fp)
				cleaned++
				continue
			}
			var state SessionState
			if err := json.Unmarshal(data, &state); err != nil {
				os.Remove(fp)
				cleaned++
				continue
			}
			if now-state.SavedAt > sessionExpireTime {
				os.Remove(fp)
				cleaned++
			}
		}
	}
	return cleaned
}

// Flush forces any pending throttled writes.
func (s *SessionStore) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for accountID, pending := range s.pendingStates {
		if pending != nil {
			s.doSave(*pending)
			s.lastSaveTimes[accountID] = time.Now().UnixMilli()
			s.pendingStates[accountID] = nil
		}
		if timer, ok := s.throttleTimers[accountID]; ok {
			timer.Stop()
			delete(s.throttleTimers, accountID)
		}
	}
}

// Close is an alias for Flush.
func (s *SessionStore) Close() {
	s.Flush()
}
