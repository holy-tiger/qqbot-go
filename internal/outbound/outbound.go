package outbound

import (
	"context"
	"strings"

	"github.com/openclaw/qqbot/internal/types"
)

// MessageSender defines the API client methods used by OutboundHandler.
type MessageSender interface {
	SendC2CMessage(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error)
	SendGroupMessage(ctx context.Context, groupOpenID, content, msgID string) (types.MessageResponse, error)
	SendChannelMessage(ctx context.Context, channelID, content, msgID string) (types.MessageResponse, error)
	SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error)
	SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error)
	SendC2CImageMessage(ctx context.Context, openid, imageURL, msgID, content string) (types.MessageResponse, error)
	SendGroupImageMessage(ctx context.Context, groupOpenID, imageURL, msgID, content string) (types.MessageResponse, error)
	SendC2CVoiceMessage(ctx context.Context, openid, voiceBase64, msgID, ttsText string) (types.MessageResponse, error)
	SendGroupVoiceMessage(ctx context.Context, groupOpenID, voiceBase64, msgID string) (types.MessageResponse, error)
	SendC2CVideoMessage(ctx context.Context, openid string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error)
	SendGroupVideoMessage(ctx context.Context, groupOpenID string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error)
	SendC2CFileMessage(ctx context.Context, openid string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error)
	SendGroupFileMessage(ctx context.Context, groupOpenID string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error)
}

// Target represents a message destination.
type Target struct {
	Type   string // "c2c", "group", "channel"
	OpenID string // user openid, group openid, or channel id
}

// OutboundHandler handles sending outbound messages with rate limiting and media tag parsing.
type OutboundHandler struct {
	sender      MessageSender
	limiter     *ReplyLimiter
	imageServer string // base URL for local images
}

// NewOutboundHandler creates a new OutboundHandler.
func NewOutboundHandler(sender MessageSender, imageServerURL string) *OutboundHandler {
	return &OutboundHandler{
		sender:      sender,
		limiter:     NewReplyLimiter(),
		imageServer: imageServerURL,
	}
}

// SendText sends a text message, parsing any embedded media tags.
func (h *OutboundHandler) SendText(ctx context.Context, target Target, text, msgID string) error {
	effectiveMsgID := msgID

	// Rate limit check for passive replies
	if effectiveMsgID != "" {
		result := h.limiter.Check(effectiveMsgID)
		if !result.Allowed && result.ShouldFallbackToProactive {
			effectiveMsgID = ""
		}
	}

	// Parse media tags from text
	tags := ParseMediaTags(text)
	if len(tags) == 0 {
		// Plain text, no media tags
		return h.sendPlainText(ctx, target, text, effectiveMsgID)
	}

	// Send text and media parts in order
	return h.sendMixedContent(ctx, target, text, tags, effectiveMsgID)
}

// SendMedia sends a media-rich text message.
func (h *OutboundHandler) SendMedia(ctx context.Context, target Target, text string, msgID string) error {
	tags := ParseMediaTags(text)
	if len(tags) == 0 {
		return h.sendPlainText(ctx, target, text, msgID)
	}
	return h.sendMixedContent(ctx, target, text, tags, msgID)
}

// SendImage sends an image message.
func (h *OutboundHandler) SendImage(ctx context.Context, target Target, imageURL, content, msgID string) error {
	imageURL = h.resolveImageURL(imageURL)
	switch target.Type {
	case "c2c":
		_, err := h.sender.SendC2CImageMessage(ctx, target.OpenID, imageURL, msgID, content)
		return err
	case "group":
		_, err := h.sender.SendGroupImageMessage(ctx, target.OpenID, imageURL, msgID, content)
		return err
	default:
		return nil
	}
}

// SendVoice sends a voice message.
func (h *OutboundHandler) SendVoice(ctx context.Context, target Target, voiceBase64, ttsText, msgID string) error {
	switch target.Type {
	case "c2c":
		_, err := h.sender.SendC2CVoiceMessage(ctx, target.OpenID, voiceBase64, msgID, ttsText)
		return err
	case "group":
		_, err := h.sender.SendGroupVoiceMessage(ctx, target.OpenID, voiceBase64, msgID)
		return err
	default:
		return nil
	}
}

