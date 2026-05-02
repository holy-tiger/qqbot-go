package channel

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ChannelServer wraps an MCP server and configuration.
type ChannelServer struct {
	mcp              *server.MCPServer
	config           Config
	sender           Sender
	pushNotification func(source, sender, chatID, content string)
	chatIDMu         sync.Mutex
	chatIDMap        map[string]string // sender -> chat_id
	lastSender       string            // most recent sender (for permission routing)
}

// RunOption configures the ChannelServer.
type RunOption func(*ChannelServer)

// WithSender sets the Sender for message sending.
// If not set, defaults to httpSender using config.QQBotAPI.
func WithSender(s Sender) RunOption {
	return func(cs *ChannelServer) {
		cs.sender = s
	}
}

// newChannelServer creates a ChannelServer with MCP server initialized.
// This is the shared setup used by both Run() and integration tests.
func newChannelServer(cfg Config, opts ...RunOption) *ChannelServer {
	cs := &ChannelServer{config: cfg, chatIDMap: make(map[string]string)}

	for _, opt := range opts {
		opt(cs)
	}
	if cs.sender == nil {
		cs.sender = newHTTPSender(cfg.QQBotAPI)
	}

	cs.mcp = server.NewMCPServer(
		"qq-channel",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithExperimental(map[string]any{
			"claude/channel":             struct{}{},
			"claude/channel/permission":  struct{}{},
		}),
		server.WithInstructions(`QQ 机器人消息通道。
消息以 <channel source="qq" sender="openid" chat_id="格式"> 标签到达。
- chat_id 格式: "c2c:user_openid" (私聊) 或 "group:group_openid" (群聊) 或 "channel:channel_id" (频道)
- 群聊消息中 @机器人 的部分已被自动去除
- 附件信息以 [图片/语音/视频/文件: url] 格式附加在文本末尾
- 权限审批请求以 permission 标签到达，回复 "yes <id>" 或 "no <id>" 进行审批

用 reply 工具回复消息。`),
	)

	cs.registerReplyTool()
	cs.registerRemindTools()
	cs.registerPermissionHandler()

	cs.pushNotification = func(source, sender, chatID, content string) {
		if cs.mcp == nil {
			log.Printf("[channel] mcp server not ready, dropping message")
			return
		}
		cs.mcp.SendNotificationToAllClients("notifications/claude/channel", map[string]any{
			"content": content,
			"meta": map[string]string{
				"source":  source,
				"sender":  sender,
				"chat_id": chatID,
			},
		})
	}

	return cs
}

