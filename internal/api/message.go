package api

import (
	"context"
	"fmt"

	"github.com/openclaw/qqbot/internal/types"
)

// SendC2CMessage sends a C2C (private) message.
func (c *APIClient) SendC2CMessage(ctx context.Context, openid, content string, msgID string, msgRef string) (types.MessageResponse, error) {
	var msgSeq int
	if msgID != "" {
		msgSeq = getNextMsgSeq(msgID)
	} else {
		msgSeq = 1
	}

	body := c.buildMessageBody(content, msgID, msgSeq, msgRef)
	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/users/%s/messages", openid), body, types.OutboundMeta{Text: content})
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// SendC2CInputNotify sends a C2C input notification (bot is typing).
func (c *APIClient) SendC2CInputNotify(ctx context.Context, openid string, msgID string, inputSeconds int) (string, error) {
	var msgSeq int
	if msgID != "" {
		msgSeq = getNextMsgSeq(msgID)
	} else {
		msgSeq = 1
	}

	body := map[string]interface{}{
		"msg_type": 6,
		"input_notify": map[string]interface{}{
			"input_type":  1,
			"input_second": inputSeconds,
		},
		"msg_seq": msgSeq,
	}
	if msgID != "" {
		body["msg_id"] = msgID
	}

	respBody, err := c.doRequest(ctx, "POST", fmt.Sprintf("/v2/users/%s/messages", openid), body, DefaultAPITimeout)
	if err != nil {
		return "", err
	}

	var resp struct {
		ExtInfo *struct {
			RefIdx string `json:"ref_idx"`
		} `json:"ext_info"`
	}
	if err := jsonUnmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if resp.ExtInfo != nil && resp.ExtInfo.RefIdx != "" {
		return resp.ExtInfo.RefIdx, nil
	}
	return "", nil
}

// SendChannelMessage sends a message to a guild channel.
func (c *APIClient) SendChannelMessage(ctx context.Context, channelID, content string, msgID string) (types.MessageResponse, error) {
	body := map[string]interface{}{
		"content": content,
	}
	if msgID != "" {
		body["msg_id"] = msgID
	}

	respBody, err := c.doRequest(ctx, "POST", fmt.Sprintf("/channels/%s/messages", channelID), body, DefaultAPITimeout)
	if err != nil {
		return types.MessageResponse{}, err
	}

	var resp types.MessageResponse
	if err := jsonUnmarshal(respBody, &resp); err != nil {
		return types.MessageResponse{}, fmt.Errorf("parse response: %w", err)
	}
	return resp, nil
}

// SendGroupMessage sends a message to a group.
func (c *APIClient) SendGroupMessage(ctx context.Context, groupOpenID, content string, msgID string) (types.MessageResponse, error) {
	var msgSeq int
	if msgID != "" {
		msgSeq = getNextMsgSeq(msgID)
	} else {
		msgSeq = 1
	}

	body := c.buildMessageBody(content, msgID, msgSeq, "")
	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/groups/%s/messages", groupOpenID), body, types.OutboundMeta{Text: content})
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// SendProactiveC2CMessage sends a proactive C2C message (no reply context).
func (c *APIClient) SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error) {
	body := c.buildProactiveMessageBody(content)
	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/users/%s/messages", openid), body, types.OutboundMeta{Text: content})
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// SendProactiveGroupMessage sends a proactive group message (no reply context).
func (c *APIClient) SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
	body := c.buildProactiveMessageBody(content)
	resp, err := c.sendAndNotify(ctx, "POST", fmt.Sprintf("/v2/groups/%s/messages", groupOpenID), body, types.OutboundMeta{Text: content})
	if err != nil {
		return types.MessageResponse{}, err
	}
	return *resp, nil
}

// buildMessageBody builds the request body for a message based on markdown support.
func (c *APIClient) buildMessageBody(content string, msgID string, msgSeq int, messageReference string) map[string]interface{} {
	var body map[string]interface{}

	if c.markdownSupport {
		body = map[string]interface{}{
			"markdown": map[string]interface{}{"content": content},
			"msg_type": 2,
			"msg_seq":  msgSeq,
		}
	} else {
		body = map[string]interface{}{
			"content":  content,
			"msg_type": 0,
			"msg_seq":  msgSeq,
		}
	}

	if msgID != "" {
		body["msg_id"] = msgID
	}

	if messageReference != "" && !c.markdownSupport {
		body["message_reference"] = map[string]interface{}{
			"message_id": messageReference,
		}
	}

	return body
}

// buildProactiveMessageBody builds the request body for a proactive message.
func (c *APIClient) buildProactiveMessageBody(content string) map[string]interface{} {
	if content == "" {
		return map[string]interface{}{
			"content":  "",
			"msg_type": 0,
		}
	}
	if c.markdownSupport {
		return map[string]interface{}{
			"markdown": map[string]interface{}{"content": content},
			"msg_type": 2,
		}
	}
	return map[string]interface{}{
		"content":  content,
		"msg_type": 0,
	}
}
