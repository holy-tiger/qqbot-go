package qqbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/openclaw/qqbot/internal/channel"
	"github.com/openclaw/qqbot/internal/config"
)

// channelSender adapts *BotManager to implement channel.Sender.
type channelSender struct {
	m *BotManager
}

// Send implements channel.Sender by routing to the appropriate BotManager method.
func (s *channelSender) Send(ctx context.Context, accountID, chatType, targetID, text, mediaType, mediaURL string) error {
	switch mediaType {
	case "":
		// Text message
		switch chatType {
		case "c2c":
			return s.m.SendC2C(ctx, accountID, targetID, text, "")
		case "group":
			return s.m.SendGroup(ctx, accountID, targetID, text, "")
		case "channel", "dm":
			return s.m.SendChannel(ctx, accountID, targetID, text, "")
		default:
			return fmt.Errorf("unknown chat type: %s", chatType)
		}
	case "image":
		return s.m.SendImage(ctx, accountID, chatType, targetID, mediaURL, text, "")
	case "file":
		return s.m.SendFile(ctx, accountID, chatType, targetID, "", mediaURL, "file", "")
	case "voice":
		return s.m.SendVoice(ctx, accountID, chatType, targetID, mediaURL, text, "")
	case "video":
		return s.m.SendVideo(ctx, accountID, chatType, targetID, mediaURL, "", text, "")
	default:
		return fmt.Errorf("unknown media_type: %s", mediaType)
	}
}

// RunChannel starts the embedded MCP channel server.
// It blocks until the MCP stdio server exits (e.g., parent process closes stdin).
func RunChannel(cfgPath string) error {
	cfg, err := loadAndValidateConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	validation := ValidateConfig(cfg)
	if !validation.Valid {
		for _, e := range validation.Errors {
			log.Printf("[config] ERROR: %s", e)
		}
		return fmt.Errorf("configuration validation failed with %d errors", len(validation.Errors))
	}
	for _, w := range validation.Warnings {
		log.Printf("[config] WARNING: %s", w)
	}

	accountIDs := config.ListAccountIDs(cfg)
	if len(accountIDs) == 0 {
		return fmt.Errorf("no accounts configured")
	}

	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	mgr, err := NewBotManager(dataDir)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	defer mgr.Stop()

	for _, id := range accountIDs {
		acct := config.ResolveAccount(cfg, id)
		if !acct.Enabled {
			log.Printf("[qqbot] account %q is disabled, skipping", id)
			continue
		}
		if err := mgr.AddAccount(acct); err != nil {
			return fmt.Errorf("add account %q: %w", id, err)
		}
		log.Printf("[qqbot] added account %q (appId=%s)", id, acct.AppID)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("start manager: %w", err)
	}

	// Create channel server with direct sender
	sender := &channelSender{m: mgr}
	cs := channel.NewEmbedded(cfgPath, sender)

	// Bridge BotManager events → MCP notifications
	mgr.SetEventHandler(func(accountID string, eventType string, payload []byte) {
		if eventType != "C2C_MESSAGE_CREATE" && eventType != "GROUP_AT_MESSAGE_CREATE" &&
			eventType != "GUILD_MESSAGE_CREATE" && eventType != "DIRECT_MESSAGE_CREATE" {
			return
		}
		content, chatID, _, senderID := channel.ExtractMessage(eventType, payload)
		if content == "" {
			return
		}
		cs.ForwardMessage(senderID, chatID, content)
	})

	log.Printf("[qqbot] embedded channel mode started")
	err = cs.ServeStdio()
	return err
}
