# QQ Channel Server 方案设计

## 概述

将 qqbot 项目扩展为一个 MCP Channel Server，使 CodeBuddy Code 能够通过 QQ 机器人收发消息，实现双向通信。

## 背景

CodeBuddy Code 的 Channels 功能允许通过 MCP 协议将外部事件推送到会话中，并支持双向回复。目前官方支持微信（内置）、Telegram、Discord 等平台，不包含 QQ。本方案基于 `github.com/mark3labs/mcp-go` 库，为 qqbot 添加 MCP Channel 模式。

## 架构

```
QQ 用户
  │
  ▼
QQ Bot Gateway (WebSocket)
  │
  ▼
qqbot (Go) ── 内嵌 Channel Server ── MCP stdio ──▶ CodeBuddy Code
  │                                              │
  │         reply 工具 ◀─────────────────────────┘
  │              │
  ▼              ▼
QQ Bot API ◀── HTTP POST (发送回复)
```

### 核心思路

将 MCP Channel Server 作为 qqbot 的一个**子命令**（`qqbot channel`），与现有的 HTTP API 服务并行或独立运行。Channel Server 通过 stdio 与 CodeBuddy Code 通信，同时监听本地 HTTP 端口接收 qqbot 的 webhook 事件转发。

### 两种运行模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **独立模式** | `qqbot channel` 单独运行，接收 qqbot 主进程的 webhook | 两个进程，灵活部署 |
| **内嵌模式** | 在 qqbot 主进程中嵌入 Channel Server | 单进程，简单部署 |

本方案优先实现**独立模式**，内嵌模式作为后续优化。

## 技术选型

### MCP 库

- **库**: `github.com/mark3labs/mcp-go` (v0.46.0+)
- **传输层**: stdio（CodeBuddy Code 将 Channel 作为子进程启动）
- **Star**: 8.5k，社区活跃，API 完整

### 关键 API

```go
// 创建 MCP Server
s := server.NewMCPServer("qq-channel", "1.0.0",
    server.WithToolCapabilities(false),
    server.WithExperimental(map[string]any{"claude/channel": {}}),
    server.WithInstructions("QQ 机器人消息通道..."),
)

// 向 CodeBuddy 推送通知（核心）
// SendNotificationToAllClients 为 void 返回，尽力投递语义
s.SendNotificationToAllClients("notifications/claude/channel", map[string]any{
    "content": "用户消息文本",
    "meta": map[string]string{
        "source":  "qq",
        "sender":  "user_openid",
        "chat_id": "c2c:openid",
    },
})

// 暴露 reply 工具
s.AddTool(replyTool, replyHandler)

// 启动 stdio 服务
server.ServeStdio(s)
```

## 详细设计

### 1. 新增包: `internal/channel/`

```
internal/channel/
  channel.go        # Channel Server 主逻辑（ChannelServer 结构体、Run、pushNotification）
  webhook.go        # HTTP 服务，接收 qqbot 的 webhook 事件（含 stripAtMention）
  reply.go          # reply 工具定义与处理
  message.go        # 消息格式转换（appendAttachmentInfo）
  config.go         # Channel 配置
  webhook_test.go   # Webhook 单元测试
  reply_test.go     # reply 工具测试
  channel_test.go   # Channel Server 边界测试
```

### 2. CLI 入口

新增子命令 `channel`，在 `cmd/qqbot/main.go` 中或新建 `cmd/qqbot-channel/main.go`：

```go
// cmd/qqbot-channel/main.go
package main

import (
    "flag"
    "log"

    "github.com/openclaw/qqbot/internal/channel"
)

func main() {
    webhookPort := flag.Int("webhook-port", 8788, "webhook HTTP 监听端口")
    qqbotAPI     := flag.String("qqbot-api", "http://127.0.0.1:9090", "qqbot HTTP API 地址")
    account      := flag.String("account", "default", "默认 qqbot 账号 ID")
    flag.Parse()

    cfg := channel.Config{
        WebhookPort: *webhookPort,
        QQBotAPI:    *qqbotAPI,
        Account:     *account,
    }

    if err := channel.Run(cfg); err != nil {
        log.Fatalf("channel: %v", err)
    }
}
```

