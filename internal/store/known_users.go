package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// KnownUser represents a user who has interacted with the bot.
type KnownUser struct {
	OpenID           string `json:"openid"`
	Type             string `json:"type"`
	Nickname         string `json:"nickname,omitempty"`
	GroupOpenID      string `json:"group_openid,omitempty"`
	AccountID        string `json:"account_id"`
	FirstSeenAt      int64  `json:"first_seen_at"`
	LastSeenAt       int64  `json:"last_seen_at"`
	InteractionCount int    `json:"interaction_count"`
}

// ListOptions controls filtering and sorting for listing known users.
type ListOptions struct {
	AccountID    string
	Type         string
	ActiveWithin int64 // milliseconds
	Limit        int
	SortBy       string // "lastSeenAt", "firstSeenAt", "interactionCount"
	SortOrder    string // "asc" or "desc"
}

// UserStats provides statistics about known users.
type UserStats struct {
	TotalUsers int `json:"total_users"`
	C2CUsers   int `json:"c2c_users"`
	GroupUsers int `json:"group_users"`
	ActiveIn24h int `json:"active_in_24h"`
	ActiveIn7d  int `json:"active_in_7d"`
}

const saveThrottleMs = 5000

// KnownUsersStore manages persistent storage of known users.
type KnownUsersStore struct {
	dir       string
	filePath  string
	mu        sync.Mutex
	cache     map[string]*KnownUser
	dirty     bool
	saveTimer *time.Timer
	closed    bool
}

// NewKnownUsersStore creates a new store backed by dir.
func NewKnownUsersStore(dir string) *KnownUsersStore {
	return &KnownUsersStore{
		dir:      dir,
		filePath: filepath.Join(dir, "known-users.json"),
		cache:    make(map[string]*KnownUser),
	}
}

func makeUserKey(accountID, userType, openID, groupOpenID string) string {
	base := accountID + ":" + userType + ":" + openID
	if userType == "group" && groupOpenID != "" {
		return base + ":" + groupOpenID
	}
	return base
}

func (s *KnownUsersStore) load() {
	if len(s.cache) > 0 {
		return
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var users []KnownUser
	if err := json.Unmarshal(data, &users); err != nil {
		return
	}

	for i := range users {
		u := &users[i]
		key := makeUserKey(u.AccountID, u.Type, u.OpenID, u.GroupOpenID)
		s.cache[key] = u
	}
}

func (s *KnownUsersStore) scheduleSave() {
	if s.saveTimer != nil {
		return
	}
	s.saveTimer = time.AfterFunc(saveThrottleMs, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		s.saveTimer = nil
		s.doSave()
	})
}

func (s *KnownUsersStore) doSave() {
	if !s.dirty {
		return
	}
	users := make([]KnownUser, 0, len(s.cache))
	for _, u := range s.cache {
		users = append(users, *u)
	}
	os.MkdirAll(s.dir, 0755)
	data, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile(s.filePath, data, 0644)
	s.dirty = false
}

// Record upserts a known user entry.
func (s *KnownUsersStore) Record(user KnownUser) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.load()
	key := makeUserKey(user.AccountID, user.Type, user.OpenID, user.GroupOpenID)
	now := time.Now().UnixMilli()

	existing, ok := s.cache[key]
	if ok {
		existing.LastSeenAt = now
		existing.InteractionCount++
		if user.Nickname != "" && user.Nickname != existing.Nickname {
			existing.Nickname = user.Nickname
		}
	} else {
		user.FirstSeenAt = now
		user.LastSeenAt = now
		user.InteractionCount = 1
		s.cache[key] = &user
	}

	s.dirty = true
	s.scheduleSave()
}

// Get retrieves a single known user.
func (s *KnownUsersStore) Get(accountID, openID, userType, groupOpenID string) *KnownUser {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.load()
	key := makeUserKey(accountID, userType, openID, groupOpenID)
	u, ok := s.cache[key]
	if !ok {
		return nil
	}
	return u
}

