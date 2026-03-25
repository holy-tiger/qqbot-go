package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/openclaw/qqbot/internal/types"
)

const (
	// APIBase is the base URL for QQ Bot API.
	APIBase = "https://api.sgroup.qq.com"
	// TokenURL is the QQ Bot token endpoint.
	TokenURL = "https://bots.qq.com/app/getAppAccessToken"
	// DefaultAPITimeout is the default timeout for API requests.
	DefaultAPITimeout = 30 * time.Second
	// FileUploadTimeout is the timeout for file upload requests.
	FileUploadTimeout = 120 * time.Second
	// UploadMaxRetries is the maximum number of retries for upload requests.
	UploadMaxRetries = 2
	// UploadBaseDelay is the base delay for upload retry (exponential backoff).
	UploadBaseDelay = 1 * time.Second
)

// APIClient is the QQ Bot API client.
type APIClient struct {
	tokenCache      *TokenCache
	uploadCache     *UploadCache
	httpClient      *http.Client
	markdownSupport bool
	onMessageSent   func(refIdx string, meta types.OutboundMeta)

	appID        string
	clientSecret string
	apiBase      string // overridable for testing
	cancel       context.CancelFunc
	mu           sync.Mutex
}

// ClientOption configures an APIClient.
type ClientOption func(*APIClient)

// WithMarkdownSupport enables or disables markdown message format.
func WithMarkdownSupport(v bool) ClientOption {
	return func(c *APIClient) {
		c.markdownSupport = v
	}
}

// GetMarkdownSupport returns whether markdown message format is enabled.
func (c *APIClient) GetMarkdownSupport() bool {
	return c.markdownSupport
}

// WithMessageSentHook sets a callback invoked when a message is sent with a ref_idx.
func WithMessageSentHook(fn func(string, types.OutboundMeta)) ClientOption {
	return func(c *APIClient) {
		c.onMessageSent = fn
	}
}

// NewAPIClient creates a new APIClient with the given options.
func NewAPIClient(opts ...ClientOption) *APIClient {
	c := &APIClient{
		tokenCache:  NewTokenCache(),
		uploadCache: NewUploadCache(500),
		httpClient: &http.Client{
			Timeout: DefaultAPITimeout,
		},
		apiBase: APIBase,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Init starts background token refresh for the given appId and clientSecret.
func (c *APIClient) Init(_ context.Context, appID, clientSecret string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop any existing background refresh
	if c.cancel != nil {
		c.cancel()
	}

	c.appID = appID
	c.clientSecret = clientSecret

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.tokenCache.tokenURL = TokenURL
	c.tokenCache.StartBackgroundRefresh(ctx, appID, clientSecret)
}

// Close stops the background token refresh.
func (c *APIClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// getAccessToken is a convenience method to get a cached token.
func (c *APIClient) getAccessToken(ctx context.Context) (string, error) {
	return c.tokenCache.GetAccessToken(ctx, c.appID, c.clientSecret)
}

// GetAccessToken returns a valid cached access token for the configured appId.
func (c *APIClient) GetAccessToken(ctx context.Context) (string, error) {
	return c.getAccessToken(ctx)
}

// getNextMsgSeq generates a time-based random sequence number in range 0-65535.
func getNextMsgSeq(_ string) int {
	timePart := time.Now().UnixMilli() % 100000000
	r := rand.Intn(65536)
	return int((timePart ^ int64(r)) % 65536)
}

// doRequest performs an authenticated HTTP request and returns the parsed response.
func (c *APIClient) doRequest(ctx context.Context, method, path string, body interface{}, timeout time.Duration) ([]byte, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := c.apiBase + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error [%s]: status %d: %s", path, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// apiRequestWithRetry performs an HTTP request with exponential backoff retry for uploads.
func (c *APIClient) apiRequestWithRetry(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= UploadMaxRetries; attempt++ {
		respBody, err := c.doRequest(ctx, method, path, body, FileUploadTimeout)
		if err == nil {
			return respBody, nil
		}
		lastErr = err

		// Don't retry on client errors
		errMsg := err.Error()
		if containsAny(errMsg, "400", "401", "Invalid", "timeout", "Timeout") {
			return nil, lastErr
		}

		if attempt < UploadMaxRetries {
			delay := UploadBaseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, lastErr
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

// sendAndNotify sends a message and triggers the onMessageSent hook if response has ref_idx.
func (c *APIClient) sendAndNotify(ctx context.Context, method, path string, body interface{}, meta types.OutboundMeta) (*types.MessageResponse, error) {
	respBody, err := c.doRequest(ctx, method, path, body, DefaultAPITimeout)
	if err != nil {
		return nil, err
	}

	var resp types.MessageResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.ExtInfo != nil && resp.ExtInfo.RefIdx != "" && c.onMessageSent != nil {
		c.onMessageSent(resp.ExtInfo.RefIdx, meta)
	}

	return &resp, nil
}

