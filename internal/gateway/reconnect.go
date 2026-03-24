package gateway

import "time"

// ReconnectConfig controls the reconnection behavior after a WebSocket disconnect.
type ReconnectConfig struct {
	Delays                  []time.Duration // exponential backoff delays [1s, 2s, 5s, 10s, 30s, 60s]
	RateLimitDelay          time.Duration   // delay when rate limited (60s)
	MaxAttempts             int             // max reconnect attempts (100)
	MaxQuickDisconnectCount int             // consecutive quick disconnects to trigger stop (3)
	QuickDisconnectThreshold time.Duration  // duration threshold for "quick" disconnect (5s)
}

// DefaultReconnectConfig is the recommended reconnection configuration.
var DefaultReconnectConfig = ReconnectConfig{
	Delays: []time.Duration{
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		60 * time.Second,
	},
	RateLimitDelay:          60 * time.Second,
	MaxAttempts:             100,
	MaxQuickDisconnectCount: 3,
	QuickDisconnectThreshold: 5 * time.Second,
}

// GetDelay returns the reconnection delay for the given attempt number.
// If attempt exceeds the delay list, the last delay is used.
func (c *ReconnectConfig) GetDelay(attempt int) time.Duration {
	if len(c.Delays) == 0 {
		return 0
	}
	if attempt < 0 {
		return c.Delays[0]
	}
	if attempt >= len(c.Delays) {
		return c.Delays[len(c.Delays)-1]
	}
	return c.Delays[attempt]
}

// ShouldQuickStop returns true if there have been enough consecutive
// quick disconnects (within QuickDisconnectThreshold) to warrant stopping.
func (c *ReconnectConfig) ShouldQuickStop(disconnectTimes []time.Time) bool {
	if len(disconnectTimes) < c.MaxQuickDisconnectCount {
		return false
	}

	now := time.Now()
	cutoff := now.Add(-c.QuickDisconnectThreshold)
	count := 0
	for _, ts := range disconnectTimes {
		if !ts.Before(cutoff) { // inclusive: exactly at threshold counts
			count++
		}
	}
	return count >= c.MaxQuickDisconnectCount
}
