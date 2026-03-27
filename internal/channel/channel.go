package channel

import (
	"context"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ChannelServer wraps an MCP server and configuration.
type ChannelServer struct {
	mcp              *server.MCPServer
	config           Config
	sender           Sender
	pushNotification func(source, sender, chatID, content string)
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
	cs := &ChannelServer{config: cfg}

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
		server.WithExperimental(map[string]any{"claude/channel": struct{}{}}),
		server.WithInstructions(`QQ 机器人消息通道。
消息以 <channel source="qq" sender="openid" chat_id="格式"> 标签到达。
- chat_id 格式: "c2c:user_openid" (私聊) 或 "group:group_openid" (群聊) 或 "channel:channel_id" (频道)
- 群聊消息中 @机器人 的部分已被自动去除
- 附件信息以 [图片/语音/视频/文件: url] 格式附加在文本末尾

用 reply 工具回复消息。`),
	)

	cs.registerReplyTool()

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
			mcp.Description("要发送的文本内容。纯文本消息时为消息正文；媒体消息时为图片说明/文件描述等"),
		),
		mcp.WithString("media_type",
			mcp.Description("媒体类型，可选: image (图片), file (文件), voice (语音), video (视频)。不设置则发送纯文本"),
		),
		mcp.WithString("media_url",
			mcp.Description("媒体文件的 URL。media_type 为 image/file/video 时必填"),
		),
	)
	cs.mcp.AddTool(tool, cs.handleReply)
}
