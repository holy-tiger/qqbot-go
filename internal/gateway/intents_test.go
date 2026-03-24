package gateway

import (
	"encoding/json"
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

func TestIdentifyParamsJSON(t *testing.T) {
	p := IdentifyParams{
		Token:   "QQBot test-token",
		Intents: 1 << 30,
		Shard:   []int{0, 1},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal IdentifyParams: %v", err)
	}

	var decoded IdentifyParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal IdentifyParams: %v", err)
	}

	if decoded.Token != p.Token {
		t.Errorf("Token mismatch: got %q, want %q", decoded.Token, p.Token)
	}
	if decoded.Intents != p.Intents {
		t.Errorf("Intents mismatch: got %d, want %d", decoded.Intents, p.Intents)
	}
	if len(decoded.Shard) != 2 || decoded.Shard[0] != 0 || decoded.Shard[1] != 1 {
		t.Errorf("Shard mismatch: got %v, want [0 1]", decoded.Shard)
	}
}

func TestIdentifyParamsJSONFields(t *testing.T) {
	p := IdentifyParams{
		Token:   "QQBot tok",
		Intents: 513,
		Shard:   []int{0, 1},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw: %v", err)
	}

	if raw["token"] != "QQBot tok" {
		t.Errorf("json token: got %v, want %q", raw["token"], "QQBot tok")
	}
	if _, ok := raw["intents"]; !ok {
		t.Error("json missing 'intents' field")
	}
	if _, ok := raw["shard"]; !ok {
		t.Error("json missing 'shard' field")
	}
}

func TestResumeParamsJSON(t *testing.T) {
	p := ResumeParams{
		Token:     "QQBot test-token",
		SessionID: "session-123",
		Seq:       42,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal ResumeParams: %v", err)
	}

	var decoded ResumeParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal ResumeParams: %v", err)
	}

	if decoded.Token != p.Token {
		t.Errorf("Token mismatch: got %q, want %q", decoded.Token, p.Token)
	}
	if decoded.SessionID != p.SessionID {
		t.Errorf("SessionID mismatch: got %q, want %q", decoded.SessionID, p.SessionID)
	}
	if decoded.Seq != p.Seq {
		t.Errorf("Seq mismatch: got %d, want %d", decoded.Seq, p.Seq)
	}
}

func TestResumeParamsJSONFields(t *testing.T) {
	p := ResumeParams{
		Token:     "QQBot tok",
		SessionID: "sess-abc",
		Seq:       99,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw: %v", err)
	}

	if raw["token"] != "QQBot tok" {
		t.Errorf("json token: got %v, want %q", raw["token"], "QQBot tok")
	}
	if raw["session_id"] != "sess-abc" {
		t.Errorf("json session_id: got %v, want %q", raw["session_id"], "sess-abc")
	}
	if raw["seq"] != float64(99) {
		t.Errorf("json seq: got %v, want 99", raw["seq"])
	}
}

func TestOpCodeConstants(t *testing.T) {
	if OpDispatch != 0 {
		t.Errorf("OpDispatch: got %d, want 0", OpDispatch)
	}
	if OpHeartbeat != 1 {
		t.Errorf("OpHeartbeat: got %d, want 1", OpHeartbeat)
	}
	if OpIdentify != 2 {
		t.Errorf("OpIdentify: got %d, want 2", OpIdentify)
	}
	if OpResume != 6 {
		t.Errorf("OpResume: got %d, want 6", OpResume)
	}
	if OpReconnect != 7 {
		t.Errorf("OpReconnect: got %d, want 7", OpReconnect)
	}
	if OpInvalidSession != 9 {
		t.Errorf("OpInvalidSession: got %d, want 9", OpInvalidSession)
	}
	if OpHeartbeatACK != 11 {
		t.Errorf("OpHeartbeatACK: got %d, want 11", OpHeartbeatACK)
	}
}

func TestEventConstants(t *testing.T) {
	tests := []struct {
		name  string
		got   string
		want  string
	}{
		{"Ready", EventReady, "READY"},
		{"Resumed", EventResumed, "RESUMED"},
		{"C2C", EventC2CMessage, "C2C_MESSAGE_CREATE"},
		{"Group", EventGroupMessage, "GROUP_AT_MESSAGE_CREATE"},
		{"Guild", EventGuildMessage, "GUILD_MESSAGE_CREATE"},
		{"GuildDM", EventGuildDM, "DIRECT_MESSAGE_CREATE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestDefaultIntentLevels(t *testing.T) {
	levels := types.DefaultIntentLevels
	if len(levels) != 3 {
		t.Fatalf("expected 3 intent levels, got %d", len(levels))
	}

	if levels[0].Name != "full" {
		t.Errorf("level 0 name: got %q, want %q", levels[0].Name, "full")
	}
	if levels[0].Priority != 0 {
		t.Errorf("level 0 priority: got %d, want 0", levels[0].Priority)
	}
	if levels[1].Name != "group_and_guild" {
		t.Errorf("level 1 name: got %q, want %q", levels[1].Name, "group_and_guild")
	}
	if levels[2].Name != "guild_only" {
		t.Errorf("level 2 name: got %q, want %q", levels[2].Name, "guild_only")
	}

	// Level 0 (full) should include all intents
	if levels[0].Intents&(types.IntentGuilds|types.IntentGuildMembers|types.IntentDirectMessage|types.IntentGroupAndC2C|types.IntentPublicGuildMessages) == 0 {
		t.Error("level 0 should include all intents")
	}

	// Level 1 should not include DirectMessage or GuildMembers
	if levels[1].Intents&types.IntentDirectMessage != 0 {
		t.Error("level 1 should not include DirectMessage")
	}

	// Level 2 should only have Guilds and PublicGuildMessages
	if levels[2].Intents != types.IntentGuilds|types.IntentPublicGuildMessages {
		t.Errorf("level 2 intents: got %d, want %d", levels[2].Intents, types.IntentGuilds|types.IntentPublicGuildMessages)
	}
}

func TestHelloPayloadParsing(t *testing.T) {
	raw := `{"op":10,"d":{"heartbeat_interval":41250}}`
	var payload types.WSPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Op != 10 {
		t.Errorf("Op: got %d, want 10", payload.Op)
	}
}
