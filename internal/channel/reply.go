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
	var apiPath string
	switch msgType {
	case "c2c":
		apiPath = fmt.Sprintf("/api/v1/accounts/%s/c2c/%s/messages", cs.config.Account, targetID)
	case "group":
		apiPath = fmt.Sprintf("/api/v1/accounts/%s/groups/%s/messages", cs.config.Account, targetID)
	case "channel", "dm":
		apiPath = fmt.Sprintf("/api/v1/accounts/%s/channels/%s/messages", cs.config.Account, targetID)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("unknown chat type: %s", msgType)), nil
	}

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
