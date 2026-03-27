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

// mediaEndpoints maps media_type to the URL suffix after the chat ID.
var mediaEndpoints = map[string]string{
	"image": "images",
	"file":  "files",
	"voice": "voice",
	"video": "videos",
}

// buildMediaPath builds the API path for a media reply.
// Returns an error if media_type is not supported for the given chat type.
func buildMediaPath(account, chatType, targetID, mediaType string) (string, error) {
	suffix, ok := mediaEndpoints[mediaType]
	if !ok {
		return "", fmt.Errorf("unknown media_type: %s", mediaType)
	}
	// Media endpoints only exist for c2c and group.
	if chatType != "c2c" && chatType != "group" {
		return "", fmt.Errorf("media not supported for %s chat type (only c2c and group)", chatType)
	}
	// Map chat_type to API path prefix (group -> groups).
	pathType := chatType
	if chatType == "group" {
		pathType = "groups"
	}
	return fmt.Sprintf("/api/v1/accounts/%s/%s/%s/%s", account, pathType, targetID, suffix), nil
}

// buildTextPath builds the API path for a text reply.
func buildTextPath(account, chatType, targetID string) (string, error) {
	switch chatType {
	case "c2c":
		return fmt.Sprintf("/api/v1/accounts/%s/c2c/%s/messages", account, targetID), nil
	case "group":
		return fmt.Sprintf("/api/v1/accounts/%s/groups/%s/messages", account, targetID), nil
	case "channel", "dm":
		return fmt.Sprintf("/api/v1/accounts/%s/channels/%s/messages", account, targetID), nil
	default:
		return "", fmt.Errorf("unknown chat type: %s", chatType)
	}
}

// buildRequestBody builds the JSON request body based on media_type.
func buildRequestBody(text, mediaType, mediaURL string) ([]byte, error) {
	if mediaType == "" {
		return json.Marshal(map[string]string{"content": text})
	}

	switch mediaType {
	case "image":
		body := map[string]string{"image_url": mediaURL, "content": text}
		return json.Marshal(body)
	case "file":
		body := map[string]string{"file_url": mediaURL, "file_name": "file", "content": text}
		return json.Marshal(body)
	case "video":
		body := map[string]string{"video_url": mediaURL, "content": text}
		return json.Marshal(body)
	case "voice":
		body := map[string]string{"voice_base64": mediaURL, "tts_text": text}
		return json.Marshal(body)
	default:
		return nil, fmt.Errorf("unknown media_type: %s", mediaType)
	}
}

// handleReply processes a reply tool call from CodeBuddy Code.
func (cs *ChannelServer) handleReply(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	chatID, err := request.RequireString("chat_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := request.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	parts := strings.SplitN(chatID, ":", 2)
	if len(parts) != 2 {
		return mcp.NewToolResultError(fmt.Sprintf("invalid chat_id format: %s", chatID)), nil
	}

	msgType, targetID := parts[0], parts[1]
	args := request.GetArguments()
	mediaType, _ := args["media_type"].(string)
	mediaURL, _ := args["media_url"].(string)

	var apiPath string
	if mediaType != "" {
		apiPath, err = buildMediaPath(cs.config.Account, msgType, targetID, mediaType)
	} else {
		apiPath, err = buildTextPath(cs.config.Account, msgType, targetID)
	}
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if mediaType != "" && mediaType != "voice" && mediaURL == "" {
		return mcp.NewToolResultError(fmt.Sprintf("media_url is required for media_type %s", mediaType)), nil
	}

	body, err := buildRequestBody(text, mediaType, mediaURL)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	log.Printf("[channel] replied to %s (%s, %d bytes)", chatID, mediaType, len(body))
	return mcp.NewToolResultText("sent"), nil
}
