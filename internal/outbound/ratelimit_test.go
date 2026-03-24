package outbound

import (
	"fmt"
	"testing"
	"time"
)

func TestReplyLimiter_FreshMessage(t *testing.T) {
	limiter := NewReplyLimiter()

	result := limiter.Check("msg-001")

	if !result.Allowed {
		t.Error("expected allowed for fresh message")
	}
	if result.Remaining != 4 {
		t.Errorf("expected remaining=4, got %d", result.Remaining)
	}
	if result.ShouldFallbackToProactive {
		t.Error("should not fallback for fresh message")
	}
}

func TestReplyLimiter_RecordAndCountDown(t *testing.T) {
	limiter := NewReplyLimiter()

	// Record 1 reply
	limiter.Record("msg-001")
	result := limiter.Check("msg-001")
	if !result.Allowed {
		t.Error("expected allowed after 1 reply")
	}
	if result.Remaining != 3 {
		t.Errorf("expected remaining=3 after 1 reply, got %d", result.Remaining)
	}

	// Record 2 more replies
	limiter.Record("msg-001")
	limiter.Record("msg-001")
	result = limiter.Check("msg-001")
	if !result.Allowed {
		t.Error("expected allowed after 3 replies")
	}
	if result.Remaining != 1 {
		t.Errorf("expected remaining=1 after 3 replies, got %d", result.Remaining)
	}
}

func TestReplyLimiter_LimitExceeded(t *testing.T) {
	limiter := NewReplyLimiter()

	// Record 4 replies (max)
	for i := 0; i < 4; i++ {
		limiter.Record("msg-001")
	}

	result := limiter.Check("msg-001")
	if result.Allowed {
		t.Error("expected not allowed when limit exceeded")
	}
	if result.Remaining != 0 {
		t.Errorf("expected remaining=0, got %d", result.Remaining)
	}
	if !result.ShouldFallbackToProactive {
		t.Error("expected fallback to proactive")
	}
	if result.FallbackReason != "limit_exceeded" {
		t.Errorf("expected fallback reason 'limit_exceeded', got '%s'", result.FallbackReason)
	}
}

func TestReplyLimiter_ExpiredMessage(t *testing.T) {
	limiter := NewReplyLimiter()

	// Manually set an old record to simulate expiration
	now := time.Now().UnixMilli()
	limiter.mu.Lock()
	limiter.tracks["msg-expired"] = &replyRecord{
		count:        2,
		firstReplyAt: now - MessageReplyTTL - 1, // expired
	}
	limiter.mu.Unlock()

	result := limiter.Check("msg-expired")
	if result.Allowed {
		t.Error("expected not allowed for expired message")
	}
	if result.Remaining != 0 {
		t.Errorf("expected remaining=0, got %d", result.Remaining)
	}
	if !result.ShouldFallbackToProactive {
		t.Error("expected fallback to proactive")
	}
	if result.FallbackReason != "expired" {
		t.Errorf("expected fallback reason 'expired', got '%s'", result.FallbackReason)
	}
}

func TestReplyLimiter_ExpiredRecordReset(t *testing.T) {
	limiter := NewReplyLimiter()

	// Set an expired record, then Record should reset it
	now := time.Now().UnixMilli()
	limiter.mu.Lock()
	limiter.tracks["msg-old"] = &replyRecord{
		count:        4,
		firstReplyAt: now - MessageReplyTTL - 1,
	}
	limiter.mu.Unlock()

	// Recording on expired message should reset the count
	limiter.Record("msg-old")

	result := limiter.Check("msg-old")
	if !result.Allowed {
		t.Error("expected allowed after reset of expired record")
	}
	if result.Remaining != 3 {
		t.Errorf("expected remaining=3 after reset, got %d", result.Remaining)
	}
}

func TestReplyLimiter_AutoCleanup(t *testing.T) {
	limiter := NewReplyLimiter()
	now := time.Now().UnixMilli()

	// Fill with expired entries to trigger cleanup threshold
	limiter.mu.Lock()
	for i := 0; i <= 10000; i++ {
		limiter.tracks[fmt.Sprintf("expired-%d", i)] = &replyRecord{
			count:        1,
			firstReplyAt: now - MessageReplyTTL - 1,
		}
	}
	// Add one valid entry
	limiter.tracks["valid-msg"] = &replyRecord{
		count:        2,
		firstReplyAt: now,
	}
	limiter.mu.Unlock()

	// Check should trigger cleanup
	result := limiter.Check("valid-msg")
	if !result.Allowed {
		t.Error("expected valid entry to survive cleanup")
	}

	limiter.mu.Lock()
	count := len(limiter.tracks)
	limiter.mu.Unlock()
	if count > 2 {
		t.Errorf("expected cleanup to reduce tracks, got %d entries", count)
	}
}

func TestReplyLimiter_Clear(t *testing.T) {
	limiter := NewReplyLimiter()

	limiter.Record("msg-001")
	limiter.Record("msg-002")

	limiter.Clear()

	result := limiter.Check("msg-001")
	if !result.Allowed {
		t.Error("expected fresh result after clear")
	}
	if result.Remaining != 4 {
		t.Errorf("expected remaining=4 after clear, got %d", result.Remaining)
	}
}

func TestReplyLimiter_DifferentMessageIDs(t *testing.T) {
	limiter := NewReplyLimiter()

	limiter.Record("msg-001")

	result1 := limiter.Check("msg-001")
	result2 := limiter.Check("msg-002")

	if result1.Remaining != 3 {
		t.Errorf("msg-001 remaining=3, got %d", result1.Remaining)
	}
	if result2.Remaining != 4 {
		t.Errorf("msg-002 remaining=4, got %d", result2.Remaining)
	}
}

func TestReplyLimiter_FallbackMessage(t *testing.T) {
	limiter := NewReplyLimiter()

	// Test limit_exceeded message
	for i := 0; i < 4; i++ {
		limiter.Record("msg-limit")
	}
	result := limiter.Check("msg-limit")
	if result.Message == "" {
		t.Error("expected fallback message for limit exceeded")
	}

	// Test expired message
	now := time.Now().UnixMilli()
	limiter.mu.Lock()
	limiter.tracks["msg-exp"] = &replyRecord{
		count:        1,
		firstReplyAt: now - MessageReplyTTL - 1,
	}
	limiter.mu.Unlock()

	result = limiter.Check("msg-exp")
	if result.Message == "" {
		t.Error("expected fallback message for expired")
	}
}
