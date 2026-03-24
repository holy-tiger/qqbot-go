package store

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRefIndexStore_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	entry := RefIndexEntry{
		Content:   "hello world",
		SenderID:  "user1",
		Timestamp: time.Now().UnixMilli(),
	}
	s.Set("REFIDX_001", entry)

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected to get entry")
	}
	if got.Content != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", got.Content)
	}
	if got.SenderID != "user1" {
		t.Errorf("expected sender 'user1', got '%s'", got.SenderID)
	}
}

func TestRefIndexStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	got := s.Get("NONEXISTENT")
	if got != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestRefIndexStore_Overwrite(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "first", SenderID: "u1", Timestamp: 1000})
	s.Set("REFIDX_001", RefIndexEntry{Content: "second", SenderID: "u2", Timestamp: 2000})

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected entry after overwrite")
	}
	if got.Content != "second" {
		t.Errorf("expected content 'second', got '%s'", got.Content)
	}
}

func TestRefIndexStore_TruncateContent(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	longContent := strings.Repeat("x", 1000)
	s.Set("REFIDX_001", RefIndexEntry{Content: longContent, SenderID: "u1", Timestamp: 1000})

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected entry")
	}
	if len(got.Content) != 500 {
		t.Errorf("expected content truncated to 500, got %d", len(got.Content))
	}
}

func TestRefIndexStore_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	// Manually insert with old timestamp
	s.mu.Lock()
	s.loadLocked()
	s.cache["REFIDX_OLD"] = &refEntryInternal{
		RefIndexEntry: RefIndexEntry{Content: "old", SenderID: "u1", Timestamp: 1000},
		createdAt:     time.Now().UnixMilli() - (7*24*60*60*1000 + 1000), // expired
	}
	s.mu.Unlock()

	got := s.Get("REFIDX_OLD")
	if got != nil {
		t.Error("expected nil for expired entry")
	}
}

func TestRefIndexStore_EvictionAtMax(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	// Fill to max and beyond
	for i := 0; i < 50100; i++ {
		s.Set("REFIDX_"+fmt.Sprintf("%d", i), RefIndexEntry{Content: "msg", SenderID: "u", Timestamp: int64(i)})
	}

	size, _, _, _ := s.Stats()
	if size > 50000 {
		t.Errorf("expected size <= 50000, got %d", size)
	}
}

func TestRefIndexStore_Compact(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	// Write 600 unique entries
	for i := 0; i < 600; i++ {
		s.Set(fmt.Sprintf("REFIDX_%d", i), RefIndexEntry{Content: "original", SenderID: "u", Timestamp: int64(i)})
	}
	// Overwrite all 600 entries (totalLines now = 1200, cacheSize = 600)
	for i := 0; i < 600; i++ {
		s.Set(fmt.Sprintf("REFIDX_%d", i), RefIndexEntry{Content: "updated", SenderID: "u", Timestamp: int64(i)})
	}

	// Before flush: totalLines=1200, cacheSize=600, ratio=2x. Need > 2x.
	// The condition is totalLines > cacheSize*2 && totalLines > 1000
	// 1200 > 1200 is false, so won't compact yet.
	// Overwrite one more time
	for i := 0; i < 10; i++ {
		s.Set(fmt.Sprintf("REFIDX_%d", i), RefIndexEntry{Content: "updated2", SenderID: "u", Timestamp: int64(i)})
	}
	// Now totalLines=1210, cacheSize=600, 1210 > 1200 && 1210 > 1000 → compact!

	s.Flush()

	// After compact, totalLines should equal cacheSize
	_, _, totalLines, _ := s.Stats()
	if totalLines > 700 {
		t.Errorf("expected compacted file with totalLines near cacheSize, totalLines=%d", totalLines)
	}
}

func TestRefIndexStore_JSONLFormat(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "test content", SenderID: "user1", Timestamp: 1234567890})
	s.Close()

	// Read the JSONL file
	fp := dir + "/ref-index.jsonl"
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("expected JSONL file to exist: %v", err)
	}

	if !strings.Contains(string(data), `"k":"REFIDX_001"`) {
		t.Error("expected JSONL to contain key 'k'")
	}
	if !strings.Contains(string(data), `"v":{`) {
		t.Error("expected JSONL to contain value 'v'")
	}
	if !strings.Contains(string(data), `"t":`) {
		t.Error("expected JSONL to contain timestamp 't'")
	}
}

func TestRefIndexStore_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	// Pre-write JSONL
	content := fmt.Sprintf(`{"k":"REFIDX_001","v":{"content":"loaded msg","sender_id":"u1","timestamp":1000},"t":%d}`+"\n", now) +
		fmt.Sprintf(`{"k":"REFIDX_002","v":{"content":"expired msg","sender_id":"u2","timestamp":2000},"t":%d}`+"\n", now-(7*24*60*60*1000+1))
	os.WriteFile(dir+"/ref-index.jsonl", []byte(content), 0644)

	s := NewRefIndexStore(dir)
	defer s.Close()

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected to load valid entry")
	}
	if got.Content != "loaded msg" {
		t.Errorf("expected 'loaded msg', got '%s'", got.Content)
	}

	got = s.Get("REFIDX_002")
	if got != nil {
		t.Error("expected expired entry to be nil")
	}
}

