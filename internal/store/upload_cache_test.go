package store

import (
	"crypto/md5"
	"fmt"
	"testing"
	"time"
)

func TestUploadCache_CacheHit(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("hello")
	c.Set(hash, "c2c", "target1", 1, "file_info_abc", "uuid_abc", 300)

	result := c.Get(hash, "c2c", "target1", 1)
	if result != "file_info_abc" {
		t.Errorf("expected 'file_info_abc', got '%s'", result)
	}
}

func TestUploadCache_CacheMiss(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	result := c.Get("nonexistent_hash", "c2c", "target1", 1)
	if result != "" {
		t.Errorf("expected empty string for cache miss, got '%s'", result)
	}
}

func TestUploadCache_CacheMissAfterExpiry(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("hello")
	// TTL of 2 seconds (minus 60s safety margin would be negative, so minimum 10s)
	// Actually with safety margin of 60s, a TTL of 65s → effective TTL = max(65-60, 10) = 5s
	// Let's use TTL = 61 to get effective TTL = max(61-60, 10) = 10s
	c.Set(hash, "c2c", "target1", 1, "file_info", "uuid", 61)

	// Manually backdate the entry's expiry
	c.mu.Lock()
	key := fmt.Sprintf("%s:%s:%s:%d", hash, "c2c", "target1", 1)
	if entry, ok := c.cache[key]; ok {
		entry.expiresAt = time.Now().UnixMilli() - 1
	}
	c.mu.Unlock()

	result := c.Get(hash, "c2c", "target1", 1)
	if result != "" {
		t.Errorf("expected empty after expiry, got '%s'", result)
	}
}

func TestUploadCache_EvictionAtMax(t *testing.T) {
	c := NewUploadCache(10)
	defer c.Close()

	// Fill cache to max
	for i := 0; i < 15; i++ {
		hash := c.ComputeHash(fmt.Sprintf("data_%d", i))
		c.Set(hash, "c2c", "target1", 1, fmt.Sprintf("info_%d", i), fmt.Sprintf("uuid_%d", i), 3600)
	}

	size, _ := c.Stats()
	if size > 10 {
		t.Errorf("expected size <= 10 after eviction, got %d", size)
	}
}

func TestUploadCache_EvictExpiredFirst(t *testing.T) {
	c := NewUploadCache(10)
	defer c.Close()

	// Fill cache to max with some expired entries
	for i := 0; i < 8; i++ {
		hash := c.ComputeHash(fmt.Sprintf("data_%d", i))
		c.Set(hash, "c2c", "target1", 1, fmt.Sprintf("info_%d", i), fmt.Sprintf("uuid_%d", i), 3600)
	}

	// Manually expire some entries
	c.mu.Lock()
	for i := 0; i < 5; i++ {
		hash := c.ComputeHash(fmt.Sprintf("data_%d", i))
		key := fmt.Sprintf("%s:%s:%s:%d", hash, "c2c", "target1", 1)
		if entry, ok := c.cache[key]; ok {
			entry.expiresAt = time.Now().UnixMilli() - 1
		}
	}
	c.mu.Unlock()

	// Add entries to trigger eviction
	for i := 8; i < 12; i++ {
		hash := c.ComputeHash(fmt.Sprintf("data_%d", i))
		c.Set(hash, "c2c", "target1", 1, fmt.Sprintf("info_%d", i), fmt.Sprintf("uuid_%d", i), 3600)
	}

	// Expired entries should have been evicted first, keeping new ones
	size, _ := c.Stats()
	if size > 10 {
		t.Errorf("expected size <= 10, got %d", size)
	}
}

func TestUploadCache_ComputeHash(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash1 := c.ComputeHash("hello")
	hash2 := c.ComputeHash("hello")
	hash3 := c.ComputeHash("world")

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}

	// Verify it matches MD5
	expected := fmt.Sprintf("%x", md5.Sum([]byte("hello")))
	if hash1 != expected {
		t.Errorf("expected %s, got %s", expected, hash1)
	}
}

