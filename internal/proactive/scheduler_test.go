package proactive

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

// trackingSender counts calls for scheduler tests.
type trackingSender struct {
	c2cCount   atomic.Int32
	groupCount atomic.Int32
	c2cTargets []string
	mu         sync.Mutex
}

func (s *trackingSender) SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error) {
	s.c2cCount.Add(1)
	s.mu.Lock()
	s.c2cTargets = append(s.c2cTargets, openid)
	s.mu.Unlock()
	return types.MessageResponse{ID: "msg1", Timestamp: "now"}, nil
}

func (s *trackingSender) SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
	s.groupCount.Add(1)
	return types.MessageResponse{ID: "msg1", Timestamp: "now"}, nil
}

// TestNewScheduler tests constructor.
func TestNewScheduler(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
}

// TestAddReminder tests adding a reminder job.
func TestAddReminder(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	job := ReminderJob{
		ID:            "job1",
		Content:       "hello",
		TargetType:    "c2c",
		TargetAddress: "user1",
		AccountID:     "acct1",
		Schedule:      "0 * * * * *",
		NextRun:       time.Now().Add(1 * time.Hour),
		CreatedAt:     time.Now(),
	}

	id := s.AddReminder(job)
	if id != "job1" {
		t.Errorf("expected job1, got %s", id)
	}

	reminders := s.GetReminders()
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	if reminders[0].ID != "job1" {
		t.Errorf("expected ID job1, got %s", reminders[0].ID)
	}
}

// TestCancelReminder tests canceling a reminder job.
func TestCancelReminder(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	job := ReminderJob{
		ID:            "job1",
		Content:       "hello",
		TargetType:    "c2c",
		TargetAddress: "user1",
		NextRun:       time.Now().Add(1 * time.Hour),
		CreatedAt:     time.Now(),
	}
	s.AddReminder(job)

	ok := s.CancelReminder("job1")
	if !ok {
		t.Error("expected cancel to return true")
	}

	if len(s.GetReminders()) != 0 {
		t.Errorf("expected 0 reminders after cancel, got %d", len(s.GetReminders()))
	}

	// Cancel non-existent should return false
	ok = s.CancelReminder("nonexistent")
	if ok {
		t.Error("expected cancel of nonexistent to return false")
	}
}

// TestGetReminders tests listing reminders.
func TestGetReminders(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	s.AddReminder(ReminderJob{ID: "j1", Content: "a", TargetType: "c2c", TargetAddress: "u1", NextRun: time.Now().Add(1 * time.Hour), CreatedAt: time.Now()})
	s.AddReminder(ReminderJob{ID: "j2", Content: "b", TargetType: "group", TargetAddress: "g1", NextRun: time.Now().Add(2 * time.Hour), CreatedAt: time.Now()})

	reminders := s.GetReminders()
	if len(reminders) != 2 {
		t.Fatalf("expected 2 reminders, got %d", len(reminders))
	}
}

// TestSchedulerDueReminder tests that due reminders are executed.
func TestSchedulerDueReminder(t *testing.T) {
	sender := &trackingSender{}
	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	// Add a job that is already due
	s.AddReminder(ReminderJob{
		ID:            "due1",
		Content:       "reminder!",
		TargetType:    "c2c",
		TargetAddress: "user1",
		NextRun:       time.Now().Add(-1 * time.Second), // already past
		CreatedAt:     time.Now(),
	})

	// Start the scheduler with a short check interval
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Wait for the scheduler to process the due job
	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Stop()

	if sender.c2cCount.Load() != 1 {
		t.Errorf("expected 1 C2C send, got %d", sender.c2cCount.Load())
	}

	sender.mu.Lock()
	if len(sender.c2cTargets) != 1 || sender.c2cTargets[0] != "user1" {
		t.Errorf("expected send to user1, got %v", sender.c2cTargets)
	}
	sender.mu.Unlock()
}

// TestSchedulerNotDueReminder tests that future reminders are not sent prematurely.
func TestSchedulerNotDueReminder(t *testing.T) {
	sender := &trackingSender{}
	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	// Add a job far in the future
	s.AddReminder(ReminderJob{
		ID:            "future1",
		Content:       "later",
		TargetType:    "c2c",
		TargetAddress: "user1",
		NextRun:       time.Now().Add(5 * time.Minute),
		CreatedAt:     time.Now(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Stop()

	if sender.c2cCount.Load() != 0 {
		t.Errorf("expected 0 sends for future reminder, got %d", sender.c2cCount.Load())
	}
}

// TestSchedulerGroupReminder tests group reminder execution.
func TestSchedulerGroupReminder(t *testing.T) {
	sender := &trackingSender{}
	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	s.AddReminder(ReminderJob{
		ID:            "group1",
		Content:       "group reminder",
		TargetType:    "group",
		TargetAddress: "grp1",
		NextRun:       time.Now().Add(-1 * time.Second),
		CreatedAt:     time.Now(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Stop()

	if sender.groupCount.Load() != 1 {
		t.Errorf("expected 1 group send, got %d", sender.groupCount.Load())
	}
}

// TestSchedulerStop tests that Stop cancels the scheduler.
func TestSchedulerStop(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Stop should not panic or hang
	s.Stop()
	cancel()
}

// TestSchedulerRecurringReminder tests that a recurring schedule recalculates next run.
func TestSchedulerRecurringReminder(t *testing.T) {
	sender := &trackingSender{}
	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	// Use a simple cron-like schedule: "every 1 second"
	// We'll test this by using NextRun set to now and a schedule that indicates recurring.
	// For simplicity, we set Schedule to "@every 1s" format.
	now := time.Now()
	s.AddReminder(ReminderJob{
		ID:            "recur1",
		Content:       "recurring",
		TargetType:    "c2c",
		TargetAddress: "user1",
		Schedule:      "@every 1s",
		NextRun:       now.Add(-1 * time.Second),
		CreatedAt:     now,
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Wait for at least 2 executions
	time.Sleep(2500 * time.Millisecond)
	cancel()
	s.Stop()

	count := sender.c2cCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 sends for recurring reminder, got %d", count)
	}
}

// TestSchedulerConcurrentAccess tests thread safety.
func TestSchedulerConcurrentAccess(t *testing.T) {
	mgr := NewProactiveManager(&mockSender{}, store.NewKnownUsersStore(t.TempDir()))
	s := NewScheduler(mgr)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			job := ReminderJob{
				ID:            fmt.Sprintf("job-%d", i),
				Content:       "test",
				TargetType:    "c2c",
				TargetAddress: "u1",
				NextRun:       time.Now().Add(time.Duration(i) * time.Minute),
				CreatedAt:     time.Now(),
			}
			s.AddReminder(job)
		}(i)
	}

	wg.Wait()

	if len(s.GetReminders()) != 100 {
		t.Errorf("expected 100 reminders, got %d", len(s.GetReminders()))
	}
}