func TestRefIndexStore_FormatForAgent(t *testing.T) {
	entry := RefIndexEntry{
		Content:  "hello",
		SenderID: "user1",
	}

	result := FormatForAgent(entry)
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}

	// With attachment
	entry2 := RefIndexEntry{
		Content: "look at this",
		Attachments: []RefAttachmentSummary{
			{Type: "image", Filename: "pic.png", URL: "http://example.com/pic.png"},
			{Type: "voice", Transcript: "hello voice", TranscriptSource: "stt"},
		},
	}
	result = FormatForAgent(entry2)
	if !strings.Contains(result, "[图片: pic.png") {
		t.Error("expected image description")
	}
	if !strings.Contains(result, "hello voice") {
		t.Error("expected voice transcript")
	}
	if !strings.Contains(result, "look at this") {
		t.Error("expected content text")
	}

	// Empty entry
	empty := RefIndexEntry{}
	result = FormatForAgent(empty)
	if result != "[空消息]" {
		t.Errorf("expected '[空消息]' for empty, got '%s'", result)
	}
}

func TestRefIndexStore_Stats(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "a", SenderID: "u1", Timestamp: 1})
	s.Set("REFIDX_002", RefIndexEntry{Content: "b", SenderID: "u1", Timestamp: 2})

	size, maxEntries, totalLines, filePath := s.Stats()
	if size != 2 {
		t.Errorf("expected size=2, got %d", size)
	}
	if maxEntries != 50000 {
		t.Errorf("expected maxEntries=50000, got %d", maxEntries)
	}
	if totalLines != 2 {
		t.Errorf("expected totalLines=2, got %d", totalLines)
	}
	if !strings.HasSuffix(filePath, "ref-index.jsonl") {
		t.Errorf("unexpected filePath: %s", filePath)
	}
}

func TestRefIndexStore_SkipInvalidLines(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	// Write JSONL with invalid lines
	content := fmt.Sprintf(`{"k":"REFIDX_001","v":{"content":"valid","sender_id":"u","timestamp":1},"t":%d}`+"\n", now) +
		"invalid json line\n" +
		`{"k":"","v":{},"t":1}` + "\n" +
		fmt.Sprintf(`{"k":"REFIDX_002","v":{"content":"also valid","sender_id":"u","timestamp":2},"t":%d}`+"\n", now)
	os.WriteFile(dir+"/ref-index.jsonl", []byte(content), 0644)

	s := NewRefIndexStore(dir)
	defer s.Close()

	_, _, totalLines, _ := s.Stats()
	// Should count all non-empty lines (including invalid/unparseable)
	if totalLines != 4 {
		t.Errorf("expected totalLines=4, got %d", totalLines)
	}

	got1 := s.Get("REFIDX_001")
	got2 := s.Get("REFIDX_002")
	if got1 == nil || got2 == nil {
		t.Error("expected valid entries to be loaded")
	}
}

func TestRefIndexStore_LazyLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	// File doesn't exist yet, should not error
	got := s.Get("NONEXISTENT")
	if got != nil {
		t.Error("expected nil when file doesn't exist")
	}
}

func TestRefIndexStore_PersistentAppend(t *testing.T) {
	dir := t.TempDir()

	// Create store, write some data
	s1 := NewRefIndexStore(dir)
	s1.Set("REFIDX_001", RefIndexEntry{Content: "first", SenderID: "u1", Timestamp: 1})
	s1.Close()

	// Open new store instance, should load data
	s2 := NewRefIndexStore(dir)
	defer s2.Close()

	got := s2.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected to load from persisted file")
	}
	if got.Content != "first" {
		t.Errorf("expected 'first', got '%s'", got.Content)
	}
}

func TestRefIndexStore_FileCorruption(t *testing.T) {
	dir := t.TempDir()

	// Write corrupted data
	os.WriteFile(dir+"/ref-index.jsonl", []byte("{{bad json}}}\n"), 0644)

	s := NewRefIndexStore(dir)
	defer s.Close()

	// Should not panic, just return empty
	got := s.Get("REFIDX_001")
	if got != nil {
		t.Error("expected nil for corrupted file")
	}
}

func TestRefIndexStore_ConcurrentSetGet(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	var done sync.WaitGroup
	for i := 0; i < 100; i++ {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			key := fmt.Sprintf("REFIDX_%d", i)
			s.Set(key, RefIndexEntry{Content: "msg", SenderID: "u", Timestamp: int64(i)})
			s.Get(key)
		}(i)
	}
	done.Wait()

	size, _, _, _ := s.Stats()
	if size != 100 {
		t.Errorf("expected 100 entries, got %d", size)
	}
}

func TestRefIndexStore_IgnoreEmptyLines(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	content := "\n\n" +
		fmt.Sprintf(`{"k":"REFIDX_001","v":{"content":"valid","sender_id":"u","timestamp":1},"t":%d}`+"\n", now) +
		"\n"
	os.WriteFile(dir+"/ref-index.jsonl", []byte(content), 0644)

	s := NewRefIndexStore(dir)
	defer s.Close()

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Error("expected valid entry to be loaded, skipping empty lines")
	}
}

func TestRefIndexStore_FlushBasic(t *testing.T) {
	dir := t.TempDir()
	s := NewRefIndexStore(dir)
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "flush test", SenderID: "u", Timestamp: 1})

	// Flush should compact if needed
	s.Flush()

	fp := dir + "/ref-index.jsonl"
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("expected file to exist after flush: %v", err)
	}
}