// Run creates and starts the channel server in standalone mode.
func Run(cfg Config) error {
	cs := newChannelServer(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cs.startWebhookServer(ctx)

	log.Printf("[channel] starting stdio server (webhook on :%d, qqbot API at %s)",
		cfg.WebhookPort, cfg.QQBotAPI)
	err := server.ServeStdio(cs.mcp)
	cancel()
	return err
}

// RunEmbedded creates and starts the channel server in embedded mode.
// The BotManager's event handler should be wired to PushNotification externally.
func RunEmbedded(cfgPath string, sender Sender) error {
	cfg := Config{Account: "default"}
	return newChannelServer(cfg, WithSender(sender)).ServeStdio()
}

// NewEmbedded creates a ChannelServer for embedded mode.
// The caller must wire events to PushNotification() and call Run().
func NewEmbedded(cfgPath string, sender Sender) *ChannelServer {
	cfg := Config{Account: "default"}
	return newChannelServer(cfg, WithSender(sender))
}

// ServeStdio serves the MCP server on stdio. Blocks until closed.
func (cs *ChannelServer) ServeStdio() error {
	log.Printf("[channel] starting embedded stdio server")
	return server.ServeStdio(cs.mcp)
}

// PushNotification sends a notification to all connected MCP clients.
// Used by the embedded mode to bridge BotManager events into MCP notifications.
func (cs *ChannelServer) PushNotification(source, sender, chatID, content string) {
	if cs.pushNotification != nil {
		cs.pushNotification(source, sender, chatID, content)
	}
}

// ForwardMessage routes a message: checks for permission replies first,
// otherwise forwards as a normal notification.
// It also tracks the sender->chat_id mapping for permission request routing.
func (cs *ChannelServer) ForwardMessage(sender, chatID, content string) {
	cs.chatIDMu.Lock()
	cs.chatIDMap[sender] = chatID
	cs.lastSender = sender
	cs.chatIDMu.Unlock()

	if verdict := ParsePermissionReply(content); verdict != nil {
		behavior := "deny"
		if verdict.Allowed {
			behavior = "allow"
		}
		log.Printf("[channel] permission verdict: %s -> %s", verdict.RequestID, behavior)
		cs.SendPermissionVerdict(verdict.RequestID, behavior)
		return
	}
	cs.PushNotification("qq", sender, chatID, content)
}

// SendPermissionVerdict sends a permission approval/denial to CodeBuddy Code.
func (cs *ChannelServer) SendPermissionVerdict(requestID, behavior string) {
	if cs.mcp == nil {
		return
	}
	cs.mcp.SendNotificationToAllClients("notifications/claude/channel/permission", map[string]any{
		"request_id": requestID,
		"behavior":   behavior,
	})
}

// PushPermissionRequest forwards a permission request from CodeBuddy Code to QQ.
// It sends a text message to the most recent sender's chat.
func (cs *ChannelServer) PushPermissionRequest(requestID, toolName, description, inputPreview string) {
	cs.chatIDMu.Lock()
	chatID := cs.chatIDMap[cs.lastSender]
	cs.chatIDMu.Unlock()
	if chatID == "" {
		log.Printf("[channel] no active chat_id, dropping permission request %s", requestID)
		return
	}

	content := fmt.Sprintf("[审批请求 %s] %s: %s", requestID, toolName, description)
	if inputPreview != "" {
		content += "\n" + inputPreview
	}
	content += "\n回复 yes " + requestID + " 或 no " + requestID

	chatType, targetID, err := parseChatID(chatID)
	if err != nil {
		log.Printf("[channel] invalid chat_id %q: %v", chatID, err)
		return
	}

	err = cs.sender.Send(context.Background(), cs.config.Account, chatType, targetID, content, "", "")
	if err != nil {
		log.Printf("[channel] failed to send permission request to QQ: %v", err)
	}
}

// registerPermissionHandler registers the notification handler for permission requests.
func (cs *ChannelServer) registerPermissionHandler() {
	if cs.mcp == nil {
		return
	}
	cs.mcp.AddNotificationHandler("notifications/claude/channel/permission_request",
		func(ctx context.Context, notification mcp.JSONRPCNotification) {
			fields := notification.Params.AdditionalFields
			requestID, _ := fields["request_id"].(string)
			toolName, _ := fields["tool_name"].(string)
			description, _ := fields["description"].(string)
			inputPreview, _ := fields["input_preview"].(string)
			if requestID == "" {
				return
			}
			log.Printf("[channel] permission request: %s %s (%s)", requestID, toolName, description)
			cs.PushPermissionRequest(requestID, toolName, description, inputPreview)
		})
}

// MCPServer returns the underlying MCP server. Used by integration tests.
func (cs *ChannelServer) MCPServer() *server.MCPServer {
	return cs.mcp
}

// registerReplyTool registers the reply tool on the MCP server.
func (cs *ChannelServer) registerReplyTool() {
	if cs.mcp == nil {
		return
	}
	tool := mcp.NewTool("reply",
		mcp.WithDescription("回复 QQ 消息。通过 qqbot HTTP API 发送文本或富媒体消息到指定的会话。不设置 media_type 则发送纯文本。"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("会话 ID，格式: c2c:user_openid (私聊) 或 group:group_openid (群聊) 或 channel:channel_id (频道) 或 dm:channel_id (频道私信)"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("要发送的文本内容。纯文本消息时为消息正文；图片/视频消息时为标题说明；voice 时作为 TTS 文本，服务端会自动用 edge-tts 将其转为语音发送，无需本端处理"),
		),
		mcp.WithString("media_type",
			mcp.Description("媒体类型，可选: image (图片), file (文件), voice (语音), video (视频)。不设置则发送纯文本。image/file/video 时需传 media_url；voice 时可省略 media_url（将 text 作为 TTS 内容）；channel/dm 只能发纯文本"),
		),
		mcp.WithString("media_url",
			mcp.Description("媒体文件的 URL。media_type 为 image/file/video 时必填。media_type 为 voice 时可选：不传则服务端自动将 text 转为语音发送(TTS)；传入时此字段不是 URL，而是 base64 编码的音频原始数据"),
		),
	)
	cs.mcp.AddTool(tool, cs.handleReply)
}