// SendVideo sends a video message.
func (h *OutboundHandler) SendVideo(ctx context.Context, target Target, videoURL, videoBase64, content, msgID string) error {
	switch target.Type {
	case "c2c":
		_, err := h.sender.SendC2CVideoMessage(ctx, target.OpenID, videoURL, videoBase64, msgID, content)
		return err
	case "group":
		_, err := h.sender.SendGroupVideoMessage(ctx, target.OpenID, videoURL, videoBase64, msgID, content)
		return err
	default:
		return nil
	}
}

// SendFile sends a file message.
func (h *OutboundHandler) SendFile(ctx context.Context, target Target, fileBase64, fileURL, fileName, msgID string) error {
	switch target.Type {
	case "c2c":
		_, err := h.sender.SendC2CFileMessage(ctx, target.OpenID, fileBase64, fileURL, msgID, fileName)
		return err
	case "group":
		_, err := h.sender.SendGroupFileMessage(ctx, target.OpenID, fileBase64, fileURL, msgID, fileName)
		return err
	default:
		return nil
	}
}

// resolveImageURL resolves a local path to a full URL using the image server.
func (h *OutboundHandler) resolveImageURL(url string) string {
	if h.imageServer != "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "data:") {
		return h.imageServer + url
	}
	return url
}

// sendPlainText sends plain text without media tags.
func (h *OutboundHandler) sendPlainText(ctx context.Context, target Target, text, msgID string) error {
	switch target.Type {
	case "c2c":
		if msgID != "" {
			_, err := h.sender.SendC2CMessage(ctx, target.OpenID, text, msgID, "")
			if err == nil {
				h.limiter.Record(msgID)
			}
			return err
		}
		_, err := h.sender.SendProactiveC2CMessage(ctx, target.OpenID, text)
		return err
	case "group":
		if msgID != "" {
			_, err := h.sender.SendGroupMessage(ctx, target.OpenID, text, msgID)
			if err == nil {
				h.limiter.Record(msgID)
			}
			return err
		}
		_, err := h.sender.SendProactiveGroupMessage(ctx, target.OpenID, text)
		return err
	case "channel":
		_, err := h.sender.SendChannelMessage(ctx, target.OpenID, text, msgID)
		return err
	default:
		return nil
	}
}

// sendMixedContent sends text with embedded media tags, processing them in order.
func (h *OutboundHandler) sendMixedContent(ctx context.Context, target Target, text string, tags []ParsedMediaTag, msgID string) error {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lastIndex := 0

	for _, tag := range tags {
		// Find the tag in the normalized text
		idx := strings.Index(normalized[lastIndex:], tag.RawMatch)
		if idx < 0 {
			continue
		}
		absIdx := lastIndex + idx

		// Send text before this tag
		textBefore := strings.TrimSpace(normalized[lastIndex:absIdx])
		if textBefore != "" {
			if err := h.sendPlainText(ctx, target, textBefore, msgID); err != nil {
				return err
			}
		}

		// Send the media
		if err := h.sendMediaTag(ctx, target, tag, msgID); err != nil {
			return err
		}

		lastIndex = absIdx + len(tag.RawMatch)
	}

	// Send remaining text after last tag
	textAfter := strings.TrimSpace(normalized[lastIndex:])
	if textAfter != "" {
		if err := h.sendPlainText(ctx, target, textAfter, msgID); err != nil {
			return err
		}
	}

	return nil
}

// sendMediaTag sends a single media tag.
func (h *OutboundHandler) sendMediaTag(ctx context.Context, target Target, tag ParsedMediaTag, msgID string) error {
	path := h.resolveImageURL(tag.Path)

	switch tag.Type {
	case "image":
		return h.SendImage(ctx, target, path, tag.Caption, msgID)
	case "voice":
		return h.SendVoice(ctx, target, path, tag.Caption, msgID)
	case "video":
		return h.SendVideo(ctx, target, path, "", tag.Caption, msgID)
	case "file":
		return h.SendFile(ctx, target, "", path, tag.Caption, msgID)
	default:
		return nil
	}
}
