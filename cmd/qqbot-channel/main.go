package main

import (
	"flag"
	"log"

	"github.com/openclaw/qqbot/internal/channel"
)

func main() {
	webhookPort := flag.Int("webhook-port", 8788, "webhook HTTP listen port")
	qqbotAPI := flag.String("qqbot-api", "http://127.0.0.1:9090", "qqbot HTTP API address")
	account := flag.String("account", "default", "default qqbot account ID")
	flag.Parse()

	cfg := channel.Config{
		WebhookPort: *webhookPort,
		QQBotAPI:    *qqbotAPI,
		Account:     *account,
	}

	if err := channel.Run(cfg); err != nil {
		log.Fatalf("channel: %v", err)
	}
}
