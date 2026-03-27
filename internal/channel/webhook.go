package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/openclaw/qqbot/internal/types"
)

// webhookEvent mirrors the WebhookEvent struct from internal/httpapi.
type webhookEvent struct {
	AccountID string          `json:"account_id"`
	EventType string          `json:"event_type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// handledEventTypes is the whitelist of event types the channel server processes.
var handledEventTypes = map[string]bool{
	"C2C_MESSAGE_CREATE":      true,
	"GROUP_AT_MESSAGE_CREATE": true,
	"GUILD_MESSAGE_CREATE":    true,
	"DIRECT_MESSAGE_CREATE":   true,
}

func (cs *ChannelServer) startWebhookServer(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", cs.handleWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", cs.config.WebhookPort)
	srv := &http.Server{Addr: addr, Handler: mux}

	log.Printf("[channel] webhook server listening on %s", addr)

	go func() {
		<-ctx.Done()
		log.Printf("[channel] shutting down webhook server")
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[channel] webhook server error: %v", err)
	}
}

func (cs *ChannelServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event webhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("[channel] invalid webhook payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	if !handledEventTypes[event.EventType] {
		return
	}

	switch event.EventType {
	case "C2C_MESSAGE_CREATE":
		cs.handleC2CMessage(event)
	case "GROUP_AT_MESSAGE_CREATE":
		cs.handleGroupMessage(event)
	case "GUILD_MESSAGE_CREATE":
		cs.handleGuildMessage(event)
	case "DIRECT_MESSAGE_CREATE":
		cs.handleDirectMessage(event)
	}
}

func (cs *ChannelServer) handleC2CMessage(event webhookEvent) {
	var msg types.C2CMessageEvent
	if err := json.Unmarshal(event.Data, &msg); err != nil {
		log.Printf("[channel] parse c2c message error: %v", err)
		return
	}

	content := appendAttachmentInfo(msg.Content, msg.Attachments)
	chatID := fmt.Sprintf("c2c:%s", msg.Author.UserOpenID)
	cs.pushNotification("qq", msg.Author.UserOpenID, chatID, content)
}

func (cs *ChannelServer) handleGroupMessage(event webhookEvent) {
	var msg types.GroupMessageEvent
	if err := json.Unmarshal(event.Data, &msg); err != nil {
		log.Printf("[channel] parse group message error: %v", err)
		return
	}

	content := stripAtMention(msg.Content)
	content = appendAttachmentInfo(content, msg.Attachments)

	chatID := fmt.Sprintf("group:%s", msg.GroupOpenID)
	cs.pushNotification("qq", msg.Author.MemberOpenID, chatID, content)
}

func (cs *ChannelServer) handleGuildMessage(event webhookEvent) {
	var msg types.GuildMessageEvent
	if err := json.Unmarshal(event.Data, &msg); err != nil {
		log.Printf("[channel] parse guild message error: %v", err)
		return
	}

	content := stripAtMention(msg.Content)
	content = appendAttachmentInfo(content, msg.Attachments)

	chatID := fmt.Sprintf("channel:%s", msg.ChannelID)
	cs.pushNotification("qq", msg.Author.ID, chatID, content)
}

func (cs *ChannelServer) handleDirectMessage(event webhookEvent) {
	var msg types.GuildMessageEvent
	if err := json.Unmarshal(event.Data, &msg); err != nil {
		log.Printf("[channel] parse direct message error: %v", err)
		return
	}

	content := stripAtMention(msg.Content)
	content = appendAttachmentInfo(content, msg.Attachments)

	chatID := fmt.Sprintf("dm:%s", msg.ChannelID)
	cs.pushNotification("qq", msg.Author.ID, chatID, content)
}

// stripAtMention removes @bot mentions from the beginning of a message.
func stripAtMention(content string) string {
	// QQ mention format: "<@!user_id> rest"
	if strings.HasPrefix(content, "<@!") {
		if idx := strings.Index(content, ">"); idx != -1 {
			return strings.TrimSpace(content[idx+1:])
		}
	}
	// QQ @nickname format: "@Bot\x00rest" or "@Bot rest"
	// Only strip if the delimiter (null or space) comes right after the @mention,
	// i.e., there must be no additional text before the delimiter.
	if strings.HasPrefix(content, "@") {
		if idx := strings.Index(content, "\x00"); idx > 0 {
			return strings.TrimSpace(content[idx+1:])
		}
		if idx := strings.Index(content, " "); idx > 0 {
			return strings.TrimSpace(content[idx+1:])
		}
	}
	return content
}
