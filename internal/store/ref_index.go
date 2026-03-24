package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxContentLength   = 500
	maxRefEntries      = 50000
	refTTLMS           = 7 * 24 * 60 * 60 * 1000 // 7 days
	compactThreshold   = 2
	compactMinLines    = 1000
)

// RefAttachmentSummary is a summary of a message attachment.
type RefAttachmentSummary struct {
	Type             string `json:"type"`
	Filename         string `json:"filename,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	Transcript       string `json:"transcript,omitempty"`
	TranscriptSource string `json:"transcript_source,omitempty"`
	LocalPath        string `json:"local_path,omitempty"`
	URL              string `json:"url,omitempty"`
}

// RefIndexEntry represents a cached message reference.
type RefIndexEntry struct {
	Content     string                 `json:"content"`
	SenderID    string                 `json:"sender_id"`
	SenderName  string                 `json:"sender_name,omitempty"`
	Timestamp   int64                  `json:"timestamp"`
	IsBot       bool                   `json:"is_bot,omitempty"`
	Attachments []RefAttachmentSummary `json:"attachments,omitempty"`
}

type refEntryInternal struct {
	RefIndexEntry
	createdAt int64
}

type refIndexLine struct {
	K string         `json:"k"`
	V RefIndexEntry  `json:"v"`
	T int64          `json:"t"`
}

// RefIndexStore manages the reference message index with JSONL persistence.
type RefIndexStore struct {
	dir        string
	filePath   string
	mu         sync.Mutex
	cache      map[string]*refEntryInternal
	totalLines int
	loaded     bool
}

// NewRefIndexStore creates a new store backed by dir.
func NewRefIndexStore(dir string) *RefIndexStore {
	return &RefIndexStore{
		dir:      dir,
		filePath: filepath.Join(dir, "ref-index.jsonl"),
		cache:    make(map[string]*refEntryInternal),
	}
}

func (s *RefIndexStore) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	now := time.Now().UnixMilli()
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		s.totalLines++

		var entry refIndexLine
		if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
			continue
		}
		if entry.K == "" || entry.V.Content == "" && len(entry.V.Attachments) == 0 || entry.T == 0 {
			continue
		}

		if now-entry.T > refTTLMS {
			continue
		}

		s.cache[entry.K] = &refEntryInternal{
			RefIndexEntry: entry.V,
			createdAt:     entry.T,
		}
	}

	if s.shouldCompactLocked() {
		s.compactLocked()
	}
}

func (s *RefIndexStore) shouldCompactLocked() bool {
	return s.totalLines > s.cacheSizeLocked()*compactThreshold && s.totalLines > compactMinLines
}

func (s *RefIndexStore) cacheSizeLocked() int {
	return len(s.cache)
}

func (s *RefIndexStore) evictIfNeeded() {
	if len(s.cache) < maxRefEntries {
		return
	}

	now := time.Now().UnixMilli()
	for k, v := range s.cache {
		if now-v.createdAt > refTTLMS {
			delete(s.cache, k)
		}
	}

	if len(s.cache) >= maxRefEntries {
		entries := make([]struct {
			key string
			t   int64
		}, 0, len(s.cache))
		for k, v := range s.cache {
			entries = append(entries, struct {
				key string
				t   int64
			}{k, v.createdAt})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].t < entries[j].t
		})
		removeCount := len(entries) - maxRefEntries + 1000
		if removeCount > len(entries) {
			removeCount = len(entries)
		}
		for i := 0; i < removeCount; i++ {
			delete(s.cache, entries[i].key)
		}
	}
}

func (s *RefIndexStore) appendLine(line refIndexLine) {
	os.MkdirAll(s.dir, 0755)
	f, err := os.OpenFile(s.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(line)
	f.Write(data)
	f.Write([]byte("\n"))
	s.totalLines++
}

func (s *RefIndexStore) compactLocked() {
	tmpPath := s.filePath + ".tmp"
	lines := make([]string, 0, len(s.cache))
	for k, entry := range s.cache {
		line := refIndexLine{
			K: k,
			V: entry.RefIndexEntry,
			T: entry.createdAt,
		}
		data, _ := json.Marshal(line)
		lines = append(lines, string(data))
	}

	content := strings.Join(lines, "\n") + "\n"
	os.MkdirAll(s.dir, 0755)
	os.WriteFile(tmpPath, []byte(content), 0644)
	os.Rename(tmpPath, s.filePath)
	s.totalLines = len(s.cache)
}

// Set stores a ref index entry.
func (s *RefIndexStore) Set(refIdx string, entry RefIndexEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loadLocked()
	s.evictIfNeeded()

	now := time.Now().UnixMilli()

	// Truncate content
	if len(entry.Content) > maxContentLength {
		entry.Content = entry.Content[:maxContentLength]
	}

	s.cache[refIdx] = &refEntryInternal{
		RefIndexEntry: entry,
		createdAt:     now,
	}

	s.appendLine(refIndexLine{K: refIdx, V: entry, T: now})

	if s.shouldCompactLocked() {
		s.compactLocked()
	}
}

// Get retrieves a ref index entry. Returns nil if not found or expired.
func (s *RefIndexStore) Get(refIdx string) *RefIndexEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loadLocked()

	entry, ok := s.cache[refIdx]
	if !ok {
		return nil
	}

	if time.Now().UnixMilli()-entry.createdAt > refTTLMS {
		delete(s.cache, refIdx)
		return nil
	}

	result := entry.RefIndexEntry
	return &result
}

// FormatForAgent formats a ref entry for AI context injection.
func FormatForAgent(entry RefIndexEntry) string {
	var parts []string

	if strings.TrimSpace(entry.Content) != "" {
		parts = append(parts, entry.Content)
	}

	if len(entry.Attachments) > 0 {
		for _, att := range entry.Attachments {
			var sourceHint string
			if att.LocalPath != "" {
				sourceHint = " (" + att.LocalPath + ")"
			} else if att.URL != "" {
				sourceHint = " (" + att.URL + ")"
			}

			switch att.Type {
			case "image":
				desc := "[图片"
				if att.Filename != "" {
					desc += ": " + att.Filename
				}
				desc += sourceHint + "]"
				parts = append(parts, desc)
			case "voice":
				if att.Transcript != "" {
					sourceMap := map[string]string{"stt": "本地识别", "asr": "官方识别", "tts": "TTS原文", "fallback": "兜底文案"}
					var sourceTag string
					if att.TranscriptSource != "" {
						if label, ok := sourceMap[att.TranscriptSource]; ok {
							sourceTag = " - " + label
						} else {
							sourceTag = " - " + att.TranscriptSource
						}
					}
					parts = append(parts, "[语音消息（内容: \""+att.Transcript+"\""+sourceTag+"）"+sourceHint+"]")
				} else {
					parts = append(parts, "[语音消息"+sourceHint+"]")
				}
			case "video":
				desc := "[视频"
				if att.Filename != "" {
					desc += ": " + att.Filename
				}
				desc += sourceHint + "]"
				parts = append(parts, desc)
			case "file":
				desc := "[文件"
				if att.Filename != "" {
					desc += ": " + att.Filename
				}
				desc += sourceHint + "]"
				parts = append(parts, desc)
			default:
				desc := "[附件"
				if att.Filename != "" {
					desc += ": " + att.Filename
				}
				desc += sourceHint + "]"
				parts = append(parts, desc)
			}
		}
	}

	if len(parts) == 0 {
		return "[空消息]"
	}
	return strings.Join(parts, " ")
}

// Flush compacts the store if needed.
func (s *RefIndexStore) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loadLocked()
	if s.shouldCompactLocked() {
		s.compactLocked()
	}
}

// Close is an alias for Flush for cleanup.
func (s *RefIndexStore) Close() {
	s.Flush()
}

// Stats returns store statistics.
func (s *RefIndexStore) Stats() (size, maxEntries, totalLines int, filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.loadLocked()
	return len(s.cache), maxRefEntries, s.totalLines, s.filePath
}
