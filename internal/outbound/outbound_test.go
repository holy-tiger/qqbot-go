package outbound

import (
	"context"
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

// mockMessageSender implements MessageSender for testing.
type mockMessageSender struct {
	sendC2CMessageFn           func(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error)
	sendGroupMessageFn         func(ctx context.Context, groupOpenID, content, msgID string) (types.MessageResponse, error)
	sendChannelMessageFn       func(ctx context.Context, channelID, content, msgID string) (types.MessageResponse, error)
	sendProactiveC2CMessageFn  func(ctx context.Context, openid, content string) (types.MessageResponse, error)
	sendProactiveGroupMessageFn func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error)
	sendC2CImageMessageFn      func(ctx context.Context, openid, imageURL, msgID, content string) (types.MessageResponse, error)
	sendGroupImageMessageFn    func(ctx context.Context, groupOpenID, imageURL, msgID, content string) (types.MessageResponse, error)
	sendC2CVoiceMessageFn      func(ctx context.Context, openid, voiceBase64, msgID, ttsText string) (types.MessageResponse, error)
	sendGroupVoiceMessageFn    func(ctx context.Context, groupOpenID, voiceBase64, msgID string) (types.MessageResponse, error)
	sendC2CVideoMessageFn      func(ctx context.Context, openid string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error)
	sendGroupVideoMessageFn    func(ctx context.Context, groupOpenID string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error)
	sendC2CFileMessageFn       func(ctx context.Context, openid string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error)
	sendGroupFileMessageFn     func(ctx context.Context, groupOpenID string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error)
}

func (m *mockMessageSender) SendC2CMessage(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error) {
	return m.sendC2CMessageFn(ctx, openid, content, msgID, msgRef)
}

func (m *mockMessageSender) SendGroupMessage(ctx context.Context, groupOpenID, content, msgID string) (types.MessageResponse, error) {
	return m.sendGroupMessageFn(ctx, groupOpenID, content, msgID)
}

func (m *mockMessageSender) SendChannelMessage(ctx context.Context, channelID, content, msgID string) (types.MessageResponse, error) {
	return m.sendChannelMessageFn(ctx, channelID, content, msgID)
}

func (m *mockMessageSender) SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error) {
	return m.sendProactiveC2CMessageFn(ctx, openid, content)
}

func (m *mockMessageSender) SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
	return m.sendProactiveGroupMessageFn(ctx, groupOpenID, content)
}

func (m *mockMessageSender) SendC2CImageMessage(ctx context.Context, openid, imageURL, msgID, content string) (types.MessageResponse, error) {
	return m.sendC2CImageMessageFn(ctx, openid, imageURL, msgID, content)
}

func (m *mockMessageSender) SendGroupImageMessage(ctx context.Context, groupOpenID, imageURL, msgID, content string) (types.MessageResponse, error) {
	return m.sendGroupImageMessageFn(ctx, groupOpenID, imageURL, msgID, content)
}

func (m *mockMessageSender) SendC2CVoiceMessage(ctx context.Context, openid, voiceBase64, msgID, ttsText string) (types.MessageResponse, error) {
	return m.sendC2CVoiceMessageFn(ctx, openid, voiceBase64, msgID, ttsText)
}

func (m *mockMessageSender) SendGroupVoiceMessage(ctx context.Context, groupOpenID, voiceBase64, msgID string) (types.MessageResponse, error) {
	return m.sendGroupVoiceMessageFn(ctx, groupOpenID, voiceBase64, msgID)
}

func (m *mockMessageSender) SendC2CVideoMessage(ctx context.Context, openid string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error) {
	return m.sendC2CVideoMessageFn(ctx, openid, videoURL, videoBase64, msgID, content)
}

func (m *mockMessageSender) SendGroupVideoMessage(ctx context.Context, groupOpenID string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error) {
	return m.sendGroupVideoMessageFn(ctx, groupOpenID, videoURL, videoBase64, msgID, content)
}

