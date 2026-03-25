package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ReminderRecord represents a persistent reminder job.
type ReminderRecord struct {
	ID            string
	Content       string
	TargetType    string // "c2c" or "group"
	TargetAddress string // openid or group openid
	AccountID     string
	Schedule      string
	NextRun       time.Time
	CreatedAt     time.Time
}

// ReminderStore manages persistent reminder storage using SQLite.
type ReminderStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewReminderStore creates a new store backed by the shared DB.
func NewReminderStore(db *DB) *ReminderStore {
	return &ReminderStore{db: db.SQLDB()}
}

// Save upserts a reminder record.
func (s *ReminderStore) Save(r ReminderRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT OR REPLACE INTO reminders
		(id, account_id, content, target_type, target_address, schedule, next_run, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.AccountID, r.Content, r.TargetType, r.TargetAddress,
		r.Schedule, r.NextRun.UnixMilli(), r.CreatedAt.UnixMilli())
	if err != nil {
		fmt.Printf("[store] Reminder Save: %v\n", err)
	}
}

// Get retrieves a single reminder by ID. Returns nil if not found.
func (s *ReminderStore) Get(id string) *ReminderRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	var r ReminderRecord
	var nextRun, createdAt int64

	err := s.db.QueryRow(`SELECT id, account_id, content, target_type, target_address,
		schedule, next_run, created_at FROM reminders WHERE id = ?`, id).Scan(
		&r.ID, &r.AccountID, &r.Content, &r.TargetType, &r.TargetAddress,
		&r.Schedule, &nextRun, &createdAt)
	if err != nil {
		return nil
	}
	r.NextRun = time.UnixMilli(nextRun)
	r.CreatedAt = time.UnixMilli(createdAt)
	return &r
}

// Delete removes a reminder by ID. Returns true if found.
func (s *ReminderStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM reminders WHERE id = ?`, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// ListByAccount returns all reminders for a given account ID.
func (s *ReminderStore) ListByAccount(accountID string) []ReminderRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT id, account_id, content, target_type, target_address,
		schedule, next_run, created_at FROM reminders WHERE account_id = ?`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	return scanReminders(rows)
}

// ListAll returns all reminders.
func (s *ReminderStore) ListAll() []ReminderRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT id, account_id, content, target_type, target_address,
		schedule, next_run, created_at FROM reminders`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	return scanReminders(rows)
}

// Clear removes all reminders for a given account. Returns count removed.
func (s *ReminderStore) Clear(accountID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.Exec(`DELETE FROM reminders WHERE account_id = ?`, accountID)
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return int(n)
}

// Flush is a no-op for SQLite backend.
func (s *ReminderStore) Flush() {}

// Close is a no-op for SQLite backend.
func (s *ReminderStore) Close() {}

func scanReminders(rows *sql.Rows) []ReminderRecord {
	var result []ReminderRecord
	for rows.Next() {
		var r ReminderRecord
		var nextRun, createdAt int64
		if err := rows.Scan(&r.ID, &r.AccountID, &r.Content, &r.TargetType, &r.TargetAddress,
			&r.Schedule, &nextRun, &createdAt); err != nil {
			continue
		}
		r.NextRun = time.UnixMilli(nextRun)
		r.CreatedAt = time.UnixMilli(createdAt)
		result = append(result, r)
	}
	if result == nil {
		return []ReminderRecord{}
	}
	return result
}
