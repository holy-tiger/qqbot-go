package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Sender sends messages to QQ conversations.
// Both httpSender (standalone mode) and localSender (embedded mode) satisfy this interface.
type Sender interface {
	Send(ctx context.Context, accountID, chatType, targetID, text, mediaType, mediaURL string) error
}

// httpSender sends messages via the qqbot HTTP API.
type httpSender struct {
	baseURL    string
	httpClient *http.Client
}

func newHTTPSender(baseURL string) *httpSender {
	return &httpSender{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send implements Sender by calling the qqbot HTTP API.
func (s *httpSender) Send(ctx context.Context, accountID, chatType, targetID, text, mediaType, mediaURL string) error {
	var apiPath string
	var err error

	if mediaType != "" {
		if mediaType != "voice" && mediaURL == "" {
			return fmt.Errorf("media_url is required for media_type %s", mediaType)
		}
		apiPath, err = buildMediaPath(accountID, chatType, targetID, mediaType)
	} else {
		apiPath, err = buildTextPath(accountID, chatType, targetID)
	}
	if err != nil {
		return err
	}

	body, err := buildRequestBody(text, mediaType, mediaURL)
	if err != nil {
		return err
	}

	url := s.baseURL + apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[channel] reply error: %v", err)
		return fmt.Errorf("发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[channel] reply HTTP %d for %s", resp.StatusCode, apiPath)
		return fmt.Errorf("发送失败: HTTP %d", resp.StatusCode)
	}

	return nil
}

// parseChatID splits "type:id" into (type, id).
func parseChatID(chatID string) (chatType, targetID string, err error) {
	parts := strings.SplitN(chatID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid chat_id format: %s", chatID)
	}
	return parts[0], parts[1], nil
}

// mediaEndpoints maps media_type to the URL suffix after the chat ID.
var mediaEndpoints = map[string]string{
	"image": "images",
	"file":  "files",
	"voice": "voice",
	"video": "videos",
}

// buildMediaPath builds the API path for a media reply.
func buildMediaPath(account, chatType, targetID, mediaType string) (string, error) {
	suffix, ok := mediaEndpoints[mediaType]
	if !ok {
		return "", fmt.Errorf("unknown media_type: %s", mediaType)
	}
	if chatType != "c2c" && chatType != "group" {
		return "", fmt.Errorf("media not supported for %s chat type (only c2c and group)", chatType)
	}
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
		return json.Marshal(map[string]string{"image_url": mediaURL, "content": text})
	case "file":
		return json.Marshal(map[string]string{"file_url": mediaURL, "file_name": "file", "content": text})
	case "video":
		return json.Marshal(map[string]string{"video_url": mediaURL, "content": text})
	case "voice":
		return json.Marshal(map[string]string{"voice_base64": mediaURL, "tts_text": text})
	default:
		return nil, fmt.Errorf("unknown media_type: %s", mediaType)
	}
}
