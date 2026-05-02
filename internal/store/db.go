package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Import the pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

// DB wraps a single *sql.DB connection for all stores.
// It handles schema creation, WAL mode, and data migration.
type DB struct {
	db  *sql.DB
	dir string
}

// Open creates (or opens) the SQLite database in dir, enables WAL mode,
// creates tables if needed, and migrates any legacy JSON data.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("store: create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "qqbot.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store: open database: %w", err)
	}

	// Connection pool tuned for WAL: one writer, multiple readers.
	db.SetMaxOpenConns(1)

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: init schema: %w", err)
	}

	d := &DB{db: db, dir: dir}

	if err := migrateFromJSON(d); err != nil {
		// Migration errors are non-fatal; log but don't block startup.
		fmt.Printf("[store] migration warning: %v\n", err)
	}

	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SQLDB returns the underlying *sql.DB for store implementations.
func (d *DB) SQLDB() *sql.DB {
	return d.db
}

// Dir returns the data directory path.
func (d *DB) Dir() string {
	return d.dir
}

// P2-11: defaultTimeout is the context timeout for store operations that lack an explicit context.
const defaultStoreTimeout = 5 * time.Second

// Ctx returns a context with the default store timeout.
func (d *DB) Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultStoreTimeout)
}

// OpenTestDB creates a test SQLite database in t.TempDir().
// Exported for use by test files in other packages.
func OpenTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func initSchema(db *sql.DB) error {
	// WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS known_users (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id        TEXT    NOT NULL,
		open_id           TEXT    NOT NULL,
		type              TEXT    NOT NULL,
		group_open_id     TEXT    NOT NULL DEFAULT '',
		nickname          TEXT    NOT NULL DEFAULT '',
		first_seen_at     INTEGER NOT NULL,
		last_seen_at      INTEGER NOT NULL,
		interaction_count INTEGER NOT NULL DEFAULT 1,
		UNIQUE(account_id, type, open_id, group_open_id)
	);
	CREATE INDEX IF NOT EXISTS idx_ku_account ON known_users(account_id);
	CREATE INDEX IF NOT EXISTS idx_ku_account_type ON known_users(account_id, type);

	CREATE TABLE IF NOT EXISTS ref_index (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ref_key     TEXT    NOT NULL,
		content     TEXT    NOT NULL DEFAULT '',
		sender_id   TEXT    NOT NULL DEFAULT '',
		sender_name TEXT    NOT NULL DEFAULT '',
		timestamp   INTEGER NOT NULL DEFAULT 0,
		is_bot      INTEGER NOT NULL DEFAULT 0,
		attachments TEXT    NOT NULL DEFAULT '[]',
		created_at  INTEGER NOT NULL,
		UNIQUE(ref_key)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		account_id         TEXT PRIMARY KEY,
		session_id         TEXT    NOT NULL DEFAULT '',
		last_seq           INTEGER NOT NULL DEFAULT 0,
		last_connected_at  INTEGER NOT NULL DEFAULT 0,
		intent_level_index INTEGER NOT NULL DEFAULT 0,
		app_id             TEXT    NOT NULL DEFAULT '',
		saved_at           INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS reminders (
		id              TEXT PRIMARY KEY,
		account_id      TEXT    NOT NULL DEFAULT '',
		content         TEXT    NOT NULL DEFAULT '',
		target_type     TEXT    NOT NULL DEFAULT '',
		target_address  TEXT    NOT NULL DEFAULT '',
		schedule        TEXT    NOT NULL DEFAULT '',
		next_run        INTEGER NOT NULL DEFAULT 0,
		created_at      INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_rem_account ON reminders(account_id);
	`
	_, err := db.Exec(schema)
	return err
}
