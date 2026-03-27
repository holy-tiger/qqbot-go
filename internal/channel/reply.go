package channel

import (
	"context"
	"log"

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

	chatType, targetID, err := parseChatID(chatID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	args := request.GetArguments()
	mediaType, _ := args["media_type"].(string)
	mediaURL, _ := args["media_url"].(string)

	accountID := cs.getAccountID(chatType, targetID)
	err = cs.sender.Send(ctx, accountID, chatType, targetID, text, mediaType, mediaURL)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	log.Printf("[channel] replied to %s (%s)", chatID, mediaType)
	return mcp.NewToolResultText("sent"), nil
}

// getAccountID returns the account ID to use for sending.
// For embedded mode, it uses the event's account_id if available.
// For standalone mode, it falls back to the config default.
func (cs *ChannelServer) getAccountID(chatType, targetID string) string {
	return cs.config.Account
}