### 3. Channel Server 核心 (`internal/channel/channel.go`)

```go
package channel

import (
    "context"
    "log"

    "github.com/mark3labs/mcp-go/server"
)

// Config holds the channel server configuration.
type Config struct {
    WebhookPort int    // HTTP 端口，接收 qqbot webhook
    QQBotAPI    string // qqbot HTTP API 地址
    Account     string // 默认账号 ID
}

// ChannelServer 封装 MCP Server 和配置，避免全局变量。
type ChannelServer struct {
    mcp    *server.MCPServer
    config Config
}

// Run 创建并启动 channel server。
func Run(cfg Config) error {
    cs := &ChannelServer{config: cfg}

    // 创建 MCP Server，声明 claude/channel capability
    cs.mcp = server.NewMCPServer(
        "qq-channel",
        "1.0.0",
        server.WithToolCapabilities(false),
        server.WithExperimental(map[string]any{"claude/channel": {}}),
        server.WithInstructions(`QQ 机器人消息通道。
消息以 <channel source="qq" sender="openid" chat_id="格式"> 标签到达。
- chat_id 格式: "c2c:user_openid" (私聊) 或 "group:group_openid" (群聊) 或 "channel:channel_id" (频道)
- 群聊消息中 @机器人 的部分已被自动去除
- 附件信息以 [图片/语音/视频/文件: url] 格式附加在文本末尾

用 reply 工具回复消息。`),
    )

    // 注册 reply 工具
    cs.registerReplyTool()

    // 使用 context 控制 webhook 生命周期，stdio 断开时一起退出
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 启动 HTTP webhook 服务（后台 goroutine）
    go cs.startWebhookServer(ctx)

    // 启动 MCP stdio 服务（阻塞），返回时触发 webhook shutdown
    log.Printf("[channel] starting stdio server (webhook on :%d, qqbot API at %s)",
        cfg.WebhookPort, cfg.QQBotAPI)
    err := server.ServeStdio(cs.mcp)
    cancel() // stdio 结束，通知 webhook 退出
    return err
}

// pushNotification 向 CodeBuddy Code 推送 channel 通知。
// SendNotificationToAllClients 为 void 返回（尽力投递语义），无需检查 error。
func (cs *ChannelServer) pushNotification(source, sender, chatID, content string) {
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
```

### 4. Webhook HTTP 服务 (`internal/channel/webhook.go`)

接收 qqbot `WebhookDispatcher` 转发的事件，转换为 MCP 通知：

