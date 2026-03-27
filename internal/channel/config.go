package channel

// Config holds the channel server configuration.
type Config struct {
	WebhookPort int    // HTTP port for receiving qqbot webhooks
	QQBotAPI    string // qqbot HTTP API address
	Account     string // default account ID for reply routing
}
