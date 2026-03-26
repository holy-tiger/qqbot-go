# openclaw-qqbot

中文 | [English](README.md)

QQ Bot HTTP API 服务 — 将 QQ Bot 消息能力通过 RESTful HTTP API 暴露的独立 Go 服务。

## 功能特性

- **C2C / 群聊 / 频道消息** — 文本、图片、语音、视频、文件
- **主动消息** — 不依赖用户消息触发，主动向用户/群发送消息
- **定时提醒** — 创建、取消、查询定时消息任务
- **广播** — 向所有 C2C 用户或群发送广播
- **Webhook 事件转发** — 将用户消息通过 HTTP POST 实时推送到指定服务
- **多账号隔离** — 每个账号独立连接、队列和存储
- **图床服务** — 内置 HTTP 图片托管，支持本地图片转 URL 发送
- **SQLite 持久化** — 用户记录、会话信息自动存储

## 安装

### 下载预编译二进制

从 [GitHub Releases](https://github.com/holy-tiger/qqbot-go/releases) 下载对应平台的压缩包，解压后即可使用。

**Linux**

```bash
# x86_64
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_linux_x86_64.tar.gz | tar xz

# ARM64 (如树莓派、华为云鲲鹏)
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_linux_aarch64.tar.gz | tar xz
```

**macOS**

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_darwin_aarch64.tar.gz | tar xz

# Intel
curl -sL https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_darwin_x86_64.tar.gz | tar xz
```

**Windows**

```powershell
# x86_64
Invoke-WebRequest -Uri https://github.com/holy-tiger/qqbot-go/releases/latest/download/qqbot_windows_x86_64.zip -OutFile qqbot.zip
Expand-Archive qqbot.zip -DestinationPath .
```

> 所有版本的 SHA256 校验值见 [checksums.txt](https://github.com/holy-tiger/qqbot-go/releases/latest/download/checksums.txt)。

### 运行

```bash
# 配置
cp configs/config.example.yaml configs/config.yaml
# 编辑 config.yaml 填入 appId 和 clientSecret

# 启动
./qqbot -config configs/config.yaml -health :8080 -api :9090
```

### 从源码构建

需要 Go 1.24+ 和 ffmpeg/ffprobe（语音处理所需）：

```bash
git clone https://github.com/holy-tiger/qqbot-go.git
cd qqbot-go
go build -o qqbot ./cmd/qqbot
```

### CLI 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-config` | `configs/config.yaml` | 配置文件路径 |
| `-health` | `:8080` | 健康检查地址（留空禁用） |
| `-api` | `:9090` | HTTP API 地址（留空禁用） |

## 配置

配置文件示例见 [`configs/config.example.yaml`](configs/config.example.yaml)。

敏感信息支持三种方式设置（优先级从高到低）：

1. 环境变量：`QQBOT_APP_ID`、`QQBOT_CLIENT_SECRET`、`QQBOT_IMAGE_SERVER_BASE_URL`
2. 文件：`clientSecretFile` 指定 secret 文件路径
3. 配置文件：直接写在 YAML 中

多账号配置示例：

```yaml
qqbot:
  appId: "default-app-id"
  clientSecret: "default-secret"
  accounts:
    second-bot:
      appId: "second-app-id"
      clientSecret: "second-secret"
      name: "Second Bot"
```

完整配置参考：[`docs/zh/configuration.md`](docs/zh/configuration.md)。

## HTTP API

所有接口前缀 `/api/v1`，无需认证（适用于内网使用）。

### 消息发送

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/accounts/{id}/c2c/{openid}/messages` | 发送文本（C2C） |
| POST | `/api/v1/accounts/{id}/groups/{openid}/messages` | 发送文本（群） |
| POST | `/api/v1/accounts/{id}/channels/{channel_id}/messages` | 发送文本（频道） |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/images` | 发送图片（C2C） |
| POST | `/api/v1/accounts/{id}/groups/{openid}/images` | 发送图片（群） |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/voice` | 发送语音（C2C） |
| POST | `/api/v1/accounts/{id}/groups/{openid}/voice` | 发送语音（群） |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/videos` | 发送视频（C2C） |
| POST | `/api/v1/accounts/{id}/groups/{openid}/videos` | 发送视频（群） |
| POST | `/api/v1/accounts/{id}/c2c/{openid}/files` | 发送文件（C2C） |
| POST | `/api/v1/accounts/{id}/groups/{openid}/files` | 发送文件（群） |

### 主动消息与广播

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/accounts/{id}/proactive/c2c/{openid}` | 主动发文本（C2C） |
| POST | `/api/v1/accounts/{id}/proactive/groups/{openid}` | 主动发文本（群） |
| POST | `/api/v1/accounts/{id}/broadcast` | 广播到所有 C2C 用户 |
| POST | `/api/v1/accounts/{id}/broadcast/groups` | 广播到所有群 |

### 定时提醒

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/accounts/{id}/reminders` | 创建提醒 |
| DELETE | `/api/v1/accounts/{id}/reminders/{rem_id}` | 取消提醒 |
| GET | `/api/v1/accounts/{id}/reminders` | 查询所有提醒 |

### 用户与状态

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/accounts/{id}/users` | 查询已知用户 |
| GET | `/api/v1/accounts/{id}/users/stats` | 用户统计 |
| DELETE | `/api/v1/accounts/{id}/users` | 清空用户记录 |
| GET | `/api/v1/accounts` | 所有账号状态 |
| GET | `/api/v1/accounts/{id}` | 单账号状态 |
| GET | `/health` | 健康检查 |

### 响应格式

```json
{"ok": true, "data": { ... }}
{"ok": false, "error": "error message"}
```

### Webhook 事件转发

配置 `defaultWebhookUrl` 或 `webhookUrl` 后，用户消息将通过 HTTP POST 推送：

```json
{
  "account_id": "default",
  "event_type": "C2C_MESSAGE_CREATE",
  "timestamp": "2026-03-25T12:00:00Z",
  "data": { ... }
}
```

异步投递，最多 3 次重试，指数退避（1s, 2s, 4s）。

完整 API 文档：[`docs/zh/api.md`](docs/zh/api.md)。

## 项目结构

```
cmd/qqbot/          CLI 入口
configs/            YAML 配置
internal/
  api/              QQ Bot REST API 客户端
  audio/            音频处理（SILK 编解码、STT、格式转换）
  config/           配置加载与多账号解析
  gateway/          WebSocket 网关（心跳、重连、消息队列）
  httpapi/          HTTP API 服务与 Webhook 调度
  image/            图床服务与图片尺寸解析
  outbound/         出站消息处理、限速、媒体标签解析
  proactive/        主动消息与定时调度
  qqbot/            顶层编排（BotManager、健康检查）
  store/            SQLite 持久化存储
  types/            核心领域类型
  utils/            工具函数
```

架构设计：[`docs/zh/architecture.md`](docs/zh/architecture.md)。

## 开发

```bash
# 测试
go test -race -count=1 ./...

# 静态分析
go vet ./...

# 编译
go build -o qqbot ./cmd/qqbot
```

## 依赖

| 依赖 | 用途 |
|------|------|
| `golang.org/x/sync` | singleflight（令牌去重） |
| `gopkg.in/yaml.v3` | YAML 配置解析 |
| `github.com/gorilla/websocket` | WebSocket 连接 |
| `modernc.org/sqlite` | SQLite（纯 Go，无需 CGO） |
| `ffmpeg` / `ffprobe` | 音频处理（运行时依赖） |

## License

MIT