```go
package channel

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"

    "github.com/openclaw/qqbot/internal/types"
)

// webhookEvent 与 qqbot 的 WebhookEvent 结构一致。
type webhookEvent struct {
    AccountID string          `json:"account_id"`
    EventType string          `json:"event_type"`
    Timestamp string          `json:"timestamp"`
    Data      json.RawMessage `json:"data"`
}

// handledEventTypes 定义 channel 处理的事件白名单。
// qqbot 的 WebhookDispatcher 会转发所有事件类型，这里仅处理消息类事件。
var handledEventTypes = map[string]bool{
    "C2C_MESSAGE_CREATE":       true,
    "GROUP_AT_MESSAGE_CREATE":  true,
    "GUILD_MESSAGE_CREATE":     true,
    "DIRECT_MESSAGE_CREATE":    true,
}

func (cs *ChannelServer) startWebhookServer(ctx context.Context) {
    mux := http.NewServeMux()
    mux.HandleFunc("/webhook", cs.handleWebhook)
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, `{"ok":true}`)
    })

    addr := fmt.Sprintf("127.0.0.1:%d", cs.config.WebhookPort)
    srv := &http.Server{Addr: addr, Handler: mux}

    log.Printf("[channel] webhook server listening on %s", addr)

    go func() {
        <-ctx.Done()
        log.Printf("[channel] shutting down webhook server")
        srv.Shutdown(context.Background())
    }()

    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("[channel] webhook server error: %v", err)
    }
}

func (cs *ChannelServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var event webhookEvent
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        log.Printf("[channel] invalid webhook payload: %v", err)
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }

    w.WriteHeader(http.StatusOK)

    if !handledEventTypes[event.EventType] {
        return
    }

    switch event.EventType {
    case "C2C_MESSAGE_CREATE":
        cs.handleC2CMessage(event)
    case "GROUP_AT_MESSAGE_CREATE":
        cs.handleGroupMessage(event)
    case "GUILD_MESSAGE_CREATE":
        cs.handleGuildMessage(event)
    case "DIRECT_MESSAGE_CREATE":
        cs.handleDirectMessage(event)
    }
}

func (cs *ChannelServer) handleC2CMessage(event webhookEvent) {
    var msg types.C2CMessageEvent
    if err := json.Unmarshal(event.Data, &msg); err != nil {
        log.Printf("[channel] parse c2c message error: %v", err)
        return
    }

    content := msg.Content
    content = appendAttachmentInfo(content, msg.Attachments)

    chatID := fmt.Sprintf("c2c:%s", msg.Author.UserOpenID)
    cs.pushNotification("qq", msg.Author.UserOpenID, chatID, content)
}

func (cs *ChannelServer) handleGroupMessage(event webhookEvent) {
    var msg types.GroupMessageEvent
    if err := json.Unmarshal(event.Data, &msg); err != nil {
        log.Printf("[channel] parse group message error: %v", err)
        return
    }

    // 去除 @机器人 部分
    content := stripAtMention(msg.Content)
    content = appendAttachmentInfo(content, msg.Attachments)

    chatID := fmt.Sprintf("group:%s", msg.GroupOpenID)
    cs.pushNotification("qq", msg.Author.MemberOpenID, chatID, content)
}

func (cs *ChannelServer) handleGuildMessage(event webhookEvent) {
    var msg types.GuildMessageEvent
    if err := json.Unmarshal(event.Data, &msg); err != nil {
        log.Printf("[channel] parse guild message error: %v", err)
        return
    }

    content := stripAtMention(msg.Content)
    content = appendAttachmentInfo(content, msg.Attachments)

    chatID := fmt.Sprintf("channel:%s", msg.ChannelID)
    cs.pushNotification("qq", msg.Author.ID, chatID, content)
}

// handleDirectMessage 处理频道私信事件（DIRECT_MESSAGE_CREATE）。
// 注意：频道私信可能与 GuildMessageEvent 结构不同，需要确认 QQ API 实际 payload。
// 目前暂用 GuildMessageEvent 解析，如不匹配需定义专门的 DirectMessageEvent 类型。
func (cs *ChannelServer) handleDirectMessage(event webhookEvent) {
    var msg types.GuildMessageEvent
    if err := json.Unmarshal(event.Data, &msg); err != nil {
        log.Printf("[channel] parse direct message error: %v", err)
        return
    }

    content := stripAtMention(msg.Content)
    content = appendAttachmentInfo(content, msg.Attachments)

    // 频道私信使用 dm: 前缀区分于频道消息
    chatID := fmt.Sprintf("dm:%s", msg.ChannelID)
    cs.pushNotification("qq", msg.Author.ID, chatID, content)
}

// stripAtMention 去除消息开头的 @机器人 提及。
func stripAtMention(content string) string {
    // 1. QQ 群消息 Mention 格式: "<@!user_id> 后续内容"
    if strings.HasPrefix(content, "<@!") {
        if idx := strings.Index(content, ">"); idx != -1 {
            return strings.TrimSpace(content[idx+1:])
        }
    }
    // 2. QQ 群消息 @昵称 格式: "@机器人昵称\x00" 或 "@机器人昵称 内容"
    if strings.HasPrefix(content, "@") {
        if idx := strings.Index(content, "\u0000"); idx != -1 {
            return strings.TrimSpace(content[idx+1:])
        }
        if idx := strings.Index(content, " "); idx != -1 {
            return strings.TrimSpace(content[idx+1:])
        }
    }
    return content
}
```

### 5. reply 工具 (`internal/channel/reply.go`)

