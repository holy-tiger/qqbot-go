package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// TokenURL is the default QQ Bot token endpoint.
	defaultTokenURL = "https://bots.qq.com/app/getAppAccessToken"
	// earlyExpiry is how long before actual expiry we consider the token expired (5 minutes).
	earlyExpiry = 5 * time.Minute
	// refreshAhead is how long before expiry to start background refresh.
	refreshAhead = 5 * time.Minute
	// minRefreshInterval is the minimum interval between background refresh attempts.
	minRefreshInterval = 60 * time.Second
	// retryDelay is the delay after a failed background refresh.
	retryDelay = 5 * time.Second
)

// flexNumber is a JSON number that can be unmarshaled from either a string or a number.
type flexNumber int

func (f *flexNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*f = 0
		return nil
	}
	// Try as string first
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
			return fmt.Errorf("flexNumber: cannot parse %q as int", s)
		}
		*f = flexNumber(n)
		return nil
	}
	// Try as number
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = flexNumber(n)
	return nil
}

// TokenStatus represents the status of a cached token.
type TokenStatus struct {
	Status    string     `json:"status"` // "valid", "expired", "refreshing", "none"
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// tokenEntry stores a cached token.
type tokenEntry struct {
	token     string
	expiresAt time.Time
	appID     string
}

// TokenCache manages access tokens with caching, singleflight dedup, and background refresh.
type TokenCache struct {
	mu    sync.RWMutex
	cache map[string]*tokenEntry
	sf    singleflight.Group

	// tokenURL is the token endpoint URL. Overridable for testing.
	tokenURL string
}

// NewTokenCache creates a new TokenCache.
func NewTokenCache() *TokenCache {
	return &TokenCache{
		cache:    make(map[string]*tokenEntry),
		tokenURL: defaultTokenURL,
	}
}

// GetAccessToken returns a valid access token for the given appId.
// It uses singleflight to deduplicate concurrent requests for the same appId.
func (c *TokenCache) GetAccessToken(ctx context.Context, appID, clientSecret string) (string, error) {
	appID = normalizeAppID(appID)

	// Check cache
	c.mu.RLock()
	entry, ok := c.cache[appID]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt.Add(-earlyExpiry)) {
		return entry.token, nil
	}

	// Singleflight: share concurrent requests for same appId
	v, err, _ := c.sf.Do(appID, func() (interface{}, error) {
		return c.fetchToken(ctx, appID, clientSecret)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *TokenCache) fetchToken(ctx context.Context, appID, clientSecret string) (string, error) {
	// Double-check cache after acquiring singleflight
	c.mu.RLock()
	entry, ok := c.cache[appID]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt.Add(-earlyExpiry)) {
		return entry.token, nil
	}

	reqBody := struct {
		AppID        string `json:"appId"`
		ClientSecret string `json:"clientSecret"`
	}{
		AppID:        appID,
		ClientSecret: clientSecret,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal token request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.tokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		AccessToken string     `json:"access_token"`
		ExpiresIn   flexNumber `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response: %s", string(respBody))
	}

	expiresIn := int(tokenResp.ExpiresIn)
	if expiresIn <= 0 {
		expiresIn = 7200
	}

	c.mu.Lock()
	c.cache[appID] = &tokenEntry{
		token:     tokenResp.AccessToken,
		expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
		appID:     appID,
	}
	c.mu.Unlock()

	return tokenResp.AccessToken, nil
}

// Clear removes the cached token for the given appId.
func (c *TokenCache) Clear(appID string) {
	appID = normalizeAppID(appID)
	c.mu.Lock()
	delete(c.cache, appID)
	c.mu.Unlock()
}

// ClearAll removes all cached tokens.
func (c *TokenCache) ClearAll() {
	c.mu.Lock()
	c.cache = make(map[string]*tokenEntry)
	c.mu.Unlock()
}

// GetStatus returns the current status of the cached token for the given appId.
func (c *TokenCache) GetStatus(appID string) TokenStatus {
	appID = normalizeAppID(appID)
	c.mu.RLock()
	entry, ok := c.cache[appID]
	c.mu.RUnlock()

	if !ok {
		return TokenStatus{Status: "none"}
	}

	isValid := time.Now().Before(entry.expiresAt.Add(-earlyExpiry))
	status := "expired"
	if isValid {
		status = "valid"
	}

	return TokenStatus{
		Status:    status,
		ExpiresAt: &entry.expiresAt,
	}
}

// StartBackgroundRefresh starts a goroutine that periodically refreshes the token.
func (c *TokenCache) StartBackgroundRefresh(ctx context.Context, appID, clientSecret string) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, err := c.GetAccessToken(ctx, appID, clientSecret)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				// Retry after delay
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
				}
				continue
			}

			// Calculate next refresh time
			c.mu.RLock()
			entry, ok := c.cache[appID]
			c.mu.RUnlock()

			var wait time.Duration
			if ok {
				expiresIn := time.Until(entry.expiresAt)
				wait = expiresIn - refreshAhead
				if wait < minRefreshInterval {
					wait = minRefreshInterval
				}
			} else {
				wait = minRefreshInterval
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
}

func normalizeAppID(appID string) string {
	return appID
}
