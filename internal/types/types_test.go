package types

import (
	"encoding/json"
	"testing"
)

func TestMediaFileTypeConstants(t *testing.T) {
	if MediaFileTypeImage != 1 {
		t.Errorf("MediaFileTypeImage = %d, want 1", MediaFileTypeImage)
	}
	if MediaFileTypeVideo != 2 {
		t.Errorf("MediaFileTypeVideo = %d, want 2", MediaFileTypeVideo)
	}
	if MediaFileTypeVoice != 3 {
		t.Errorf("MediaFileTypeVoice = %d, want 3", MediaFileTypeVoice)
	}
	if MediaFileTypeFile != 4 {
		t.Errorf("MediaFileTypeFile = %d, want 4", MediaFileTypeFile)
	}
}

func TestIntentConstants(t *testing.T) {
	if IntentGuilds != 1<<0 {
		t.Errorf("IntentGuilds = %d, want 1", IntentGuilds)
	}
	if IntentGuildMembers != 1<<1 {
		t.Errorf("IntentGuildMembers = %d, want 2", IntentGuildMembers)
	}
	if IntentDirectMessage != 1<<12 {
		t.Errorf("IntentDirectMessage = %d, want 4096", IntentDirectMessage)
	}
	if IntentGroupAndC2C != 1<<30 {
		t.Errorf("IntentGroupAndC2C = %d, want 1073741824", IntentGroupAndC2C)
	}
	if IntentPublicGuildMessages != 1<<25 {
		t.Errorf("IntentPublicGuildMessages = %d, want 33554432", IntentPublicGuildMessages)
	}
}

func TestDefaultIntentLevels(t *testing.T) {
	if len(DefaultIntentLevels) != 3 {
		t.Fatalf("DefaultIntentLevels length = %d, want 3", len(DefaultIntentLevels))
	}
	// Full level should have all intents combined
	if DefaultIntentLevels[0].Intents == 0 {
		t.Error("Full level should have intents")
	}
	// Check levels are ordered by priority (first = highest)
	if DefaultIntentLevels[0].Priority != 0 {
		t.Errorf("First level priority = %d, want 0", DefaultIntentLevels[0].Priority)
	}
}

