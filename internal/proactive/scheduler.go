package proactive

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ReminderJob represents a scheduled reminder.
type ReminderJob struct {
	ID            string
	Content       string
	TargetType    string // "c2c" or "group"
	TargetAddress string // openid or group openid
	AccountID     string
	Schedule      string // cron expression or "@every Xs" or "@every Xm"
	NextRun       time.Time
	CreatedAt     time.Time
}

// checkInterval is how often the scheduler checks for due jobs.
const checkInterval = 100 * time.Millisecond

// Scheduler runs reminder jobs at their scheduled times.
type Scheduler struct {
	mu      sync.Mutex
	jobs    map[string]*ReminderJob
	manager *ProactiveManager
	store   ReminderPersister
	cancel  context.CancelFunc
}

// ReminderPersister defines the interface for persisting reminders.
type ReminderPersister interface {
	Save(job ReminderJob)
	Get(id string) *ReminderJob
	Delete(id string) bool
	ListByAccount(accountID string) []ReminderJob
	ListAll() []ReminderJob
}

// NewScheduler creates a new Scheduler.
func NewScheduler(manager *ProactiveManager) *Scheduler {
	return &Scheduler{
		jobs:    make(map[string]*ReminderJob),
		manager: manager,
	}
}

// Start begins checking for due jobs. Loads persisted reminders on first start.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = cancel

	// Load persisted jobs on first start
	if s.store != nil && len(s.jobs) == 0 {
		for _, job := range s.store.ListAll() {
			j := job
			s.jobs[j.ID] = &j
		}
	}
	s.mu.Unlock()

	go s.run(ctx)
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.mu.Unlock()
}

// AddReminder adds a reminder job and returns its ID.
func (s *Scheduler) AddReminder(job ReminderJob) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.ID == "" {
		job.ID = fmt.Sprintf("rem-%d", time.Now().UnixNano())
	}
	s.jobs[job.ID] = &job
	if s.store != nil {
		s.store.Save(job)
	}
	return job.ID
}

// SetStore sets the persistence backend for reminders.
func (s *Scheduler) SetStore(store ReminderPersister) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
}

// CancelReminder removes a job by ID. Returns true if found.
func (s *Scheduler) CancelReminder(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[jobID]; !ok {
		return false
	}
	delete(s.jobs, jobID)
	if s.store != nil {
		s.store.Delete(jobID)
	}
	return true
}

// GetReminders returns all current reminder jobs.
func (s *Scheduler) GetReminders() []ReminderJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ReminderJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		result = append(result, *j)
	}
	return result
}

// run is the main scheduler loop.
func (s *Scheduler) run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkDue(ctx)
		}
	}
}

// checkDue sends all jobs whose NextRun has passed.
func (s *Scheduler) checkDue(ctx context.Context) {
	now := time.Now()

	s.mu.Lock()
	var due []*ReminderJob
	for _, j := range s.jobs {
		if !j.NextRun.IsZero() && !j.NextRun.After(now) {
			due = append(due, j)
		}
	}
	s.mu.Unlock()

	for _, j := range due {
		s.executeJob(ctx, j)
		s.mu.Lock()
		if job, ok := s.jobs[j.ID]; ok {
			next := CalculateNextRun(job.Schedule, now)
			if next.IsZero() {
				// Non-recurring: remove the job
				delete(s.jobs, j.ID)
			} else {
				job.NextRun = next
			}
		}
		s.mu.Unlock()
	}
}

func (s *Scheduler) executeJob(ctx context.Context, j *ReminderJob) {
	var err error
	switch j.TargetType {
	case "c2c":
		err = s.manager.SendC2C(ctx, j.TargetAddress, j.Content)
	case "group":
		err = s.manager.SendGroup(ctx, j.TargetAddress, j.Content)
	default:
		log.Printf("[scheduler] unknown target type %q for job %s", j.TargetType, j.ID)
		return
	}
	if err != nil {
		log.Printf("[scheduler] failed to send job %s: %v", j.ID, err)
	}
}

// CalculateNextRun computes the next run time based on the schedule string.
// Supports "@every Xs", "@every Xm", "@every Xh" and simple "0 * * * * *" (6-field cron).
func CalculateNextRun(schedule string, after time.Time) time.Time {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return time.Time{} // no recurrence
	}

	if strings.HasPrefix(schedule, "@every ") {
		dur, err := parseDuration(schedule[len("@every "):])
		if err != nil {
			return time.Time{}
		}
		return after.Add(dur)
	}

	// For raw cron expressions, use the cron parser.
	return parseCronExpression(schedule, after)
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	return time.ParseDuration(s)
}
