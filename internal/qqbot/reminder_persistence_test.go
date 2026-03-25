package qqbot

import (
	"context"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
)

// TestReminderPersistence_WritesToSQLite verifies that the scheduler
// correctly persists reminders to SQLite via the adapter.
func TestReminderPersistence_WritesToSQLite(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	rs := store.NewReminderStore(db)
	mgr := proactive.NewProactiveManager(nil, store.NewKnownUsersStore(db))
	scheduler := proactive.NewScheduler(mgr)
	scheduler.SetStore(newReminderStoreAdapter(rs))

	// Add a reminder
	now := time.Now()
	jobID := scheduler.AddReminder(proactive.ReminderJob{
		Content:       "persist test",
		TargetType:    "c2c",
		TargetAddress: "user1",
		AccountID:     "acct1",
		Schedule:      "@every 30m",
		NextRun:       now.Add(30 * time.Minute),
		CreatedAt:     now,
	})

	// Verify it was persisted to SQLite
	record := rs.Get(jobID)
	if record == nil {
		t.Fatal("expected reminder to be persisted in SQLite")
	}
	if record.Content != "persist test" {
		t.Errorf("expected content %q, got %q", "persist test", record.Content)
	}
	if record.AccountID != "acct1" {
		t.Errorf("expected account_id %q, got %q", "acct1", record.AccountID)
	}

	// Cancel and verify deletion
	ok := scheduler.CancelReminder(jobID)
	if !ok {
		t.Error("expected CancelReminder to return true")
	}
	if rs.Get(jobID) != nil {
		t.Error("expected reminder to be deleted from SQLite after cancel")
	}
}

// TestReminderPersistence_LoadsOnStart verifies that persisted reminders
// are loaded when the scheduler starts.
func TestReminderPersistence_LoadsOnStart(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	rs := store.NewReminderStore(db)

	// Pre-populate a reminder directly in SQLite
	rs.Save(store.ReminderRecord{
		ID:            "pre-existing",
		Content:       "loaded from db",
		TargetType:    "c2c",
		TargetAddress: "user1",
		AccountID:     "acct1",
		Schedule:      "@every 1h",
		NextRun:       time.Now().Add(1 * time.Hour),
		CreatedAt:     time.Now(),
	})

	// Create scheduler with the store and start it
	mgr := proactive.NewProactiveManager(nil, store.NewKnownUsersStore(db))
	scheduler := proactive.NewScheduler(mgr)
	scheduler.SetStore(newReminderStoreAdapter(rs))

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	// Brief wait for loading
	time.Sleep(50 * time.Millisecond)
	cancel()
	scheduler.Stop()

	reminders := scheduler.GetReminders()
	if len(reminders) != 1 {
		t.Fatalf("expected 1 loaded reminder, got %d", len(reminders))
	}
	if reminders[0].ID != "pre-existing" {
		t.Errorf("expected ID %q, got %q", "pre-existing", reminders[0].ID)
	}
	if reminders[0].Content != "loaded from db" {
		t.Errorf("expected content %q, got %q", "loaded from db", reminders[0].Content)
	}
	_ = ctx
}