CodeBuddy Code 通过 `reply` 工具将回复发送回 QQ：

```go
package channel

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/mark3labs/mcp-go/mcp"
)

// registerReplyTool 注册 reply 工具到 MCP Server。
func (cs *ChannelServer) registerReplyTool() {
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

// handleReply 处理 CodeBuddy Code 的 reply 工具调用。
func (cs *ChannelServer) handleReply(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    chatID, err := request.RequireString("chat_id")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }
    text, err := request.RequireString("text")
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    // 解析 chat_id: "c2c:openid" / "group:group_openid" / "channel:channel_id" / "dm:channel_id"
    parts := strings.SplitN(chatID, ":", 2)
    if len(parts) != 2 {
        return mcp.NewToolResultError(fmt.Sprintf("invalid chat_id format: %s", chatID)), nil
    }

    msgType, targetID := parts[0], parts[1]
    var apiPath string
    switch msgType {
    case "c2c":
        apiPath = fmt.Sprintf("/api/v1/accounts/%s/c2c/%s/messages", cs.config.Account, targetID)
    case "group":
        apiPath = fmt.Sprintf("/api/v1/accounts/%s/groups/%s/messages", cs.config.Account, targetID)
    case "channel", "dm":
        // 频道消息和频道私信共用 /channels/ API
        apiPath = fmt.Sprintf("/api/v1/accounts/%s/channels/%s/messages", cs.config.Account, targetID)
    default:
        return mcp.NewToolResultError(fmt.Sprintf("unknown chat type: %s", msgType)), nil
    }

    // 调用 qqbot HTTP API
    body, _ := json.Marshal(map[string]string{"content": text})
    req, _ := http.NewRequest(http.MethodPost, cs.config.QQBotAPI+apiPath, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("[channel] reply error: %v", err)
        return mcp.NewToolResultError(fmt.Sprintf("发送失败: %v", err)), nil
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        log.Printf("[channel] reply HTTP %d for %s", resp.StatusCode, apiPath)
        return mcp.NewToolResultError(fmt.Sprintf("发送失败: HTTP %d", resp.StatusCode)), nil
    }

    log.Printf("[channel] replied to %s (%d bytes)", chatID, len(text))
    return mcp.NewToolResultText("sent"), nil
}
```

### 6. 消息格式转换 (`internal/channel/message.go`)

将 QQ 事件中的消息内容、附件信息构建为推送给 CodeBuddy 的纯文本：

```go
package channel

import (
    "fmt"
    "strings"

    "github.com/openclaw/qqbot/internal/types"
)

// appendAttachmentInfo 将附件信息附加到消息文本末尾。
func appendAttachmentInfo(content string, attachments []types.MessageAttachment) string {
    if len(attachments) == 0 {
        return content
    }
    var sb strings.Builder
    sb.WriteString(content)
    for _, att := range attachments {
        switch att.ContentType {
        case "image":
            sb.WriteString(fmt.Sprintf("\n[图片: %s]", att.URL))
        case "voice":
            url := att.URL
            if att.VoiceWavURL != nil && *att.VoiceWavURL != "" {
                url = *att.VoiceWavURL
            }
            sb.WriteString(fmt.Sprintf("\n[语音: %s]", url))
            if att.ASRReferText != nil && *att.ASRReferText != "" {
                sb.WriteString(fmt.Sprintf(" (识别: %s)", *att.ASRReferText))
            }
        case "video":
            sb.WriteString(fmt.Sprintf("\n[视频: %s]", att.URL))
        case "file":
            name := "未知文件"
            if att.Filename != nil {
                name = *att.Filename
            }
            sb.WriteString(fmt.Sprintf("\n[文件: %s — %s]", name, att.URL))
        }
    }
    return sb.String()
}
```

### 7. 消息格式说明

#### CodeBuddy 收到的消息格式

```xml
<channel source="qq" sender="user_openid" chat_id="c2c:user_openid">
你好，帮我看一下这个 bug
</channel>
```

群聊消息（@机器人 部分已自动去除）：

