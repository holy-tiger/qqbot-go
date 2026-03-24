package store

import (
	"database/sql"
	"fmt"
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

// KnownUsersStore manages persistent storage of known users using SQLite.
type KnownUsersStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewKnownUsersStore creates a new store backed by the shared DB.
func NewKnownUsersStore(db *DB) *KnownUsersStore {
	return &KnownUsersStore{db: db.SQLDB()}
}

// Record upserts a known user entry.
func (s *KnownUsersStore) Record(user KnownUser) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	_, err := s.db.Exec(`INSERT INTO known_users
		(account_id, open_id, type, group_open_id, nickname, first_seen_at, last_seen_at, interaction_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(account_id, type, open_id, group_open_id) DO UPDATE SET
			last_seen_at = excluded.last_seen_at,
			interaction_count = interaction_count + 1,
			nickname = CASE WHEN excluded.nickname != '' THEN excluded.nickname ELSE nickname END`,
		user.AccountID, user.OpenID, user.Type, user.GroupOpenID, user.Nickname, now, now)
	if err != nil {
		fmt.Printf("[store] Record: %v\n", err)
	}
}

// Get retrieves a single known user.
func (s *KnownUsersStore) Get(accountID, openID, userType, groupOpenID string) *KnownUser {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.db.QueryRow(`SELECT account_id, open_id, type, group_open_id, nickname,
		first_seen_at, last_seen_at, interaction_count
		FROM known_users WHERE account_id = ? AND type = ? AND open_id = ? AND group_open_id = ?`,
		accountID, userType, openID, groupOpenID)

	var u KnownUser
	if err := row.Scan(&u.AccountID, &u.OpenID, &u.Type, &u.GroupOpenID, &u.Nickname,
		&u.FirstSeenAt, &u.LastSeenAt, &u.InteractionCount); err != nil {
		return nil
	}
	return &u
}

// List returns known users matching the given options.
func (s *KnownUsersStore) List(opts ListOptions) []KnownUser {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT account_id, open_id, type, group_open_id, nickname,
		first_seen_at, last_seen_at, interaction_count FROM known_users`
	var args []interface{}
	var where string

	if opts.AccountID != "" {
		where += " WHERE account_id = ?"
		args = append(args, opts.AccountID)
	}
	if opts.Type != "" {
		if where != "" {
			where += " AND type = ?"
		} else {
			where += " WHERE type = ?"
		}
		args = append(args, opts.Type)
	}
	if opts.ActiveWithin > 0 {
		cutoff := time.Now().UnixMilli() - opts.ActiveWithin
		if where != "" {
			where += " AND last_seen_at >= ?"
		} else {
			where += " WHERE last_seen_at >= ?"
		}
		args = append(args, cutoff)
	}

	query += where

	// Sorting.
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "lastSeenAt"
	}
	var col string
	switch sortBy {
	case "firstSeenAt":
		col = "first_seen_at"
	case "interactionCount":
		col = "interaction_count"
	default:
		col = "last_seen_at"
	}

	sortOrder := opts.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	query += " ORDER BY " + col
	if sortOrder == "asc" {
		query += " ASC"
	} else {
		query += " DESC"
	}

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		fmt.Printf("[store] List: %v\n", err)
		return nil
	}
	defer rows.Close()

	var users []KnownUser
	for rows.Next() {
		var u KnownUser
		if err := rows.Scan(&u.AccountID, &u.OpenID, &u.Type, &u.GroupOpenID, &u.Nickname,
			&u.FirstSeenAt, &u.LastSeenAt, &u.InteractionCount); err != nil {
			continue
		}
		users = append(users, u)
	}

	if users == nil {
		return []KnownUser{}
	}
	return users
}

// Stats returns user statistics.
func (s *KnownUsersStore) Stats(accountID string) UserStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	day := int64(24 * 60 * 60 * 1000)

	var total, c2c, grp, active24h, active7d int
	query := `SELECT
		COUNT(*),
		SUM(CASE WHEN type = 'c2c' THEN 1 ELSE 0 END),
		SUM(CASE WHEN type = 'group' THEN 1 ELSE 0 END),
		SUM(CASE WHEN last_seen_at >= ? THEN 1 ELSE 0 END),
		SUM(CASE WHEN last_seen_at >= ? THEN 1 ELSE 0 END)
		FROM known_users`
	var args []interface{}
	args = append(args, now-day, now-7*day)

	if accountID != "" {
		query += " WHERE account_id = ?"
		args = append(args, accountID)
	}

	row := s.db.QueryRow(query, args...)
	if err := row.Scan(&total, &c2c, &grp, &active24h, &active7d); err != nil {
		return UserStats{}
	}

	return UserStats{
		TotalUsers:  total,
		C2CUsers:    c2c,
		GroupUsers:  grp,
		ActiveIn24h: active24h,
		ActiveIn7d:  active7d,
	}
}

// Remove deletes a known user. Returns true if the user was found.
func (s *KnownUsersStore) Remove(accountID, openID, userType, groupOpenID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM known_users
		WHERE account_id = ? AND type = ? AND open_id = ? AND group_open_id = ?`,
		accountID, userType, openID, groupOpenID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// Clear removes users. If accountID is empty, clears all. Returns count of removed users.
func (s *KnownUsersStore) Clear(accountID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res sql.Result
	var err error
	if accountID != "" {
		res, err = s.db.Exec(`DELETE FROM known_users WHERE account_id = ?`, accountID)
	} else {
		res, err = s.db.Exec(`DELETE FROM known_users`)
	}
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return int(n)
}

// Flush is a no-op for SQLite backend; data is written immediately.
func (s *KnownUsersStore) Flush() {}

// Close is a no-op for SQLite backend; the DB connection is managed by DB.Close().
func (s *KnownUsersStore) Close() {}

// GetUserGroups returns group openids for a user.
func (s *KnownUsersStore) GetUserGroups(accountID, openID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT DISTINCT group_open_id FROM known_users
		WHERE account_id = ? AND open_id = ? AND type = 'group' AND group_open_id != ''`,
		accountID, openID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			continue
		}
		groups = append(groups, g)
	}
	return groups
}

// GetGroupMembers returns all known users in a group.
func (s *KnownUsersStore) GetGroupMembers(accountID, groupOpenID string) []KnownUser {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT account_id, open_id, type, group_open_id, nickname,
		first_seen_at, last_seen_at, interaction_count
		FROM known_users WHERE account_id = ? AND type = 'group' AND group_open_id = ?`,
		accountID, groupOpenID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var members []KnownUser
	for rows.Next() {
		var u KnownUser
		if err := rows.Scan(&u.AccountID, &u.OpenID, &u.Type, &u.GroupOpenID, &u.Nickname,
			&u.FirstSeenAt, &u.LastSeenAt, &u.InteractionCount); err != nil {
			continue
		}
		members = append(members, u)
	}
	return members
}
