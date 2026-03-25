package store

import (
	"testing"
	"time"
)

func TestReminderStore_SaveAndGet(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	job := ReminderRecord{
		ID:            "rem-1",
		Content:       "test reminder",
		TargetType:    "c2c",
		TargetAddress: "user1",
		AccountID:     "acct1",
		Schedule:      "@every 30m",
		NextRun:       time.Now().Add(30 * time.Minute),
		CreatedAt:     time.Now(),
	}

	rs.Save(job)

	got := rs.Get("rem-1")
	if got == nil {
		t.Fatal("expected non-nil reminder")
	}
	if got.Content != "test reminder" {
		t.Errorf("expected content %q, got %q", "test reminder", got.Content)
	}
	if got.TargetType != "c2c" {
		t.Errorf("expected target_type %q, got %q", "c2c", got.TargetType)
	}
	if got.AccountID != "acct1" {
		t.Errorf("expected account_id %q, got %q", "acct1", got.AccountID)
	}
	if got.Schedule != "@every 30m" {
		t.Errorf("expected schedule %q, got %q", "@every 30m", got.Schedule)
	}
}

func TestReminderStore_Get_NotFound(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	got := rs.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent reminder")
	}
}

func TestReminderStore_Delete(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	rs.Save(ReminderRecord{
		ID: "rem-1", Content: "test", AccountID: "acct1",
		TargetType: "c2c", TargetAddress: "u1",
		NextRun: time.Now().Add(1 * time.Hour), CreatedAt: time.Now(),
	})

	if !rs.Delete("rem-1") {
		t.Error("expected Delete to return true")
	}
	if rs.Get("rem-1") != nil {
		t.Error("expected nil after delete")
	}
	if rs.Delete("rem-1") {
		t.Error("expected Delete to return false for nonexistent")
	}
}

func TestReminderStore_ListByAccount(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	rs.Save(ReminderRecord{
		ID: "rem-1", AccountID: "acct1", Content: "a",
		TargetType: "c2c", TargetAddress: "u1",
		NextRun: time.Now().Add(1 * time.Hour), CreatedAt: time.Now(),
	})
	rs.Save(ReminderRecord{
		ID: "rem-2", AccountID: "acct1", Content: "b",
		TargetType: "group", TargetAddress: "g1",
		NextRun: time.Now().Add(2 * time.Hour), CreatedAt: time.Now(),
	})
	rs.Save(ReminderRecord{
		ID: "rem-3", AccountID: "acct2", Content: "c",
		TargetType: "c2c", TargetAddress: "u2",
		NextRun: time.Now().Add(1 * time.Hour), CreatedAt: time.Now(),
	})

	jobs := rs.ListByAccount("acct1")
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs for acct1, got %d", len(jobs))
	}

	jobs2 := rs.ListByAccount("acct2")
	if len(jobs2) != 1 {
		t.Fatalf("expected 1 job for acct2, got %d", len(jobs2))
	}
}

func TestReminderStore_ListAll(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	rs.Save(ReminderRecord{
		ID: "rem-1", AccountID: "a1", Content: "x",
		TargetType: "c2c", TargetAddress: "u1",
		NextRun: time.Now(), CreatedAt: time.Now(),
	})
	rs.Save(ReminderRecord{
		ID: "rem-2", AccountID: "a2", Content: "y",
		TargetType: "c2c", TargetAddress: "u2",
		NextRun: time.Now(), CreatedAt: time.Now(),
	})

	all := rs.ListAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 total jobs, got %d", len(all))
	}
}

func TestReminderStore_Clear(t *testing.T) {
	db := OpenTestDB(t)
	rs := NewReminderStore(db)

	rs.Save(ReminderRecord{
		ID: "rem-1", AccountID: "acct1", Content: "x",
		TargetType: "c2c", TargetAddress: "u1",
		NextRun: time.Now(), CreatedAt: time.Now(),
	})
	rs.Save(ReminderRecord{
		ID: "rem-2", AccountID: "acct2", Content: "y",
		TargetType: "c2c", TargetAddress: "u2",
		NextRun: time.Now(), CreatedAt: time.Now(),
	})

	n := rs.Clear("acct1")
	if n != 1 {
		t.Errorf("expected 1 cleared, got %d", n)
	}
	if len(rs.ListAll()) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(rs.ListAll()))
	}
}