func (m *mockMessageSender) SendC2CFileMessage(ctx context.Context, openid string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error) {
	return m.sendC2CFileMessageFn(ctx, openid, fileBase64, fileURL, msgID, fileName)
}

func (m *mockMessageSender) SendGroupFileMessage(ctx context.Context, groupOpenID string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error) {
	return m.sendGroupFileMessageFn(ctx, groupOpenID, fileBase64, fileURL, msgID, fileName)
}

func defaultMsgResp(id string) types.MessageResponse {
	return types.MessageResponse{ID: id, Timestamp: "1234567890"}
}

func TestOutboundHandler_SendText_PlainText_C2C(t *testing.T) {
	mock := &mockMessageSender{
		sendC2CMessageFn: func(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error) {
			if openid != "user123" {
				t.Errorf("expected openid 'user123', got '%s'", openid)
			}
			if content != "hello world" {
				t.Errorf("expected content 'hello world', got '%s'", content)
			}
			return defaultMsgResp("resp-1"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendText(context.Background(), target, "hello world", "msg-001")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendText_PlainText_Group(t *testing.T) {
	mock := &mockMessageSender{
		sendGroupMessageFn: func(ctx context.Context, groupOpenID, content, msgID string) (types.MessageResponse, error) {
			if groupOpenID != "group456" {
				t.Errorf("expected groupOpenID 'group456', got '%s'", groupOpenID)
			}
			if content != "hello group" {
				t.Errorf("expected content 'hello group', got '%s'", content)
			}
			return defaultMsgResp("resp-2"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "group", OpenID: "group456"}
	err := handler.SendText(context.Background(), target, "hello group", "msg-002")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendText_RateLimitFallback(t *testing.T) {
	proactiveCalled := false
	mock := &mockMessageSender{
		sendProactiveC2CMessageFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
			proactiveCalled = true
			if openid != "user123" {
				t.Errorf("expected openid 'user123', got '%s'", openid)
			}
			return defaultMsgResp("resp-proactive"), nil
		},
		sendC2CMessageFn: func(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error) {
			t.Error("should not call passive C2C when rate limited")
			return types.MessageResponse{}, nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	limiter := handler.limiter

	// Exhaust the rate limit
	for i := 0; i < MessageReplyLimit; i++ {
		limiter.Record("msg-limited")
	}

	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendText(context.Background(), target, "should fallback", "msg-limited")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !proactiveCalled {
		t.Error("expected proactive message to be called after rate limit exceeded")
	}
}

func TestOutboundHandler_SendText_WithMediaTags(t *testing.T) {
	textParts := []string{}
	imageCalled := false

	mock := &mockMessageSender{
		sendC2CMessageFn: func(ctx context.Context, openid, content, msgID, msgRef string) (types.MessageResponse, error) {
			textParts = append(textParts, content)
			return defaultMsgResp("resp-text"), nil
		},
		sendC2CImageMessageFn: func(ctx context.Context, openid, imageURL, msgID, content string) (types.MessageResponse, error) {
			imageCalled = true
			if imageURL != "https://example.com/img.png" {
				t.Errorf("expected image URL, got '%s'", imageURL)
			}
			return defaultMsgResp("resp-img"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendText(context.Background(), target, "check this <qqimg>https://example.com/img.png</qqimg> nice!", "msg-003")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !imageCalled {
		t.Error("expected image message to be sent")
	}
	if len(textParts) != 2 {
		t.Errorf("expected 2 text parts, got %d", len(textParts))
	}
	if textParts[0] != "check this" {
		t.Errorf("expected first text part 'check this', got '%s'", textParts[0])
	}
	if textParts[1] != "nice!" {
		t.Errorf("expected second text part 'nice!', got '%s'", textParts[1])
	}
}

func TestOutboundHandler_SendImage_C2C(t *testing.T) {
	mock := &mockMessageSender{
		sendC2CImageMessageFn: func(ctx context.Context, openid, imageURL, msgID, content string) (types.MessageResponse, error) {
			if openid != "user123" {
				t.Errorf("expected openid 'user123', got '%s'", openid)
			}
			if imageURL != "https://example.com/photo.jpg" {
				t.Errorf("expected imageURL 'https://example.com/photo.jpg', got '%s'", imageURL)
			}
			return defaultMsgResp("resp-img"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendImage(context.Background(), target, "https://example.com/photo.jpg", "a photo", "msg-004")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendImage_Group(t *testing.T) {
	mock := &mockMessageSender{
		sendGroupImageMessageFn: func(ctx context.Context, groupOpenID, imageURL, msgID, content string) (types.MessageResponse, error) {
			if groupOpenID != "group789" {
				t.Errorf("expected groupOpenID 'group789', got '%s'", groupOpenID)
			}
			return defaultMsgResp("resp-img"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "group", OpenID: "group789"}
	err := handler.SendImage(context.Background(), target, "https://example.com/photo.jpg", "", "msg-005")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendVoice_C2C(t *testing.T) {
	mock := &mockMessageSender{
		sendC2CVoiceMessageFn: func(ctx context.Context, openid, voiceBase64, msgID, ttsText string) (types.MessageResponse, error) {
			if openid != "user123" {
				t.Errorf("expected openid 'user123', got '%s'", openid)
			}
			if voiceBase64 != "base64data" {
				t.Errorf("expected voiceBase64 'base64data', got '%s'", voiceBase64)
			}
			if ttsText != "hello" {
				t.Errorf("expected ttsText 'hello', got '%s'", ttsText)
			}
			return defaultMsgResp("resp-voice"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendVoice(context.Background(), target, "base64data", "hello", "msg-006")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendVideo_C2C(t *testing.T) {
	mock := &mockMessageSender{
		sendC2CVideoMessageFn: func(ctx context.Context, openid string, videoURL, videoBase64, msgID, content string) (types.MessageResponse, error) {
			if videoURL != "https://example.com/video.mp4" {
				t.Errorf("expected videoURL, got '%s'", videoURL)
			}
			return defaultMsgResp("resp-vid"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendVideo(context.Background(), target, "https://example.com/video.mp4", "", "", "msg-007")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendFile_C2C(t *testing.T) {
	mock := &mockMessageSender{
		sendC2CFileMessageFn: func(ctx context.Context, openid string, fileBase64, fileURL, msgID, fileName string) (types.MessageResponse, error) {
			if fileBase64 != "base64file" {
				t.Errorf("expected fileBase64 'base64file', got '%s'", fileBase64)
			}
			if fileName != "doc.pdf" {
				t.Errorf("expected fileName 'doc.pdf', got '%s'", fileName)
			}
			return defaultMsgResp("resp-file"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendFile(context.Background(), target, "base64file", "", "doc.pdf", "msg-008")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOutboundHandler_SendText_Proactive_NoMsgID(t *testing.T) {
	proactiveCalled := false
	mock := &mockMessageSender{
		sendProactiveC2CMessageFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
			proactiveCalled = true
			return defaultMsgResp("resp-pro"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "c2c", OpenID: "user123"}
	err := handler.SendText(context.Background(), target, "hello proactive", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !proactiveCalled {
		t.Error("expected proactive message for empty msgID")
	}
}

func TestOutboundHandler_SendText_Channel(t *testing.T) {
	mock := &mockMessageSender{
		sendChannelMessageFn: func(ctx context.Context, channelID, content, msgID string) (types.MessageResponse, error) {
			if channelID != "ch001" {
				t.Errorf("expected channelID 'ch001', got '%s'", channelID)
			}
			return defaultMsgResp("resp-ch"), nil
		},
	}

	handler := NewOutboundHandler(mock, "")
	target := Target{Type: "channel", OpenID: "ch001"}
	err := handler.SendText(context.Background(), target, "hello channel", "msg-ch")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
