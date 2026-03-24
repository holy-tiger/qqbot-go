package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// migrateFromJSON imports data from legacy JSON/JSONL files into SQLite,
// then renames each file to .bak. This runs once on Open().
func migrateFromJSON(d *DB) error {
	var errs []string

	if err := migrateKnownUsers(d); err != nil {
		errs = append(errs, err.Error())
	}
	if err := migrateRefIndex(d); err != nil {
		errs = append(errs, err.Error())
	}
	if err := migrateSessions(d); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("migration issues: %s", strings.Join(errs, "; "))
	}
	return nil
}

func migrateKnownUsers(d *DB) error {
	fp := filepath.Join(d.dir, "known-users.json")
	if _, err := os.Stat(fp); err != nil {
		return nil // file doesn't exist, nothing to migrate
	}
	if _, err := os.Stat(fp + ".bak"); err == nil {
		return nil // already migrated
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Errorf("read known-users.json: %w", err)
	}

	var users []KnownUser
	if err := json.Unmarshal(data, &users); err != nil {
		return fmt.Errorf("parse known-users.json: %w", err)
	}

	if len(users) == 0 {
		return renameToBak(fp)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO known_users
		(account_id, open_id, type, group_open_id, nickname, first_seen_at, last_seen_at, interaction_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, u := range users {
		_, err := stmt.Exec(u.AccountID, u.OpenID, u.Type, u.GroupOpenID, u.Nickname,
			u.FirstSeenAt, u.LastSeenAt, u.InteractionCount)
		if err != nil {
			continue // skip bad rows
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("[store] migrated %d known users from JSON to SQLite\n", count)
	return renameToBak(fp)
}

func migrateRefIndex(d *DB) error {
	fp := filepath.Join(d.dir, "ref-index.jsonl")
	if _, err := os.Stat(fp); err != nil {
		return nil
	}
	if _, err := os.Stat(fp + ".bak"); err == nil {
		return nil
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Errorf("read ref-index.jsonl: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO ref_index
		(ref_key, content, sender_id, sender_name, timestamp, is_bot, attachments, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry refIndexLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.K == "" || entry.T == 0 {
			continue
		}

		attJSON, _ := json.Marshal(entry.V.Attachments)
		_, err := stmt.Exec(entry.K, entry.V.Content, entry.V.SenderID, entry.V.SenderName,
			entry.V.Timestamp, entry.V.IsBot, string(attJSON), entry.T)
		if err != nil {
			continue
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	fmt.Printf("[store] migrated %d ref index entries from JSONL to SQLite\n", count)
	return renameToBak(fp)
}

func migrateSessions(d *DB) error {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO sessions
		(account_id, session_id, last_seq, last_connected_at, intent_level_index, app_id, saved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasSuffix(name, ".json.bak") {
			continue
		}

		fp := filepath.Join(d.dir, name)
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}

		var state SessionState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}
		if state.AccountID == "" {
			continue
		}

		_, err = stmt.Exec(state.AccountID, state.SessionID, state.LastSeq,
			state.LastConnectedAt, state.IntentLevelIndex, state.AppID, state.SavedAt)
		if err != nil {
			continue
		}
		count++

		if err := renameToBak(fp); err != nil {
			fmt.Printf("[store] warning: could not rename session file %s: %v\n", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if count > 0 {
		fmt.Printf("[store] migrated %d sessions from JSON to SQLite\n", count)
	}
	return nil
}

func renameToBak(fp string) error {
	bak := fp + ".bak"
	if err := os.Rename(fp, bak); err != nil {
		return fmt.Errorf("rename %s to .bak: %w", fp, err)
	}
	return nil
}
