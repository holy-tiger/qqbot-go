package gateway

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewMessageQueue(t *testing.T) {
	q := NewMessageQueue(10, 20)
	if q == nil {
		t.Fatal("NewMessageQueue returned nil")
	}
	q.Stop()
}

func TestEnqueueAndProcess(t *testing.T) {
	q := NewMessageQueue(10, 20)
	q.Start()
	defer q.Stop()

	var processed atomic.Int32
	item := &MessageItem{
		ID:        "msg-1",
		AccountID: "user-1",
		EventType: "C2C",
		Handler: func() error {
			processed.Add(1)
			return nil
		},
	}

	if !q.Enqueue(item) {
		t.Error("Enqueue should succeed")
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	if got := processed.Load(); got != 1 {
		t.Errorf("processed count: got %d, want 1", got)
	}
}

func TestPerUserQueueLimit(t *testing.T) {
	// Use perUserQueueSize=1 so only 1 message per user at a time
	q := NewMessageQueue(10, 1)
	q.Start()
	defer q.Stop()

	// Handler that takes a long time, ensuring count stays at 1
	block := make(chan struct{})

	// Enqueue first message — will block in handler
	enqueued1 := q.Enqueue(&MessageItem{
		ID: "msg-1", AccountID: "user-1", EventType: "C2C",
		Handler: func() error {
			<-block
			return nil
		},
	})
	if !enqueued1 {
		t.Error("first enqueue should succeed")
	}

	// Give the worker time to pick up msg-1
	time.Sleep(100 * time.Millisecond)

	// Second should be rejected because per-user count = 1 (limit)
	enqueued2 := q.Enqueue(&MessageItem{
		ID: "msg-2", AccountID: "user-1", EventType: "C2C",
		Handler: func() error { return nil },
	})
	if enqueued2 {
		t.Error("second enqueue should be rejected (per-user limit = 1)")
	}

	close(block)
}

func TestConcurrentProcessing(t *testing.T) {
	q := NewMessageQueue(100, 10)
	q.Start()
	defer q.Stop()

	var wg sync.WaitGroup
	var count atomic.Int32

	// Enqueue messages for 5 different users concurrently
	for i := 0; i < 5; i++ {
		userID := string(rune('A' + i))
		for j := 0; j < 3; j++ {
			wg.Add(1)
			item := &MessageItem{
				ID:        userID + "-" + string(rune('0'+j)),
				AccountID: userID,
				EventType: "C2C",
				Handler: func() error {
					defer wg.Done()
					time.Sleep(50 * time.Millisecond)
					count.Add(1)
					return nil
				},
			}
			q.Enqueue(item)
		}
	}

	wg.Wait()

	if got := count.Load(); got != 15 {
		t.Errorf("processed: got %d, want 15", got)
	}
}

func TestSameUserSerial(t *testing.T) {
	q := NewMessageQueue(100, 10)
	q.Start()
	defer q.Stop()

	var order []string
	var mu sync.Mutex
	done := make(chan struct{})

	q.Enqueue(&MessageItem{
		ID: "a", AccountID: "user-x", EventType: "C2C",
		Handler: func() error {
			mu.Lock()
			order = append(order, "a-start")
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			order = append(order, "a-end")
			mu.Unlock()
			return nil
		},
	})
	q.Enqueue(&MessageItem{
		ID: "b", AccountID: "user-x", EventType: "C2C",
		Handler: func() error {
			mu.Lock()
			order = append(order, "b-start")
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			order = append(order, "b-end")
			mu.Unlock()
			return nil
		},
	})
	q.Enqueue(&MessageItem{
		ID: "c", AccountID: "user-x", EventType: "C2C",
		Handler: func() error {
			mu.Lock()
			order = append(order, "c-done")
			mu.Unlock()
			close(done)
			return nil
		},
	})

	<-done
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 5 {
		t.Fatalf("expected at least 5 log entries, got %d: %v", len(order), order)
	}

	// "a-end" must come before "b-start" (same user, serialized)
	aEnd := -1
	bStart := -1
	for i, s := range order {
		if s == "a-end" {
			aEnd = i
		}
		if s == "b-start" {
			bStart = i
		}
	}
	if aEnd == -1 || bStart == -1 || aEnd >= bStart {
		t.Errorf("same-user messages not serialized: order=%v", order)
	}
}

func TestQueueStop(t *testing.T) {
	q := NewMessageQueue(100, 10)
	q.Start()
	q.Stop()
	// Enqueue after stop should return false
	if q.Enqueue(&MessageItem{ID: "x", AccountID: "u", EventType: "C2C", Handler: func() error { return nil }}) {
		t.Error("Enqueue after Stop should return false")
	}
}

func TestQueueStats(t *testing.T) {
	q := NewMessageQueue(10, 20)
	q.Start()
	defer q.Stop()

	// No messages yet
	queueLen, activeUsers := q.Stats()
	if queueLen != 0 {
		t.Errorf("initial queueLen: got %d, want 0", queueLen)
	}
	if activeUsers != 0 {
		t.Errorf("initial activeUsers: got %d, want 0", activeUsers)
	}

	// Enqueue a blocking message
	started := make(chan struct{})
	q.Enqueue(&MessageItem{
		ID: "msg-1", AccountID: "user-1", EventType: "C2C",
		Handler: func() error {
			close(started)
			time.Sleep(500 * time.Millisecond)
			return nil
		},
	})

	<-started
	time.Sleep(20 * time.Millisecond)

	queueLen, activeUsers = q.Stats()
	if activeUsers != 1 {
		t.Errorf("activeUsers during processing: got %d, want 1", activeUsers)
	}
}

func TestHandlerError(t *testing.T) {
	q := NewMessageQueue(10, 20)
	q.Start()
	defer q.Stop()

	var processed atomic.Int32
	q.Enqueue(&MessageItem{
		ID: "err-1", AccountID: "u", EventType: "C2C",
		Handler: func() error {
			processed.Add(1)
			return nil // even if handler had error, queue continues
		},
	})

	time.Sleep(200 * time.Millisecond)
	if processed.Load() != 1 {
		t.Error("handler should have been called")
	}
}

func TestDifferentUsersParallel(t *testing.T) {
	q := NewMessageQueue(100, 10)
	q.Start()
	defer q.Stop()

	var mu sync.Mutex
	overlapDetected := false
	barrier := make(chan struct{})

	// Start a slow handler for user-a
	q.Enqueue(&MessageItem{
		ID: "a", AccountID: "user-a", EventType: "C2C",
		Handler: func() error {
			close(barrier)
			time.Sleep(200 * time.Millisecond)
			return nil
		},
	})

	<-barrier

	// Start handler for user-b - should start immediately (different user)
	started := make(chan struct{})
	q.Enqueue(&MessageItem{
		ID: "b", AccountID: "user-b", EventType: "C2C",
		Handler: func() error {
			close(started)
			mu.Lock()
			overlapDetected = true
			mu.Unlock()
			return nil
		},
	})

	select {
	case <-started:
		// Good - user-b started while user-a was still running
	case <-time.After(100 * time.Millisecond):
		t.Error("user-b handler should have started in parallel with user-a")
	}

	mu.Lock()
	if !overlapDetected {
		t.Error("expected overlap between different users")
	}
	mu.Unlock()
}
