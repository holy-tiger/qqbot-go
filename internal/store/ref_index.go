package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	maxContentLength = 500
	maxRefEntries    = 50000
	refTTLMS         = 7 * 24 * 60 * 60 * 1000 // 7 days
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

type refIndexLine struct {
	K string        `json:"k"`
	V RefIndexEntry `json:"v"`
	T int64         `json:"t"`
}

// RefIndexStore manages the reference message index with SQLite persistence.
type RefIndexStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewRefIndexStore creates a new store backed by the shared DB.
func NewRefIndexStore(db *DB) *RefIndexStore {
	return &RefIndexStore{db: db.SQLDB()}
}

// Set stores a ref index entry.
func (s *RefIndexStore) Set(refIdx string, entry RefIndexEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(entry.Content) > maxContentLength {
		entry.Content = entry.Content[:maxContentLength]
	}

	attJSON, _ := json.Marshal(entry.Attachments)
	now := time.Now().UnixMilli()

	_, err := s.db.Exec(`INSERT OR REPLACE INTO ref_index
		(ref_key, content, sender_id, sender_name, timestamp, is_bot, attachments, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		refIdx, entry.Content, entry.SenderID, entry.SenderName,
		entry.Timestamp, entry.IsBot, string(attJSON), now)
	if err != nil {
		fmt.Printf("[store] RefIndex Set: %v\n", err)
		return
	}

	// Evict expired entries.
	expiry := now - refTTLMS
	s.db.Exec(`DELETE FROM ref_index WHERE created_at < ?`, expiry)

	// Cap at max entries.
	s.db.Exec(`DELETE FROM ref_index WHERE id NOT IN (
		SELECT id FROM ref_index ORDER BY created_at DESC LIMIT ?)`, maxRefEntries)
}

// Get retrieves a ref index entry. Returns nil if not found or expired.
func (s *RefIndexStore) Get(refIdx string) *RefIndexEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var content, senderID, senderName string
	var timestamp int64
	var isBot int
	var attJSON string
	var createdAt int64

	err := s.db.QueryRow(`SELECT content, sender_id, sender_name, timestamp, is_bot, attachments, created_at
		FROM ref_index WHERE ref_key = ?`, refIdx).Scan(
		&content, &senderID, &senderName, &timestamp, &isBot, &attJSON, &createdAt)
	if err != nil {
		return nil
	}

	if time.Now().UnixMilli()-createdAt > refTTLMS {
		s.db.Exec(`DELETE FROM ref_index WHERE ref_key = ?`, refIdx)
		return nil
	}

	var attachments []RefAttachmentSummary
	json.Unmarshal([]byte(attJSON), &attachments)

	return &RefIndexEntry{
		Content:     content,
		SenderID:    senderID,
		SenderName:  senderName,
		Timestamp:   timestamp,
		IsBot:       isBot == 1,
		Attachments: attachments,
	}
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

// Flush is a no-op for SQLite backend.
func (s *RefIndexStore) Flush() {}

// Close is a no-op for SQLite backend.
func (s *RefIndexStore) Close() {}

// Stats returns store statistics.
func (s *RefIndexStore) Stats() (size, maxEntries, totalLines int, filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM ref_index`).Scan(&count)
	return count, maxRefEntries, count, "" // filePath no longer relevant
}
