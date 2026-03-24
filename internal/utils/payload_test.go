package utils

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseQQBotPayload_CronReminder(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"type": "cron_reminder", "content": "drink water", "targetType": "c2c", "targetAddress": "user_123"}`

	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Fatal("expected IsPayload=true")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Text != "" {
		t.Errorf("expected empty Text, got %q", result.Text)
	}
	payload, ok := result.Payload.(*CronReminderPayload)
	if !ok {
		t.Fatal("expected *CronReminderPayload")
	}
	if payload.Type != "cron_reminder" {
		t.Errorf("got type %q", payload.Type)
	}
	if payload.Content != "drink water" {
		t.Errorf("got content %q", payload.Content)
	}
	if payload.TargetType != "c2c" {
		t.Errorf("got targetType %q", payload.TargetType)
	}
	if payload.TargetAddress != "user_123" {
		t.Errorf("got targetAddress %q", payload.TargetAddress)
	}
}

func TestParseQQBotPayload_Media(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"type": "media", "mediaType": "image", "source": "file", "path": "/tmp/photo.jpg", "caption": "a photo"}`

	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Fatal("expected IsPayload=true")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	payload, ok := result.Payload.(*MediaPayload)
	if !ok {
		t.Fatal("expected *MediaPayload")
	}
	if payload.Type != "media" {
		t.Errorf("got type %q", payload.Type)
	}
	if payload.MediaType != "image" {
		t.Errorf("got mediaType %q", payload.MediaType)
	}
	if payload.Source != "file" {
		t.Errorf("got source %q", payload.Source)
	}
	if payload.Path != "/tmp/photo.jpg" {
		t.Errorf("got path %q", payload.Path)
	}
	if payload.Caption != "a photo" {
		t.Errorf("got caption %q", payload.Caption)
	}
}

func TestParseQQBotPayload_NonPayload(t *testing.T) {
	input := "Hello, this is just a regular message"
	result := ParseQQBotPayload(input)
	if result.IsPayload {
		t.Error("expected IsPayload=false")
	}
	if result.Text != input {
		t.Errorf("got text %q, want %q", result.Text, input)
	}
}

func TestParseQQBotPayload_InvalidJSON(t *testing.T) {
	input := "QQBOT_PAYLOAD:\n{not valid json}"
	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Error("expected IsPayload=true")
	}
	if result.Error == "" {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseQQBotPayload_EmptyContent(t *testing.T) {
	input := "QQBOT_PAYLOAD:\n"
	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Error("expected IsPayload=true")
	}
	if result.Error == "" {
		t.Error("expected error for empty content")
	}
}

func TestParseQQBotPayload_MissingFields_Cron(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"type": "cron_reminder", "content": "test"}`
	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Error("expected IsPayload=true")
	}
	if result.Error == "" {
		t.Error("expected error for missing fields")
	}
}

func TestParseQQBotPayload_MissingFields_Media(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"type": "media", "mediaType": "image"}`
	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Error("expected IsPayload=true")
	}
	if result.Error == "" {
		t.Error("expected error for missing fields")
	}
}

func TestParseQQBotPayload_MissingType(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"content": "test"}`
	result := ParseQQBotPayload(input)
	if !result.IsPayload {
		t.Error("expected IsPayload=true")
	}
	if result.Error == "" {
		t.Error("expected error for missing type")
	}
}

func TestEncodeDecodeCronPayload_Roundtrip(t *testing.T) {
	original := CronReminderPayload{
		Type:          "cron_reminder",
		Content:       "time to drink water",
		TargetType:    "group",
		TargetAddress: "group_456",
	}

	encoded := EncodePayloadForCron(original)
	if !strings.HasPrefix(encoded, "QQBOT_CRON:") {
		t.Errorf("encoded should start with QQBOT_CRON:, got %q", encoded)
	}

	isCron, decoded, err := DecodeCronPayload(encoded)
	if !isCron {
		t.Error("expected isCron=true")
	}
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if decoded == nil {
		t.Fatal("expected non-nil payload")
	}
	if decoded.Content != original.Content {
		t.Errorf("got content %q, want %q", decoded.Content, original.Content)
	}
	if decoded.TargetType != original.TargetType {
		t.Errorf("got targetType %q, want %q", decoded.TargetType, original.TargetType)
	}
	if decoded.TargetAddress != original.TargetAddress {
		t.Errorf("got targetAddress %q, want %q", decoded.TargetAddress, original.TargetAddress)
	}
}

func TestDecodeCronPayload_NotCron(t *testing.T) {
	isCron, _, err := DecodeCronPayload("regular message")
	if isCron {
		t.Error("expected isCron=false")
	}
	if err != "" {
		t.Errorf("expected no error, got %q", err)
	}
}

func TestDecodeCronPayload_Empty(t *testing.T) {
	isCron, _, err := DecodeCronPayload("QQBOT_CRON:")
	if !isCron {
		t.Error("expected isCron=true")
	}
	if err == "" {
		t.Error("expected error for empty content")
	}
}

func TestDecodeCronPayload_InvalidBase64(t *testing.T) {
	isCron, _, err := DecodeCronPayload("QQBOT_CRON:!!!invalid!!!")
	if !isCron {
		t.Error("expected isCron=true")
	}
	if err == "" {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodeCronPayload_WrongType(t *testing.T) {
	// Encode a media payload but try to decode as cron
	mediaJSON, _ := json.Marshal(map[string]string{"type": "media"})
	encoded := "QQBOT_CRON:" + base64.StdEncoding.EncodeToString(mediaJSON)

	isCron, _, err := DecodeCronPayload(encoded)
	if !isCron {
		t.Error("expected isCron=true")
	}
	if err == "" {
		t.Error("expected error for wrong type")
	}
}

func TestIsCronReminderPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		want    bool
	}{
		{"cron", &CronReminderPayload{Type: "cron_reminder"}, true},
		{"media", &MediaPayload{Type: "media"}, false},
		{"nil", nil, false},
		{"string", "not a payload", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCronReminderPayload(tt.payload)
			if got != tt.want {
				t.Errorf("IsCronReminderPayload = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMediaPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		want    bool
	}{
		{"media", &MediaPayload{Type: "media"}, true},
		{"cron", &CronReminderPayload{Type: "cron_reminder"}, false},
		{"nil", nil, false},
		{"string", "not a payload", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMediaPayload(tt.payload)
			if got != tt.want {
				t.Errorf("IsMediaPayload = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseQQBotPayload_CronWithOriginalMsgID(t *testing.T) {
	input := `QQBOT_PAYLOAD:
{"type": "cron_reminder", "content": "test", "targetType": "c2c", "targetAddress": "user_1", "originalMessageId": "msg_123"}`
	result := ParseQQBotPayload(input)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	payload := result.Payload.(*CronReminderPayload)
	if payload.OriginalMsgID != "msg_123" {
		t.Errorf("got OriginalMsgID %q, want %q", payload.OriginalMsgID, "msg_123")
	}
}