// List returns known users matching the given options.
func (s *KnownUsersStore) List(opts ListOptions) []KnownUser {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.load()
	users := make([]KnownUser, 0, len(s.cache))
	for _, u := range s.cache {
		users = append(users, *u)
	}

	if opts.AccountID != "" {
		filtered := users[:0]
		for _, u := range users {
			if u.AccountID == opts.AccountID {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}
	if opts.Type != "" {
		filtered := users[:0]
		for _, u := range users {
			if u.Type == opts.Type {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}
	if opts.ActiveWithin > 0 {
		cutoff := time.Now().UnixMilli() - opts.ActiveWithin
		filtered := users[:0]
		for _, u := range users {
			if u.LastSeenAt >= cutoff {
				filtered = append(filtered, u)
			}
		}
		users = filtered
	}

	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "lastSeenAt"
	}
	sortOrder := opts.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(users, func(i, j int) bool {
		var aVal, bVal int64
		switch sortBy {
		case "firstSeenAt":
			aVal, bVal = users[i].FirstSeenAt, users[j].FirstSeenAt
		case "interactionCount":
			aVal, bVal = int64(users[i].InteractionCount), int64(users[j].InteractionCount)
		default:
			aVal, bVal = users[i].LastSeenAt, users[j].LastSeenAt
		}
		if sortOrder == "asc" {
			return aVal < bVal
		}
		return aVal > bVal
	})

	if opts.Limit > 0 && len(users) > opts.Limit {
		users = users[:opts.Limit]
	}

	return users
}

// Stats returns user statistics.
func (s *KnownUsersStore) Stats(accountID string) UserStats {
	users := s.List(ListOptions{AccountID: accountID})

	now := time.Now().UnixMilli()
	day := int64(24 * 60 * 60 * 1000)

	stats := UserStats{TotalUsers: len(users)}
	for _, u := range users {
		switch u.Type {
		case "c2c":
			stats.C2CUsers++
		case "group":
			stats.GroupUsers++
		}
		if now-u.LastSeenAt < day {
			stats.ActiveIn24h++
		}
		if now-u.LastSeenAt < 7*day {
			stats.ActiveIn7d++
		}
	}
	return stats
}

// Remove deletes a known user. Returns true if the user was found.
func (s *KnownUsersStore) Remove(accountID, openID, userType, groupOpenID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.load()
	key := makeUserKey(accountID, userType, openID, groupOpenID)
	if _, ok := s.cache[key]; !ok {
		return false
	}
	delete(s.cache, key)
	s.dirty = true
	s.scheduleSave()
	return true
}

// Clear removes users. If accountID is empty, clears all.
func (s *KnownUsersStore) Clear(accountID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.load()
	count := 0

	if accountID != "" {
		for key, u := range s.cache {
			if u.AccountID == accountID {
				delete(s.cache, key)
				count++
			}
		}
	} else {
		count = len(s.cache)
		s.cache = make(map[string]*KnownUser)
	}

	if count > 0 {
		s.dirty = true
		s.doSave()
	}
	return count
}

// Close stops timers and flushes to disk.
func (s *KnownUsersStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.doSave()
	s.closed = true
}

// Flush forces an immediate write to disk.
func (s *KnownUsersStore) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.doSave()
}

// GetUserGroups returns group openids for a user.
func (s *KnownUsersStore) GetUserGroups(accountID, openID string) []string {
	users := s.List(ListOptions{AccountID: accountID, Type: "group"})
	groups := make([]string, 0)
	for _, u := range users {
		if u.OpenID == openID && u.GroupOpenID != "" {
			groups = append(groups, u.GroupOpenID)
		}
	}
	return groups
}

// GetGroupMembers returns all known users in a group.
func (s *KnownUsersStore) GetGroupMembers(accountID, groupOpenID string) []KnownUser {
	users := s.List(ListOptions{AccountID: accountID, Type: "group"})
	members := make([]KnownUser, 0)
	for _, u := range users {
		if u.GroupOpenID == groupOpenID {
			members = append(members, u)
		}
	}
	return members
}
