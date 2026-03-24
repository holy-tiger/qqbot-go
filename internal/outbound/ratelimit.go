package outbound

import (
	"fmt"
	"sync"
	"time"
)

const (
	// MessageReplyLimit is the maximum number of passive replies per message_id per hour.
	MessageReplyLimit = 4
	// MessageReplyTTL is the time-to-live for a message_id reply window (1 hour in milliseconds).
	MessageReplyTTL = 60 * 60 * 1000
)

// ReplyLimitResult holds the result of a reply rate limit check.
type ReplyLimitResult struct {
	Allowed               bool
	Remaining             int
	ShouldFallbackToProactive bool
	FallbackReason        string // "expired" or "limit_exceeded"
	Message               string
}

type replyRecord struct {
	count        int
	firstReplyAt int64 // unix ms
}

// ReplyLimiter tracks per-messageID reply counts to enforce QQ Bot rate limits.
type ReplyLimiter struct {
	mu     sync.Mutex
	tracks map[string]*replyRecord
}

// NewReplyLimiter creates a new ReplyLimiter.
func NewReplyLimiter() *ReplyLimiter {
	return &ReplyLimiter{
		tracks: make(map[string]*replyRecord),
	}
}

// Check checks whether a passive reply is allowed for the given messageID.
func (r *ReplyLimiter) Check(messageID string) ReplyLimitResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()

	// Auto-cleanup when tracks grow large
	if len(r.tracks) > 10000 {
		for id, rec := range r.tracks {
			if now-rec.firstReplyAt > MessageReplyTTL {
				delete(r.tracks, id)
			}
		}
	}

	record, exists := r.tracks[messageID]
	if !exists {
		return ReplyLimitResult{
			Allowed:    true,
			Remaining:  MessageReplyLimit,
		}
	}

	// Check if expired (>1 hour since first reply)
	if now-record.firstReplyAt > MessageReplyTTL {
		return ReplyLimitResult{
			Allowed:               false,
			Remaining:             0,
			ShouldFallbackToProactive: true,
			FallbackReason:        "expired",
			Message:               "message expired beyond 1 hour, falling back to proactive message",
		}
	}

	remaining := MessageReplyLimit - record.count
	if remaining <= 0 {
		return ReplyLimitResult{
			Allowed:               false,
			Remaining:             0,
			ShouldFallbackToProactive: true,
			FallbackReason:        "limit_exceeded",
			Message:               fmt.Sprintf("message reached max reply limit (%d) within 1 hour, falling back to proactive message", MessageReplyLimit),
		}
	}

	return ReplyLimitResult{
		Allowed:   true,
		Remaining: remaining,
	}
}

// Record records a reply for the given messageID. If the record is expired, it resets the count.
func (r *ReplyLimiter) Record(messageID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	record, exists := r.tracks[messageID]
	if !exists {
		r.tracks[messageID] = &replyRecord{count: 1, firstReplyAt: now}
		return
	}

	// If expired, reset the record
	if now-record.firstReplyAt > MessageReplyTTL {
		r.tracks[messageID] = &replyRecord{count: 1, firstReplyAt: now}
		return
	}

	record.count++
}

// Clear removes all tracked records.
func (r *ReplyLimiter) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracks = make(map[string]*replyRecord)
}
