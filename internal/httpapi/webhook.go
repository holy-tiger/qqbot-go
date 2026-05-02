package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	webhookMaxRetries  = 3
	webhookTimeout     = 10 * time.Second
	webhookMaxInFlight = 16 // P2-14: max concurrent webhook deliveries
)

// WebhookEvent is the JSON payload sent to the webhook URL.
type WebhookEvent struct {
	AccountID string          `json:"account_id"`
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// WebhookDispatcher forwards gateway events to configured webhook URLs.
type WebhookDispatcher struct {
	urls    map[string]string // accountID -> webhookURL
	mu      sync.RWMutex
	client  *http.Client
	sem     chan struct{} // P2-14: semaphore to limit concurrent goroutines
}

// NewWebhookDispatcher creates a new WebhookDispatcher.
func NewWebhookDispatcher() *WebhookDispatcher {
	return &WebhookDispatcher{
		urls: make(map[string]string),
		client: &http.Client{
			Timeout: webhookTimeout,
		},
		sem: make(chan struct{}, webhookMaxInFlight),
	}
}

// SetURL registers a webhook URL for an account.
func (d *WebhookDispatcher) SetURL(accountID, url string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.urls[accountID] = url
}

// Dispatch sends the event payload to the configured webhook URL asynchronously.
// If no URL is configured for the account, this is a no-op.
// P2-14: bounded concurrency via semaphore.
func (d *WebhookDispatcher) Dispatch(accountID, eventType string, payload []byte) {
	d.mu.RLock()
	url, ok := d.urls[accountID]
	d.mu.RUnlock()
	if !ok || url == "" {
		return
	}

	event := WebhookEvent{
		AccountID: accountID,
		EventType: eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      json.RawMessage(payload),
	}

	select {
	case d.sem <- struct{}{}:
		go func() {
			defer func() { <-d.sem }()
			d.send(url, event)
		}()
	default:
		// P2-14: drop event if too many in-flight deliveries
		log.Printf("[webhook] dropping %s for %s: too many in-flight deliveries", event.EventType, url)
	}
}

// send delivers the event with bounded retry.
func (d *WebhookDispatcher) send(url string, event WebhookEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("[webhook] marshal error: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout*time.Duration(webhookMaxRetries))
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < webhookMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				log.Printf("[webhook] context cancelled during retry for %s to %s", event.EventType, url)
				return
			case <-time.After(time.Duration(1<<uint(attempt-1)) * time.Second):
			}
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 400 {
			log.Printf("[webhook] delivered %s to %s (status %d)", event.EventType, url, resp.StatusCode)
			return
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	log.Printf("[webhook] failed to deliver %s to %s after %d attempts: %v", event.EventType, url, webhookMaxRetries, lastErr)
}