func TestWSPayloadMarshalJSON(t *testing.T) {
	p := WSPayload{
		Op: 0,
		Data: json.RawMessage(`{"content":"hello"}`),
		Seq: intPtr(1),
		Event: strPtr("MESSAGE_CREATE"),
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded["op"].(float64) != 0 {
		t.Errorf("op = %v, want 0", decoded["op"])
	}
	if decoded["s"].(float64) != 1 {
		t.Errorf("s = %v, want 1", decoded["s"])
	}
	if decoded["t"].(string) != "MESSAGE_CREATE" {
		t.Errorf("t = %v, want MESSAGE_CREATE", decoded["t"])
	}
}

func TestWSPayloadUnmarshalJSON(t *testing.T) {
	raw := `{"op":10,"s":42,"t":"READY","d":{"session_id":"abc123"}}`
	var p WSPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if p.Op != 10 {
		t.Errorf("Op = %d, want 10", p.Op)
	}
	if p.Seq == nil || *p.Seq != 42 {
		t.Errorf("Seq = %v, want 42", p.Seq)
	}
	if p.Event == nil || *p.Event != "READY" {
		t.Errorf("Event = %v, want READY", p.Event)
	}
}

func TestWSPayloadOmitEmpty(t *testing.T) {
	p := WSPayload{Op: 1}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(data)
	if contains(s, `"s"`) {
		t.Errorf("empty Seq should be omitted, got %s", s)
	}
	if contains(s, `"t"`) {
		t.Errorf("empty Event should be omitted, got %s", s)
	}
}

func TestC2CMessageEventMarshalJSON(t *testing.T) {
	evt := C2CMessageEvent{
		Author: C2CAuthor{
			ID:           "user123",
			UnionOpenID:  "union_abc",
			UserOpenID:   "user_xyz",
		},
		Content:   "hello bot",
		ID:        "msg_001",
		Timestamp: "2024-01-01T00:00:00Z",
		Attachments: []MessageAttachment{
			{
				ContentType:  "image/png",
				Filename:     strPtr("test.png"),
				Height:       intPtr(100),
				Width:        intPtr(200),
				Size:         intPtr(1024),
				URL:          "https://example.com/img.png",
				VoiceWavURL:  strPtr("https://example.com/voice.wav"),
				ASRReferText: strPtr("hello"),
			},
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded["content"].(string) != "hello bot" {
		t.Errorf("content = %v, want hello bot", decoded["content"])
	}

	atts := decoded["attachments"].([]interface{})
	if len(atts) != 1 {
		t.Fatalf("attachments length = %d, want 1", len(atts))
	}
	att := atts[0].(map[string]interface{})
	if att["content_type"].(string) != "image/png" {
		t.Errorf("content_type = %v, want image/png", att["content_type"])
	}
}

func TestC2CMessageEventUnmarshalJSON(t *testing.T) {
	raw := `{
		"author":{"id":"u1","union_openid":"uo1","user_openid":"uo2"},
		"content":"hi","id":"m1","timestamp":"2024-01-01T00:00:00Z",
		"attachments":[{"content_type":"image/jpeg","url":"https://x.com/i.jpg"}]
	}`
	var evt C2CMessageEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if evt.Author.ID != "u1" {
		t.Errorf("Author.ID = %v, want u1", evt.Author.ID)
	}
	if len(evt.Attachments) != 1 || evt.Attachments[0].ContentType != "image/jpeg" {
		t.Errorf("Attachments mismatch: %+v", evt.Attachments)
	}
}

func TestGuildMessageEventMarshalJSON(t *testing.T) {
	evt := GuildMessageEvent{
		ID:        "guild_msg_1",
		ChannelID: "ch_1",
		GuildID:   "guild_1",
		Content:   "@bot hello",
		Timestamp: "2024-01-01T00:00:00Z",
		Author: GuildAuthor{
			ID:       "author_1",
			Username: strPtr("testuser"),
			Bot:      boolPtr(false),
		},
		Member: &GuildMember{
			Nick:     strPtr("nickname"),
			JoinedAt: strPtr("2023-01-01T00:00:00Z"),
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded["guild_id"].(string) != "guild_1" {
		t.Errorf("guild_id = %v, want guild_1", decoded["guild_id"])
	}
}

func TestGuildMessageEventUnmarshalJSON(t *testing.T) {
	raw := `{
		"id":"g1","channel_id":"c1","guild_id":"g1",
		"content":"hello","timestamp":"2024-01-01T00:00:00Z",
		"author":{"id":"a1","bot":true},
		"member":{"nick":"bob"}
	}`
	var evt GuildMessageEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if evt.Author.ID != "a1" {
		t.Errorf("Author.ID = %v, want a1", evt.Author.ID)
	}
	if evt.Member == nil || *evt.Member.Nick != "bob" {
		t.Errorf("Member.Nick = %v, want bob", evt.Member)
	}
}

func TestGroupMessageEventMarshalJSON(t *testing.T) {
	evt := GroupMessageEvent{
		Author: GroupAuthor{
			ID:           "user1",
			MemberOpenID: "mo1",
		},
		Content:     "@bot hi",
		ID:          "grp_1",
		Timestamp:   "2024-01-01T00:00:00Z",
		GroupID:     "grp_id_1",
		GroupOpenID: "grp_oid_1",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded["group_openid"].(string) != "grp_oid_1" {
		t.Errorf("group_openid = %v, want grp_oid_1", decoded["group_openid"])
	}
}

func TestGroupMessageEventUnmarshalJSON(t *testing.T) {
	raw := `{
		"author":{"id":"u1","member_openid":"mo1"},
		"content":"hi","id":"g1","timestamp":"2024-01-01T00:00:00Z",
		"group_id":"gid1","group_openid":"goid1"
	}`
	var evt GroupMessageEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if evt.GroupID != "gid1" {
		t.Errorf("GroupID = %v, want gid1", evt.GroupID)
	}
}

func TestMessageAttachment(t *testing.T) {
	att := MessageAttachment{
		ContentType:  "audio/silk",
		URL:          "https://example.com/voice.silk",
		Filename:     strPtr("voice.silk"),
		Height:       intPtr(0),
		Width:        intPtr(0),
		Size:         intPtr(4096),
		VoiceWavURL:  strPtr("https://example.com/voice.wav"),
		ASRReferText: strPtr("recognized text"),
	}

	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded MessageAttachment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ContentType != "audio/silk" {
		t.Errorf("ContentType = %v, want audio/silk", decoded.ContentType)
	}
	if decoded.VoiceWavURL == nil || *decoded.VoiceWavURL != "https://example.com/voice.wav" {
		t.Errorf("VoiceWavURL = %v", decoded.VoiceWavURL)
	}
}

func TestMessageAttachmentOmitEmpty(t *testing.T) {
	att := MessageAttachment{
		ContentType: "image/png",
		URL:         "https://example.com/img.png",
	}
	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	s := string(data)
	if contains(s, `"filename"`) {
		t.Errorf("empty Filename should be omitted, got %s", s)
	}
	if contains(s, `"height"`) {
		t.Errorf("empty Height should be omitted, got %s", s)
	}
}

func TestMessageResponse(t *testing.T) {
	resp := MessageResponse{
		ID:      "resp_1",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded MessageResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.ID != "resp_1" {
		t.Errorf("ID = %v, want resp_1", decoded.ID)
	}
}

func TestUploadMediaResponse(t *testing.T) {
	resp := UploadMediaResponse{
		FileUUID: "uuid_123",
		FileInfo: "file_info_data",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded UploadMediaResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.FileUUID != "uuid_123" {
		t.Errorf("FileUUID = %v, want uuid_123", decoded.FileUUID)
	}
}

func TestExtInfo(t *testing.T) {
	info := ExtInfo{
		BusinessID: 1,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ExtInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.BusinessID != 1 {
		t.Errorf("BusinessID = %v, want 1", decoded.BusinessID)
	}
}

func TestOutboundMeta(t *testing.T) {
	meta := OutboundMeta{
		MessageID:    "msg_001",
		MessageScene: "guild",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded OutboundMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.MessageID != "msg_001" {
		t.Errorf("MessageID = %v, want msg_001", decoded.MessageID)
	}
}

func TestAudioFormatPolicy(t *testing.T) {
	policy := AudioFormatPolicy{
		STTDirectFormats:   []string{".silk", ".amr"},
		UploadDirectFormats: []string{".wav", ".mp3", ".silk"},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded AudioFormatPolicy
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(decoded.STTDirectFormats) != 2 {
		t.Errorf("STTDirectFormats length = %d, want 2", len(decoded.STTDirectFormats))
	}
	if len(decoded.UploadDirectFormats) != 3 {
		t.Errorf("UploadDirectFormats length = %d, want 3", len(decoded.UploadDirectFormats))
	}
}

func TestQQBotAccountConfig(t *testing.T) {
	cfg := QQBotAccountConfig{
		Enabled:          boolPtr(true),
		Name:             strPtr("TestBot"),
		AppID:            strPtr("app123"),
		ClientSecret:     strPtr("secret"),
		SystemPrompt:     strPtr("be helpful"),
		MarkdownSupport:  boolPtr(true),
		ImageServerBaseUrl: strPtr("http://localhost:18765"),
		AllowFrom:        []string{"*"},
		DMPolicy:         strPtr("open"),
		AudioFormatPolicy: &AudioFormatPolicy{
			STTDirectFormats:   []string{".silk"},
			UploadDirectFormats: []string{".wav"},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded QQBotAccountConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Name == nil || *decoded.Name != "TestBot" {
		t.Errorf("Name = %v, want TestBot", decoded.Name)
	}
	if decoded.AudioFormatPolicy == nil || len(decoded.AudioFormatPolicy.STTDirectFormats) != 1 {
		t.Errorf("AudioFormatPolicy mismatch: %+v", decoded.AudioFormatPolicy)
	}
}

func TestResolvedQQBotAccount(t *testing.T) {
	acc := ResolvedQQBotAccount{
		AccountID:        "default",
		Name:             strPtr("Bot"),
		Enabled:          true,
		AppID:            "app1",
		ClientSecret:     "sec1",
		SecretSource:     "config",
		SystemPrompt:     strPtr("prompt"),
		ImageServerBaseUrl: strPtr("http://ip:18765"),
		MarkdownSupport:  true,
	}

	data, err := json.Marshal(acc)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ResolvedQQBotAccount
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.AccountID != "default" {
		t.Errorf("AccountID = %v, want default", decoded.AccountID)
	}
	if decoded.SecretSource != "config" {
		t.Errorf("SecretSource = %v, want config", decoded.SecretSource)
	}
}

// helper functions
func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
func boolPtr(v bool) *bool    { return &v }
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
