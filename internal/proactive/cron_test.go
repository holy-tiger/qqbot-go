package proactive

import (
	"testing"
	"time"
)

func TestCalculateNextRun_EveryDuration(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		schedule string
		wantDiff time.Duration
	}{
		{"every 30s", "@every 30s", 30 * time.Second},
		{"every 1m", "@every 1m", 1 * time.Minute},
		{"every 30m", "@every 30m", 30 * time.Minute},
		{"every 1h", "@every 1h", 1 * time.Hour},
		{"every 2h", "@every 2h", 2 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := CalculateNextRun(tc.schedule, now)
			diff := next.Sub(now)
			if diff != tc.wantDiff {
				t.Errorf("expected %v, got %v", tc.wantDiff, diff)
			}
		})
	}
}

func TestCalculateNextRun_EmptySchedule(t *testing.T) {
	now := time.Now()
	next := CalculateNextRun("", now)
	if !next.IsZero() {
		t.Error("expected zero time for empty schedule")
	}
}

func TestCalculateNextRun_CronEveryMinute(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 30, 0, time.UTC)
	next := CalculateNextRun("* * * * *", now)

	expected := time.Date(2026, 3, 25, 10, 1, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronSpecificMinute(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := CalculateNextRun("30 * * * *", now)

	expected := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronHourly(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	next := CalculateNextRun("0 * * * *", now)

	expected := time.Date(2026, 3, 25, 11, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronDaily(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	next := CalculateNextRun("0 9 * * *", now)

	expected := time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronInterval(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := CalculateNextRun("*/15 * * * *", now)

	expected := time.Date(2026, 3, 25, 10, 15, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronHourInterval(t *testing.T) {
	now := time.Date(2026, 3, 25, 10, 30, 0, 0, time.UTC)
	next := CalculateNextRun("0 */2 * * *", now)

	expected := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronWeekday(t *testing.T) {
	// 2026-03-25 is Wednesday. "0 9 * * 1" means Monday 9:00.
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := CalculateNextRun("0 9 * * 1", now)

	// Next Monday is March 30
	expected := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestCalculateNextRun_CronSameTime(t *testing.T) {
	// If now matches the cron expression, next should be the next occurrence.
	now := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	next := CalculateNextRun("0 10 * * *", now)

	expected := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected next day %v, got %v", expected, next)
	}
}
