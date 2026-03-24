package store

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRefIndexStore_SetAndGet(t *testing.T) {
	s := NewRefIndexStore(openTestDB(t))
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
	s := NewRefIndexStore(openTestDB(t))
	defer s.Close()

	got := s.Get("NONEXISTENT")
	if got != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestRefIndexStore_Overwrite(t *testing.T) {
	s := NewRefIndexStore(openTestDB(t))
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
	s := NewRefIndexStore(openTestDB(t))
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
	db := openTestDB(t)
	s := NewRefIndexStore(db)
	defer s.Close()

	// Insert with old created_at directly
	oldTime := time.Now().UnixMilli() - (7*24*60*60*1000 + 1000)
	db.SQLDB().Exec(`INSERT OR REPLACE INTO ref_index
		(ref_key, content, sender_id, sender_name, timestamp, is_bot, attachments, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"REFIDX_OLD", "old", "u1", "", 1000, 0, "[]", oldTime)

	got := s.Get("REFIDX_OLD")
	if got != nil {
		t.Error("expected nil for expired entry")
	}
}

func TestRefIndexStore_EvictionAtMax(t *testing.T) {
	s := NewRefIndexStore(openTestDB(t))
	defer s.Close()

	for i := 0; i < 50100; i++ {
		s.Set("REFIDX_"+fmt.Sprintf("%d", i), RefIndexEntry{Content: "msg", SenderID: "u", Timestamp: int64(i)})
	}

	size, _, _, _ := s.Stats()
	if size > 50000 {
		t.Errorf("expected size <= 50000, got %d", size)
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
	s := NewRefIndexStore(openTestDB(t))
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "a", SenderID: "u1", Timestamp: 1})
	s.Set("REFIDX_002", RefIndexEntry{Content: "b", SenderID: "u1", Timestamp: 2})

	size, maxEntries, totalLines, _ := s.Stats()
	if size != 2 {
		t.Errorf("expected size=2, got %d", size)
	}
	if maxEntries != 50000 {
		t.Errorf("expected maxEntries=50000, got %d", maxEntries)
	}
	if totalLines != 2 {
		t.Errorf("expected totalLines=2, got %d", totalLines)
	}
}

func TestRefIndexStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	db1, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s1 := NewRefIndexStore(db1)
	s1.Set("REFIDX_001", RefIndexEntry{Content: "first", SenderID: "u1", Timestamp: 1})
	db1.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	s2 := NewRefIndexStore(db2)

	got := s2.Get("REFIDX_001")
	if got == nil {
		t.Fatal("expected to load from persisted SQLite")
	}
	if got.Content != "first" {
		t.Errorf("expected 'first', got '%s'", got.Content)
	}
}

func TestRefIndexStore_FlushBasic(t *testing.T) {
	// Flush is a no-op for SQLite, should not panic
	s := NewRefIndexStore(openTestDB(t))
	defer s.Close()

	s.Set("REFIDX_001", RefIndexEntry{Content: "flush test", SenderID: "u", Timestamp: 1})
	s.Flush()

	got := s.Get("REFIDX_001")
	if got == nil {
		t.Error("expected entry after flush")
	}
}

func TestRefIndexStore_ConcurrentSetGet(t *testing.T) {
	s := NewRefIndexStore(openTestDB(t))
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

func TestRefIndexStore_LazyInit(t *testing.T) {
	// DB doesn't exist yet, should not error
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := NewRefIndexStore(db)
	got := s.Get("NONEXISTENT")
	if got != nil {
		t.Error("expected nil when no entries exist")
	}
}

func TestRefIndexStore_WithAttachments(t *testing.T) {
	s := NewRefIndexStore(openTestDB(t))
	defer s.Close()

	entry := RefIndexEntry{
		Content:   "msg with attachment",
		SenderID:  "u1",
		Timestamp: 1000,
		Attachments: []RefAttachmentSummary{
			{Type: "image", Filename: "test.png", URL: "http://example.com/test.png"},
			{Type: "voice", Transcript: "hello", TranscriptSource: "stt"},
		},
	}
	s.Set("REFIDX_ATT", entry)

	got := s.Get("REFIDX_ATT")
	if got == nil {
		t.Fatal("expected entry")
	}
	if len(got.Attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Type != "image" {
		t.Errorf("expected image attachment, got %s", got.Attachments[0].Type)
	}
}
