package gateway

// WebSocket opcode constants matching the QQ Bot Gateway protocol.
const (
	OpDispatch       = 0  // Event dispatch
	OpHeartbeat      = 1  // Heartbeat
	OpIdentify       = 2  // Identify
	OpResume         = 6  // Resume
	OpReconnect      = 7  // Reconnect
	OpInvalidSession = 9  // Invalid session
	OpHeartbeatACK   = 11 // Heartbeat ACK
	OpHello          = 10 // Hello (initial handshake)
)

// Gateway event type constants.
const (
	EventReady        = "READY"
	EventResumed      = "RESUMED"
	EventC2CMessage   = "C2C_MESSAGE_CREATE"
	EventGroupMessage = "GROUP_AT_MESSAGE_CREATE"
	EventGuildMessage = "GUILD_MESSAGE_CREATE"
	EventGuildDM      = "DIRECT_MESSAGE_CREATE"
)

// IdentifyParams is sent during the WebSocket Identify handshake.
type IdentifyParams struct {
	Token   string `json:"token"`
	Intents int    `json:"intents"`
	Shard   []int  `json:"shard"`
}

// ResumeParams is sent during the WebSocket Resume handshake.
type ResumeParams struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}
