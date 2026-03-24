package store

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"
)

const (
	defaultMaxCacheSize = 500
	safetyMargin        = 60 // seconds
	minEffectiveTTL     = 10 // seconds
)

type uploadCacheEntry struct {
	fileInfo  string
	fileUUID  string
	expiresAt int64
}

// UploadCache caches file upload results to avoid redundant uploads.
type UploadCache struct {
	mu      sync.Mutex
	cache   map[string]*uploadCacheEntry
	maxSize int
}

// NewUploadCache creates a new upload cache with the given max size.
func NewUploadCache(maxSize int) *UploadCache {
	if maxSize <= 0 {
		maxSize = defaultMaxCacheSize
	}
	return &UploadCache{
		cache:   make(map[string]*uploadCacheEntry),
		maxSize: maxSize,
	}
}

func (c *UploadCache) buildKey(contentHash, scope, targetID string, fileType int) string {
	return fmt.Sprintf("%s:%s:%s:%d", contentHash, scope, targetID, fileType)
}

// Get returns the cached file_info for the given key. Returns "" if not found or expired.
func (c *UploadCache) Get(contentHash, scope, targetID string, fileType int) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.buildKey(contentHash, scope, targetID, fileType)
	entry, ok := c.cache[key]
	if !ok {
		return ""
	}

	if time.Now().UnixMilli() >= entry.expiresAt {
		delete(c.cache, key)
		return ""
	}

	return entry.fileInfo
}

// Set caches a file upload result with the given TTL.
func (c *UploadCache) Set(contentHash, scope, targetID string, fileType int, fileInfo, fileUUID string, ttl int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict expired entries if at capacity
	if len(c.cache) >= c.maxSize {
		now := time.Now().UnixMilli()
		for k, v := range c.cache {
			if now >= v.expiresAt {
				delete(c.cache, k)
			}
		}
		// If still over capacity, evict oldest half
		if len(c.cache) >= c.maxSize {
			keys := make([]string, 0, len(c.cache))
			for k := range c.cache {
				keys = append(keys, k)
			}
			removeCount := len(keys) / 2
			for i := 0; i < removeCount; i++ {
				delete(c.cache, keys[i])
			}
		}
	}

	key := c.buildKey(contentHash, scope, targetID, fileType)
	effectiveTTL := ttl - safetyMargin
	if effectiveTTL < minEffectiveTTL {
		effectiveTTL = minEffectiveTTL
	}

	c.cache[key] = &uploadCacheEntry{
		fileInfo:  fileInfo,
		fileUUID:  fileUUID,
		expiresAt: time.Now().UnixMilli() + int64(effectiveTTL)*1000,
	}
}

// ComputeHash returns the MD5 hex hash of the given data.
func (c *UploadCache) ComputeHash(data string) string {
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// Stats returns the current cache size and max size.
func (c *UploadCache) Stats() (size, maxSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.cache), c.maxSize
}

// Clear removes all entries from the cache.
func (c *UploadCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*uploadCacheEntry)
}

// Close is a no-op for in-memory cache, provided for interface consistency.
func (c *UploadCache) Close() {}
