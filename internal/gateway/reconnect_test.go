package gateway

import (
	"testing"
	"time"
)

func TestDefaultReconnectConfig(t *testing.T) {
	cfg := DefaultReconnectConfig

	if len(cfg.Delays) != 6 {
		t.Fatalf("expected 6 delays, got %d", len(cfg.Delays))
	}

	expectedDelays := []time.Duration{
		1 * time.Second, 2 * time.Second, 5 * time.Second,
		10 * time.Second, 30 * time.Second, 60 * time.Second,
	}
	for i, d := range cfg.Delays {
		if d != expectedDelays[i] {
			t.Errorf("delay[%d]: got %v, want %v", i, d, expectedDelays[i])
		}
	}

	if cfg.RateLimitDelay != 60*time.Second {
		t.Errorf("RateLimitDelay: got %v, want 60s", cfg.RateLimitDelay)
	}
	if cfg.MaxAttempts != 100 {
		t.Errorf("MaxAttempts: got %d, want 100", cfg.MaxAttempts)
	}
	if cfg.MaxQuickDisconnectCount != 3 {
		t.Errorf("MaxQuickDisconnectCount: got %d, want 3", cfg.MaxQuickDisconnectCount)
	}
	if cfg.QuickDisconnectThreshold != 5*time.Second {
		t.Errorf("QuickDisconnectThreshold: got %v, want 5s", cfg.QuickDisconnectThreshold)
	}
}

func TestGetDelay(t *testing.T) {
	cfg := DefaultReconnectConfig

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 5 * time.Second},
		{3, 10 * time.Second},
		{4, 30 * time.Second},
		{5, 60 * time.Second},
		{6, 60 * time.Second}, // beyond array length, use last
		{99, 60 * time.Second},
		{-1, 1 * time.Second}, // negative, use first
	}

	for _, tt := range tests {
		got := cfg.GetDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("GetDelay(%d): got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestGetDelayCustomConfig(t *testing.T) {
	cfg := ReconnectConfig{
		Delays: []time.Duration{500 * time.Millisecond, 1 * time.Second},
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 500 * time.Millisecond},
		{1, 1 * time.Second},
		{2, 1 * time.Second}, // capped at last
	}

	for _, tt := range tests {
		got := cfg.GetDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("GetDelay(%d): got %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestShouldQuickStop(t *testing.T) {
	cfg := DefaultReconnectConfig
	now := time.Now()

	// No disconnects at all
	if cfg.ShouldQuickStop(nil) {
		t.Error("should not quick stop with no disconnects")
	}
	if cfg.ShouldQuickStop([]time.Time{}) {
		t.Error("should not quick stop with empty slice")
	}

	// Fewer than threshold quick disconnects
	times := []time.Time{
		now.Add(-1 * time.Second),
		now.Add(-2 * time.Second),
	}
	if cfg.ShouldQuickStop(times) {
		t.Error("should not quick stop with 2 quick disconnects (threshold is 3)")
	}

	// Exactly threshold quick disconnects
	times = []time.Time{
		now.Add(-1 * time.Second),
		now.Add(-2 * time.Second),
		now.Add(-3 * time.Second),
	}
	if !cfg.ShouldQuickStop(times) {
		t.Error("should quick stop with 3 quick disconnects within threshold")
	}

	// More than threshold
	times = []time.Time{
		now.Add(-500 * time.Millisecond),
		now.Add(-1 * time.Second),
		now.Add(-2 * time.Second),
		now.Add(-4 * time.Second),
	}
	if !cfg.ShouldQuickStop(times) {
		t.Error("should quick stop with 4 quick disconnects within threshold")
	}

	// Disconnects outside threshold window — only 1 is recent
	times = []time.Time{
		now.Add(-10 * time.Second), // too old
		now.Add(-2 * time.Second),
		now.Add(-1 * time.Second),
	}
	if cfg.ShouldQuickStop(times) {
		t.Error("should not quick stop when only 2 are within threshold")
	}
}

func TestShouldQuickStopExactThreshold(t *testing.T) {
	cfg := DefaultReconnectConfig
	now := time.Now()

	// At exactly the threshold boundary (5s), should still count
	// Use -4.9s to avoid race with time.Now() inside ShouldQuickStop
	times := []time.Time{
		now.Add(-4900 * time.Millisecond),
		now.Add(-4 * time.Second),
		now.Add(-3 * time.Second),
	}
	if !cfg.ShouldQuickStop(times) {
		t.Error("should quick stop when disconnects are within threshold boundary")
	}

	// Just outside threshold (5001ms)
	times = []time.Time{
		now.Add(-5001 * time.Millisecond),
		now.Add(-4 * time.Second),
		now.Add(-3 * time.Second),
	}
	if cfg.ShouldQuickStop(times) {
		t.Error("should not quick stop when oldest disconnect is just outside threshold")
	}
}

func TestReconnectConfigZeroLength(t *testing.T) {
	cfg := ReconnectConfig{Delays: []time.Duration{}}
	got := cfg.GetDelay(0)
	if got != 0 {
		t.Errorf("GetDelay with empty delays: got %v, want 0", got)
	}
}
