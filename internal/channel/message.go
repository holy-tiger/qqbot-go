package channel

import (
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