```xml
<channel source="qq" sender="member_openid" chat_id="group:group_openid">
帮我看一下这个 bug
[图片: https://example.com/image.png]
</channel>
```

#### chat_id 编码规则

| 场景 | chat_id 格式 | 示例 |
|------|-------------|------|
| 私聊 | `c2c:{user_openid}` | `c2c:o_xxxxxx` |
| 群聊 | `group:{group_openid}` | `group:o_groupxxxxx` |
| 频道消息 | `channel:{channel_id}` | `channel:12345678` |
| 频道私信 | `dm:{channel_id}` | `dm:87654321` |

## 配置与使用

### 1. 编译

```bash
# 编译 qqbot 主程序（不变）
go build -o qqbot ./cmd/qqbot

# 编译 channel 子命令
go build -o qqbot-channel ./cmd/qqbot-channel
```

### 2. 配置 qqbot 的 webhook 转发

修改 `configs/config.yaml`：

```yaml
qqbot:
  defaultWebhookUrl: "http://127.0.0.1:8788/webhook"
```

### 3. 注册到 CodeBuddy Code 的 `.mcp.json`

在项目根目录或用户目录创建 `.mcp.json`：

```json
{
  "mcpServers": {
    "qq-channel": {
      "command": "./qqbot-channel",
      "args": ["-qqbot-api", "http://127.0.0.1:9090", "-account", "default"]
    }
  }
}
```

### 4. 启动

```bash
# 终端 1: 启动 qqbot 主服务
./qqbot -config configs/config.yaml -api :9090

# 终端 2: 启动 CodeBuddy Code，加载 qq channel
codebuddy --dangerously-load-development-channels server:qq-channel
```

### 5. 使用流程

1. QQ 用户向机器人发送消息
2. qqbot 收到消息，通过 webhook 转发到 `http://127.0.0.1:8788/webhook`
3. Channel Server 解析事件，通过 MCP stdio 推送通知到 CodeBuddy Code
4. CodeBuddy Code 显示: `#qq · user_openid: 你好`
5. CodeBuddy Code 处理后调用 `reply` 工具
6. Channel Server 调用 qqbot HTTP API 发送回复
7. QQ 用户收到回复

## 安全考虑

### 发送者验证

Channel Server 运行在本地，webhook 仅监听 `127.0.0.1`，外部无法直接访问。qqbot 的 webhook 转发已经包含 `account_id` 和事件签名，信任来源即可。

如果需要额外的安全层，可以在 webhook handler 中验证请求来源 IP 或添加 token：

```go
// 可选: webhook token 验证
const webhookToken = "your-secret-token"

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    if r.Header.Get("X-Webhook-Token") != webhookToken {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    // ...
}
```

### 权限中继

CodeBuddy Channel 支持权限中继（`claude/channel/permission`），允许通过 QQ 远程审批工具调用。如需启用：

```go
server.WithExperimental(map[string]any{
    "claude/channel":            {},
    "claude/channel/permission": {},
})
```

然后在 webhook handler 中识别 `yes <id>` / `no <id>` 格式的回复并转发权限裁决通知。

## 依赖变更

```bash
go get github.com/mark3labs/mcp-go@latest
```

`go.mod` 新增依赖：
```
github.com/mark3labs/mcp-go v0.46.0
```

## 测试策略

### 单元测试

- `webhook_test.go`: 测试事件解析、@去除、附件格式化、并发请求
- `reply_test.go`: 测试 reply 工具的 chat_id 解析和 API 调用（mock HTTP）
- `message_test.go`: 测试各种消息类型的转换
- `channel_test.go`: 测试 mcpServer 未就绪时的消息丢弃行为

