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
	pushNotification func(source, sender, chatID, content string)
}

// newChannelServer creates a ChannelServer with MCP server initialized.
// This is the shared setup used by both Run() and integration tests.
func newChannelServer(cfg Config) *ChannelServer {
	cs := &ChannelServer{config: cfg}

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

// Run creates and starts the channel server.
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
		mcp.WithDescription("回复 QQ 消息。通过 qqbot HTTP API 发送文本消息到指定的会话。"),
		mcp.WithString("chat_id",
			mcp.Required(),
			mcp.Description("会话 ID，格式: c2c:user_openid (私聊) 或 group:group_openid (群聊) 或 channel:channel_id (频道) 或 dm:channel_id (频道私信)"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("要发送的回复文本"),
		),
	)
	cs.mcp.AddTool(tool, cs.handleReply)
}
