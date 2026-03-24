# QQ Bot 消息发送 API 文档

本文档介绍如何使用 curl 命令直接调用 QQ Bot API 发送消息。

## 目录

- [前置条件](#前置条件)
- [获取 Access Token](#获取-access-token)
- [发送私聊消息](#发送私聊消息)
- [发送群聊消息](#发送群聊消息)
- [Markdown 消息格式](#markdown-消息格式)
- [发送图片消息](#发送图片消息)
- [发送其他媒体文件](#发送其他媒体文件)
- [回复消息](#回复消息)
- [完整示例脚本](#完整示例脚本)
- [常见错误](#常见错误)

---

## 前置条件

在调用 API 之前，你需要：

1. **AppID** - 机器人的 AppID
2. **ClientSecret** - 机器人的 ClientSecret
3. **用户 OpenID** - 目标用户的 OpenID（私聊）
4. **群 OpenID** - 目标群的 OpenID（群聊）

> 注意：用户的 OpenID 不是 QQ 号，需要用户先给 Bot 发过消息后才能获取。

---

## 获取 Access Token

所有 API 请求都需要 Access Token 进行认证。

### 请求

```bash
curl -X POST "https://bots.qq.com/app/getAppAccessToken" \
  -H "Content-Type: application/json" \
  -d '{
    "appId": "YOUR_APP_ID",
    "clientSecret": "YOUR_CLIENT_SECRET"
  }'
```

### 响应

```json
{
  "access_token": "xxxxxxxxxxxx",
  "expires_in": "7200"
}
```

- `access_token` - 用于后续 API 调用的令牌
- `expires_in` - 令牌有效期（秒），通常为 7200 秒（2 小时）

### 示例脚本

```bash
# 获取 token 并保存到变量
TOKEN_RESP=$(curl -s -X POST "https://bots.qq.com/app/getAppAccessToken" \
  -H "Content-Type: application/json" \
  -d '{"appId":"YOUR_APP_ID","clientSecret":"YOUR_CLIENT_SECRET"}')

ACCESS_TOKEN=$(echo "$TOKEN_RESP" | jq -r '.access_token')
echo "Access Token: $ACCESS_TOKEN"
```

---

## 发送私聊消息

### API 端点

```
POST https://api.sgroup.qq.com/v2/users/{openid}/messages
```

### 请求头

| Header | 值 |
|--------|-----|
| Authorization | `QQBot {access_token}` |
| Content-Type | `application/json` |

### 发送纯文本消息

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/messages" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "你好，这是一条测试消息！",
    "msg_type": 0
  }'
```

### 响应

```json
{
  "id": "ROBOT1.0_xxxxxxxxx",
  "timestamp": "2026-03-24T23:41:24+08:00",
  "ext_info": {
    "ref_idx": "REFIDX_xxxxxxxxx"
  }
}
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| openid | string | 是 | 目标用户的 OpenID |
| content | string | 是 | 消息内容（纯文本模式） |
| msg_type | int | 是 | 消息类型：0=纯文本，2=Markdown |
| msg_id | string | 否 | 被回复的消息 ID |
| msg_seq | int | 否 | 消息序号，用于回复 |

---

## 发送群聊消息

### API 端点

```
POST https://api.sgroup.qq.com/v2/groups/{group_openid}/messages
```

### 示例

```bash
curl -X POST "https://api.sgroup.qq.com/v2/groups/GROUP_OPENID/messages" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "大家好，Bot 上线了！",
    "msg_type": 0
  }'
```

---

## Markdown 消息格式

QQ Bot 支持 Markdown 格式的消息，可以发送富文本内容。

### 发送 Markdown 消息

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/messages" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "markdown": {
      "content": "# 标题\n\n**粗体文字**\n\n*斜体文字*\n\n- 列表项 1\n- 列表项 2\n\n`代码块`\n\n[链接](https://q.qq.com)"
    },
    "msg_type": 2
  }'
```

### 支持的 Markdown 语法

| 语法 | 效果 | 示例 |
|------|------|------|
| `# text` | 一级标题 | `# 欢迎使用` |
| `## text` | 二级标题 | `## 功能介绍` |
| `**text**` | 粗体 | `**重要通知**` |
| `*text*` | 斜体 | `*温馨提示*` |
| `- item` | 无序列表 | `- 功能 A` |
| `` `code` `` | 行内代码 | `` `print("hello")` `` |
| `[text](url)` | 链接 | `[点击跳转](https://q.qq.com)` |
| `![alt](url)` | 图片 | `![图片](https://example.com/img.png)` |

### Markdown 示例

```json
{
  "markdown": {
    "content": "# Bot 通知\n\n**时间**: 2026-03-24\n\n## 今日任务\n\n- [x] 任务 1\n- [ ] 任务 2\n- [ ] 任务 3\n\n> 这是一条引用文字\n\n如需帮助，请[点击这里](https://q.qq.com)"
  },
  "msg_type": 2
}
```

---

## 发送图片消息

发送图片需要先上传图片获取 `file_info`，然后发送消息。也可以使用 `srv_send_msg` 参数一步完成上传和发送。

### API 端点

```
POST https://api.sgroup.qq.com/v2/users/{openid}/files
POST https://api.sgroup.qq.com/v2/groups/{group_openid}/files
```

### 方式一：一步发送（推荐）

使用 `srv_send_msg: true` 参数，上传后自动发送：

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 1,
    "srv_send_msg": true,
    "url": "https://example.com/image.png"
  }'
```

### 方式二：分步发送

**步骤 1：上传图片获取 file_info**

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 1,
    "srv_send_msg": false,
    "url": "https://example.com/image.png"
  }'
```

**响应：**

```json
{
  "file_uuid": "xxxxxxxx",
  "file_info": "8Xr4A6jfKuZY4lCRUthGTb...",
  "ttl": 86400
}
```

**步骤 2：发送图片消息**

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/messages" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "msg_type": 7,
    "media": {
      "file_info": "YOUR_FILE_INFO"
    },
    "content": "这是一张图片"
  }'
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_type | int | 是 | 文件类型：1=图片，2=语音，3=视频，4=文件 |
| srv_send_msg | bool | 否 | 是否上传后自动发送，默认 false |
| url | string | 是* | 图片 URL 地址 |
| file_data | string | 是* | 图片 Base64 编码（与 url 二选一） |

> 注意：`url` 和 `file_data` 必须提供其中一个。

### 发送 Base64 图片

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 1,
    "srv_send_msg": true,
    "file_data": "iVBORw0KGgoAAAANSUhEUgAA..."
  }'
```

### 发送群聊图片

```bash
curl -X POST "https://api.sgroup.qq.com/v2/groups/GROUP_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 1,
    "srv_send_msg": true,
    "url": "https://example.com/image.png"
  }'
```

---

## 发送其他媒体文件

除了图片，还可以发送语音、视频和文件。

### 文件类型对照表

| file_type | 类型 | 支持格式 |
|-----------|------|----------|
| 1 | 图片 | png, jpg, gif, bmp |
| 2 | 语音 | silk, mp3, wav, amr |
| 3 | 视频 | mp4, mov |
| 4 | 文件 | 任意格式 |

### 发送语音消息

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 2,
    "srv_send_msg": true,
    "file_data": "BASE64_ENCODED_AUDIO"
  }'
```

### 发送视频消息

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 3,
    "srv_send_msg": true,
    "url": "https://example.com/video.mp4"
  }'
```

### 发送文件

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/files" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_type": 4,
    "srv_send_msg": true,
    "url": "https://example.com/document.pdf",
    "file_name": "文档.pdf"
  }'
```

---

## 回复消息

要回复用户的消息，需要提供原消息的 `msg_id`。

### 示例

```bash
curl -X POST "https://api.sgroup.qq.com/v2/users/USER_OPENID/messages" \
  -H "Authorization: QQBot YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "收到你的消息了！",
    "msg_type": 0,
    "msg_id": "ORIGINAL_MESSAGE_ID"
  }'
```

---

## 完整示例脚本

```bash
#!/bin/bash

# 配置
APP_ID="your_app_id"
CLIENT_SECRET="your_client_secret"
USER_OPENID="user_openid"

# 1. 获取 Access Token
echo "获取 Access Token..."
TOKEN_RESP=$(curl -s -X POST "https://bots.qq.com/app/getAppAccessToken" \
  -H "Content-Type: application/json" \
  -d "{\"appId\":\"$APP_ID\",\"clientSecret\":\"$CLIENT_SECRET\"}")

ACCESS_TOKEN=$(echo "$TOKEN_RESP" | jq -r '.access_token')

if [ "$ACCESS_TOKEN" == "null" ] || [ -z "$ACCESS_TOKEN" ]; then
  echo "获取 Token 失败: $TOKEN_RESP"
  exit 1
fi

echo "Access Token: $ACCESS_TOKEN"

# 2. 发送消息
echo "发送消息..."
SEND_RESP=$(curl -s -X POST "https://api.sgroup.qq.com/v2/users/$USER_OPENID/messages" \
  -H "Authorization: QQBot $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"你好！这是一条测试消息。","msg_type":0}')

echo "发送结果: $SEND_RESP"

# 提取消息 ID
MSG_ID=$(echo "$SEND_RESP" | jq -r '.id')
echo "消息 ID: $MSG_ID"
```

---

## 常见错误

### 1. 认证失败 (code: 100016)

```json
{"code":100016,"message":"invalid appid or secret"}
```

**原因**: AppID 或 ClientSecret 错误

**解决**: 检查配置文件中的凭证是否正确

### 2. Token 过期

```json
{"code":xxxxx,"message":"token expired"}
```

**原因**: Access Token 已过期

**解决**: 重新获取 Access Token

### 3. 用户不存在

```json
{"code":xxxxx,"message":"user not found"}
```

**原因**: 用户 OpenID 不正确，或用户从未与 Bot 互动过

**解决**: 确认 OpenID 正确，或让用户先给 Bot 发一条消息

### 4. 消息内容为空

```json
{"code":xxxxx,"message":"content is empty"}
```

**原因**: 消息内容为空字符串

**解决**: 确保 `content` 或 `markdown.content` 不为空

---

## API 参考

- QQ 机器人开发文档: https://bot.q.qq.com/wiki/
- 获取 Access Token: `POST https://bots.qq.com/app/getAppAccessToken`
- 发送私聊消息: `POST https://api.sgroup.qq.com/v2/users/{openid}/messages`
- 发送群聊消息: `POST https://api.sgroup.qq.com/v2/groups/{group_openid}/messages`

---

## 相关文档

- [配置文件说明](./config.md)
- [多账号配置](./multi-account.md)