func TestUploadCache_DifferentScopes(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("same_data")
	c.Set(hash, "c2c", "target1", 1, "c2c_info", "uuid1", 3600)
	c.Set(hash, "group", "target1", 1, "group_info", "uuid2", 3600)

	result1 := c.Get(hash, "c2c", "target1", 1)
	result2 := c.Get(hash, "group", "target1", 1)

	if result1 != "c2c_info" {
		t.Errorf("expected 'c2c_info', got '%s'", result1)
	}
	if result2 != "group_info" {
		t.Errorf("expected 'group_info', got '%s'", result2)
	}
}

func TestUploadCache_DifferentTargets(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("same_data")
	c.Set(hash, "c2c", "user1", 1, "info1", "uuid1", 3600)
	c.Set(hash, "c2c", "user2", 1, "info2", "uuid2", 3600)

	result1 := c.Get(hash, "c2c", "user1", 1)
	result2 := c.Get(hash, "c2c", "user2", 1)

	if result1 != "info1" {
		t.Errorf("expected 'info1', got '%s'", result1)
	}
	if result2 != "info2" {
		t.Errorf("expected 'info2', got '%s'", result2)
	}
}

func TestUploadCache_DifferentFileTypes(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("same_data")
	c.Set(hash, "c2c", "target1", 1, "image_info", "uuid1", 3600)
	c.Set(hash, "c2c", "target1", 2, "video_info", "uuid2", 3600)

	result1 := c.Get(hash, "c2c", "target1", 1)
	result2 := c.Get(hash, "c2c", "target1", 2)

	if result1 != "image_info" {
		t.Errorf("expected 'image_info', got '%s'", result1)
	}
	if result2 != "video_info" {
		t.Errorf("expected 'video_info', got '%s'", result2)
	}
}

func TestUploadCache_Overwrite(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("hello")
	c.Set(hash, "c2c", "target1", 1, "old_info", "old_uuid", 300)
	c.Set(hash, "c2c", "target1", 1, "new_info", "new_uuid", 300)

	result := c.Get(hash, "c2c", "target1", 1)
	if result != "new_info" {
		t.Errorf("expected 'new_info', got '%s'", result)
	}
}

func TestUploadCache_Stats(t *testing.T) {
	c := NewUploadCache(50)
	defer c.Close()

	hash := c.ComputeHash("hello")
	c.Set(hash, "c2c", "target1", 1, "info", "uuid", 3600)

	size, maxSize := c.Stats()
	if size != 1 {
		t.Errorf("expected size=1, got %d", size)
	}
	if maxSize != 50 {
		t.Errorf("expected maxSize=50, got %d", maxSize)
	}
}

func TestUploadCache_Clear(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("hello")
	c.Set(hash, "c2c", "target1", 1, "info", "uuid", 3600)

	c.Clear()

	size, _ := c.Stats()
	if size != 0 {
		t.Errorf("expected size=0 after clear, got %d", size)
	}

	result := c.Get(hash, "c2c", "target1", 1)
	if result != "" {
		t.Errorf("expected empty after clear, got '%s'", result)
	}
}

func TestUploadCache_SafetyMargin(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	hash := c.ComputeHash("hello")
	// TTL of 300 seconds, effective TTL = max(300-60, 10) = 240 seconds
	c.Set(hash, "c2c", "target1", 1, "info", "uuid", 300)

	// Entry should be valid
	result := c.Get(hash, "c2c", "target1", 1)
	if result != "info" {
		t.Errorf("expected 'info' with safety margin applied, got '%s'", result)
	}
}

func TestUploadCache_ConcurrentAccess(t *testing.T) {
	c := NewUploadCache(100)
	defer c.Close()

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			hash := c.ComputeHash(fmt.Sprintf("data_%d", i%20))
			c.Set(hash, "c2c", "target1", 1, fmt.Sprintf("info_%d", i), "uuid", 3600)
			c.Get(hash, "c2c", "target1", 1)
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
