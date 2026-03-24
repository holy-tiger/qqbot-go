package api

import (
	"crypto/md5"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/openclaw/qqbot/internal/types"
)

const (
	uploadCacheSafetyMargin = 60 // seconds
	uploadCacheMinTTL       = 10 // seconds
)

// cacheEntry stores uploaded file info with TTL.
type cacheEntry struct {
	fileInfo string
	fileUUID string
	expiresAt time.Time
}

// UploadCache caches file_info from upload responses to avoid re-uploading.
type UploadCache struct {
	mu      sync.Mutex
	cache   map[string]*cacheEntry
	maxSize int
}

// NewUploadCache creates a new UploadCache with the given maximum size.
func NewUploadCache(maxSize int) *UploadCache {
	return &UploadCache{
		cache:   make(map[string]*cacheEntry),
		maxSize: maxSize,
	}
}

// buildCacheKey constructs the cache key in the format md5(data):scope:targetId:fileType.
func buildCacheKey(contentHash, scope, targetID string, fileType types.MediaFileType) string {
	return fmt.Sprintf("%s:%s:%s:%d", contentHash, scope, targetID, fileType)
}

// Get retrieves cached file_info for the given key. Returns empty string on miss or expiry.
func (c *UploadCache) Get(contentHash, scope, targetID string, fileType types.MediaFileType) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := buildCacheKey(contentHash, scope, targetID, fileType)
	entry, ok := c.cache[key]
	if !ok {
		return ""
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.cache, key)
		return ""
	}
	return entry.fileInfo
}

// Set stores file_info in the cache with the given TTL.
func (c *UploadCache) Set(contentHash, scope, targetID string, fileType types.MediaFileType, fileInfo, fileUUID string, ttl int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict expired entries if at capacity
	if len(c.cache) >= c.maxSize {
		now := time.Now()
		for k, v := range c.cache {
			if now.After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
	}
	// If still at capacity, evict oldest half
	if len(c.cache) >= c.maxSize {
		entries := make([]cacheEntryWithKey, 0, len(c.cache))
		for k, v := range c.cache {
			entries = append(entries, cacheEntryWithKey{key: k, expiresAt: v.expiresAt})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].expiresAt.Before(entries[j].expiresAt)
		})
		removeCount := len(entries) / 2
		if removeCount < 1 {
			removeCount = 1
		}
		for i := 0; i < removeCount; i++ {
			delete(c.cache, entries[i].key)
		}
	}

	key := buildCacheKey(contentHash, scope, targetID, fileType)
	effectiveTTL := ttl - uploadCacheSafetyMargin
	if effectiveTTL < uploadCacheMinTTL {
		effectiveTTL = uploadCacheMinTTL
	}

	c.cache[key] = &cacheEntry{
		fileInfo:  fileInfo,
		fileUUID:  fileUUID,
		expiresAt: time.Now().Add(time.Duration(effectiveTTL) * time.Second),
	}
}

type cacheEntryWithKey struct {
	key       string
	expiresAt time.Time
}

// ComputeFileHash computes the MD5 hash of the given data string.
func (c *UploadCache) ComputeFileHash(data string) string {
	h := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", h)
}

// Stats returns the current cache size and maximum size.
func (c *UploadCache) Stats() (size, maxSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.cache), c.maxSize
}

// Clear removes all entries from the cache.
func (c *UploadCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
}
