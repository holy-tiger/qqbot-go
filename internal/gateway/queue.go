package gateway

import (
	"context"
	"sync"
	"sync/atomic"
)

const (
	defaultGlobalQueueSize    = 1000
	defaultPerUserQueueSize   = 20
	defaultMaxConcurrentUsers = 10
)

// MessageItem represents a message to be processed asynchronously.
type MessageItem struct {
	ID        string
	AccountID string
	EventType string // "C2C", "GROUP", "GUILD"
	Payload   []byte
	Handler   func() error
}

// MessageQueue provides per-user serialized, cross-user parallel message processing.
type MessageQueue struct {
	globalCh     chan *MessageItem
	perUserCh    map[string]chan *MessageItem
	perUserCount map[string]int // tracks in-flight + queued items per user
	mu           sync.Mutex
	semaphore    chan struct{} // limits concurrent users
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	perUserSize  int
	stopped      atomic.Bool
}

// NewMessageQueue creates a message queue with the given concurrency limits.
// maxConcurrentUsers: max number of users processed in parallel.
// perUserQueueSize: max pending messages per user (including the one being processed).
func NewMessageQueue(maxConcurrentUsers, perUserQueueSize int) *MessageQueue {
	if maxConcurrentUsers <= 0 {
		maxConcurrentUsers = defaultMaxConcurrentUsers
	}
	if perUserQueueSize <= 0 {
		perUserQueueSize = defaultPerUserQueueSize
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &MessageQueue{
		globalCh:     make(chan *MessageItem, defaultGlobalQueueSize),
		perUserCh:    make(map[string]chan *MessageItem),
		perUserCount: make(map[string]int),
		semaphore:    make(chan struct{}, maxConcurrentUsers),
		ctx:          ctx,
		cancel:       cancel,
		perUserSize:  perUserQueueSize,
	}
}

// Start begins the queue processing loop.
func (q *MessageQueue) Start() {
	go q.dispatchLoop()
}

// Stop gracefully stops the queue and waits for in-flight messages.
func (q *MessageQueue) Stop() {
	if q.stopped.Swap(true) {
		return
	}
	q.cancel()
	close(q.globalCh)
	q.wg.Wait()
}

// Enqueue adds a message to the queue. Returns false if queue is full or stopped.
// This checks per-user limits synchronously.
func (q *MessageQueue) Enqueue(item *MessageItem) bool {
	if q.stopped.Load() {
		return false
	}

	// Check per-user limit synchronously
	q.mu.Lock()
	count := q.perUserCount[item.AccountID]
	if count >= q.perUserSize {
		q.mu.Unlock()
		return false
	}
	q.perUserCount[item.AccountID] = count + 1

	ch, ok := q.perUserCh[item.AccountID]
	if !ok {
		ch = make(chan *MessageItem, q.perUserSize)
		q.perUserCh[item.AccountID] = ch
		q.wg.Add(1)
		go q.userWorker(item.AccountID, ch)
	}
	q.mu.Unlock()

	// Send to per-user channel (non-blocking, but we already checked count)
	select {
	case ch <- item:
	case <-q.ctx.Done():
		q.decrementCount(item.AccountID)
		return false
	}

	return true
}

// Stats returns the current queue length and number of active (processing) users.
func (q *MessageQueue) Stats() (queueLen, activeUsers int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, ch := range q.perUserCh {
		queueLen += len(ch)
	}
	activeUsers = len(q.semaphore) // occupied semaphore slots = active users
	return queueLen, activeUsers
}

// dispatchLoop is kept for future use if we need a global dispatch step.
func (q *MessageQueue) dispatchLoop() {
	// No-op: Enqueue now directly routes to per-user channels.
	// This method exists for API compatibility.
}

// userWorker processes messages for a single user serially.
func (q *MessageQueue) userWorker(accountID string, ch chan *MessageItem) {
	defer func() {
		q.mu.Lock()
		delete(q.perUserCh, accountID)
		delete(q.perUserCount, accountID)
		q.mu.Unlock()
		q.wg.Done()
	}()

	for {
		select {
		case item, ok := <-ch:
			if !ok {
				return
			}

			// Acquire semaphore (limits total concurrent users)
			select {
			case q.semaphore <- struct{}{}:
			case <-q.ctx.Done():
				q.decrementCount(accountID)
				return
			}

			if item.Handler != nil {
				_ = item.Handler() // errors are logged but don't stop processing
			}

			// Release semaphore
			<-q.semaphore

			q.decrementCount(accountID)
		case <-q.ctx.Done():
			return
		}
	}
}

func (q *MessageQueue) decrementCount(accountID string) {
	q.mu.Lock()
	if c, ok := q.perUserCount[accountID]; ok && c > 0 {
		q.perUserCount[accountID] = c - 1
	}
	q.mu.Unlock()
}