```go
// internal/channel/webhook_test.go 示例
func TestStripAtMention(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"@Bot 你好", "你好"},                       // @昵称 + 空格
        {"@Bot\x00你好", "你好"},                    // @昵称 + null
        {"<@!123456> 你好", "你好"},                  // Mention 格式
        {"<@!123456>", ""},                          // Mention 无后续内容
        {"你好", "你好"},                             // 无提及
        {"@Bot你 好", "@Bot你 好"},                   // @昵称后无分隔符，不处理
    }
    for _, tt := range tests {
        got := stripAtMention(tt.input)
        if got != tt.expected {
            t.Errorf("stripAtMention(%q) = %q, want %q", tt.input, got, tt.expected)
        }
    }
}

func TestAppendAttachmentInfo(t *testing.T) {
    content := appendAttachmentInfo("看看这个", []types.MessageAttachment{
        {ContentType: "image", URL: "https://example.com/img.png"},
        {ContentType: "voice", URL: "https://example.com/voice.silk",
            VoiceWavURL:  strPtr("https://example.com/voice.wav"),
            ASRReferText: strPtr("你好")},
    })
    expected := "看看这个\n[图片: https://example.com/img.png]\n[语音: https://example.com/voice.wav] (识别: 你好)"
    if content != expected {
        t.Errorf("got %q, want %q", content, expected)
    }
}

func TestWebhookMethodNotAllowed(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
    w := httptest.NewRecorder()
    handleWebhook(w, req)
    if w.Code != http.StatusMethodNotAllowed {
        t.Errorf("expected 405, got %d", w.Code)
    }
}

func TestWebhookInvalidJSON(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/webhook",
        strings.NewReader("not json"))
    w := httptest.NewRecorder()
    handleWebhook(w, req)
    if w.Code != http.StatusBadRequest {
        t.Errorf("expected 400, got %d", w.Code)
    }
}

func TestWebhookIgnoresUnhandledEventTypes(t *testing.T) {
    event := webhookEvent{EventType: "GUILD_MEMBER_ADD", Data: json.RawMessage(`{}`)}
    body, _ := json.Marshal(event)
    req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
    w := httptest.NewRecorder()
    handleWebhook(w, req)
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

### 集成测试

使用 `mcp-go` 的 `NewTestServer` 创建测试 HTTP 服务，模拟完整的 MCP 通信流程。

### 边界测试

| 场景 | 测试方法 |
|------|---------|
| webhook 并发请求 | 使用 `sync.WaitGroup` + 并发 POST，验证不会 panic 或 data race |
| MCP server 未就绪时收到 webhook | 构造 `ChannelServer{mcp: nil}`，调用 `pushNotification`，验证不 panic |
| reply HTTP 失败 | 使用 `httptest.NewServer` 返回 500，验证返回 `NewToolResultError` |
| reply chat_id 格式错误 | 测试 `"invalid"`, `"c2c"` (缺少冒号后部分) 等输入 |
| graceful shutdown | 启动 webhook server，cancel context，验证 `ErrServerClosed` |

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/channel/channel.go` | 新增 | Channel Server 主逻辑（ChannelServer 结构体） |
| `internal/channel/webhook.go` | 新增 | Webhook HTTP 服务（含事件白名单、graceful shutdown） |
| `internal/channel/reply.go` | 新增 | reply 工具 |
| `internal/channel/message.go` | 新增 | 消息格式转换（appendAttachmentInfo） |
| `internal/channel/config.go` | 新增 | 配置定义 |
| `internal/channel/webhook_test.go` | 新增 | Webhook 单元测试 |
| `internal/channel/reply_test.go` | 新增 | reply 工具测试 |
| `internal/channel/channel_test.go` | 新增 | Channel Server 边界测试 |
| `cmd/qqbot-channel/main.go` | 新增 | CLI 入口 |
| `.mcp.json` | 新增 | MCP 服务器注册配置 |
| `go.mod` / `go.sum` | 修改 | 新增 mcp-go 依赖 |
| `docs/en/configuration.md` | 修改 | 补充 channel 配置说明 |

## 后续优化

1. **内嵌模式**: 将 Channel Server 嵌入 qqbot 主进程，单二进制部署
2. **富媒体回复**: 支持 reply 工具发送图片、文件等富媒体（调用 `/images`、`/files` API）
3. **多账号支持**: 根据事件中的 `account_id` 自动路由到正确的 QQ 账号
4. **权限中继**: 实现 `claude/channel/permission` 支持远程审批
5. **打包为插件**: 按照插件规范打包，支持 `/plugin install` 安装
