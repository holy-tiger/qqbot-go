package channel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openclaw/qqbot/internal/types"
)

// appendAttachmentInfo appends human-readable attachment descriptions to the message content.
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

// ExtractMessage extracts content, chatID, source, and sender from a raw event payload.
// Returns empty content if the event type is not a message event or parsing fails.
func ExtractMessage(eventType string, payload []byte) (content, chatID, source, sender string) {
	switch eventType {
	case "C2C_MESSAGE_CREATE":
		var event types.C2CMessageEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return "", "", "", ""
		}
		sender = event.Author.UserOpenID
		chatID = "c2c:" + sender
		content = appendAttachmentInfo(event.Content, event.Attachments)

	case "GROUP_AT_MESSAGE_CREATE":
		var event types.GroupMessageEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return "", "", "", ""
		}
		sender = event.Author.MemberOpenID
		chatID = "group:" + event.GroupOpenID
		content = appendAttachmentInfo(event.Content, event.Attachments)

	case "GUILD_MESSAGE_CREATE":
		var event types.GuildMessageEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return "", "", "", ""
		}
		sender = event.Author.ID
		chatID = "channel:" + event.ChannelID
		content = appendAttachmentInfo(event.Content, event.Attachments)

	case "DIRECT_MESSAGE_CREATE":
		var event struct {
			Content     string                   `json:"content"`
			Author      types.C2CAuthor          `json:"author"`
			ChannelID   string                   `json:"channel_id"`
			Attachments []types.MessageAttachment `json:"attachments"`
		}
		if err := json.Unmarshal(payload, &event); err != nil {
			return "", "", "", ""
		}
		sender = event.Author.UserOpenID
		if sender == "" {
			sender = event.Author.ID
		}
		chatID = "dm:" + event.ChannelID
		content = appendAttachmentInfo(event.Content, event.Attachments)

	default:
		return "", "", "", ""
	}

	source = "qq"
	return content, chatID, source, sender
}
