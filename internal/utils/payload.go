package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// PayloadPrefix is the prefix for AI-generated structured payloads.
	PayloadPrefix = "QQBOT_PAYLOAD:"
	// CronPrefix is the prefix for cron-encoded reminder payloads.
	CronPrefix = "QQBOT_CRON:"
)

// CronReminderPayload represents a scheduled reminder.
type CronReminderPayload struct {
	Type          string `json:"type"`
	Content       string `json:"content"`
	TargetType    string `json:"targetType"`
	TargetAddress string `json:"targetAddress"`
	OriginalMsgID string `json:"originalMessageId,omitempty"`
}

// MediaPayload represents a media message (image, audio, video, file).
type MediaPayload struct {
	Type      string `json:"type"`
	MediaType string `json:"mediaType"`
	Source    string `json:"source"`
	Path      string `json:"path"`
	Caption   string `json:"caption,omitempty"`
}

// ParseResult holds the result of parsing a QQ Bot payload.
type ParseResult struct {
	IsPayload bool
	Payload   interface{}
	Text      string
	Error     string
}

// ParseQQBotPayload checks if text starts with QQBOT_PAYLOAD: prefix and parses the JSON.
func ParseQQBotPayload(text string) ParseResult {
	trimmed := strings.TrimSpace(text)

	if !strings.HasPrefix(trimmed, PayloadPrefix) {
		return ParseResult{IsPayload: false, Text: text}
	}

	jsonContent := strings.TrimSpace(trimmed[len(PayloadPrefix):])
	if jsonContent == "" {
		return ParseResult{IsPayload: true, Error: "payload content is empty"}
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &raw); err != nil {
		return ParseResult{IsPayload: true, Error: fmt.Sprintf("JSON parse error: %v", err)}
	}

	typeVal, _ := raw["type"].(string)
	if typeVal == "" {
		return ParseResult{IsPayload: true, Error: "payload missing type field"}
	}

	switch typeVal {
	case "cron_reminder":
		var payload CronReminderPayload
		if err := json.Unmarshal([]byte(jsonContent), &payload); err != nil {
			return ParseResult{IsPayload: true, Error: fmt.Sprintf("JSON parse error: %v", err)}
		}
		if payload.Content == "" || payload.TargetType == "" || payload.TargetAddress == "" {
			return ParseResult{IsPayload: true, Error: "cron_reminder payload missing required fields (content, targetType, targetAddress)"}
		}
		return ParseResult{IsPayload: true, Payload: &payload}

	case "media":
		var payload MediaPayload
		if err := json.Unmarshal([]byte(jsonContent), &payload); err != nil {
			return ParseResult{IsPayload: true, Error: fmt.Sprintf("JSON parse error: %v", err)}
		}
		if payload.MediaType == "" || payload.Source == "" || payload.Path == "" {
			return ParseResult{IsPayload: true, Error: "media payload missing required fields (mediaType, source, path)"}
		}
		return ParseResult{IsPayload: true, Payload: &payload}

	default:
		return ParseResult{IsPayload: true, Error: fmt.Sprintf("unknown payload type: %s", typeVal)}
	}
}

// EncodePayloadForCron encodes a CronReminderPayload as "QQBOT_CRON:{base64}".
func EncodePayloadForCron(payload CronReminderPayload) string {
	jsonBytes, _ := json.Marshal(payload)
	return CronPrefix + base64.StdEncoding.EncodeToString(jsonBytes)
}

// DecodeCronPayload decodes a "QQBOT_CRON:{base64}" message.
// Returns (isCron, payload, error).
func DecodeCronPayload(message string) (bool, *CronReminderPayload, string) {
	trimmed := strings.TrimSpace(message)

	if !strings.HasPrefix(trimmed, CronPrefix) {
		return false, nil, ""
	}

	base64Content := trimmed[len(CronPrefix):]
	if base64Content == "" {
		return true, nil, "cron payload content is empty"
	}

	jsonBytes, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return true, nil, fmt.Sprintf("cron payload decode error: %v", err)
	}

	var payload CronReminderPayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return true, nil, fmt.Sprintf("cron payload JSON error: %v", err)
	}

	if payload.Type != "cron_reminder" {
		return true, nil, fmt.Sprintf("expected type cron_reminder, got %s", payload.Type)
	}

	if payload.Content == "" || payload.TargetType == "" || payload.TargetAddress == "" {
		return true, nil, "cron payload missing required fields"
	}

	return true, &payload, ""
}

// IsCronReminderPayload checks if the payload is a cron reminder.
func IsCronReminderPayload(payload interface{}) bool {
	p, ok := payload.(*CronReminderPayload)
	return ok && p.Type == "cron_reminder"
}

// IsMediaPayload checks if the payload is a media message.
func IsMediaPayload(payload interface{}) bool {
	p, ok := payload.(*MediaPayload)
	return ok && p.Type == "media"
}
