package qqbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openclaw/qqbot/internal/config"
	"github.com/openclaw/qqbot/internal/httpapi"
	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
)

// accountInfoAdapter wraps *Account to satisfy httpapi's account info interface.
type accountInfoAdapter struct {
	a *Account
}

func (a *accountInfoAdapter) GetID() string       { return a.a.ID }
func (a *accountInfoAdapter) IsConnected() bool   { return a.a.Gateway.IsConnected() }

// statusAdapter wraps AccountStatus to satisfy httpapi's status interface.
type statusAdapter struct {
	s AccountStatus
}

func (a statusAdapter) GetID() string     { return a.s.ID }
func (a statusAdapter) IsConnected() bool { return a.s.Connected }

// botAPIAdapter adapts *BotManager to implement httpapi.BotAPI.
type botAPIAdapter struct {
	m *BotManager
}

func (a *botAPIAdapter) GetAccount(id string) interface{ GetID() string; IsConnected() bool } {
	acct := a.m.GetAccount(id)
	if acct == nil {
		return nil
	}
	return &accountInfoAdapter{a: acct}
}

func (a *botAPIAdapter) GetAllStatuses() []interface{ GetID() string; IsConnected() bool } {
	statuses := a.m.GetAllStatuses()
	result := make([]interface{ GetID() string; IsConnected() bool }, len(statuses))
	for i, s := range statuses {
		result[i] = statusAdapter{s: s}
	}
	return result
}

func (a *botAPIAdapter) SendC2C(ctx context.Context, accountID, openid, content, msgID string) error {
	return a.m.SendC2C(ctx, accountID, openid, content, msgID)
}

func (a *botAPIAdapter) SendGroup(ctx context.Context, accountID, groupOpenID, content, msgID string) error {
	return a.m.SendGroup(ctx, accountID, groupOpenID, content, msgID)
}

func (a *botAPIAdapter) SendChannel(ctx context.Context, accountID, channelID, content, msgID string) error {
	return a.m.SendChannel(ctx, accountID, channelID, content, msgID)
}

func (a *botAPIAdapter) SendImage(ctx context.Context, accountID, targetType, targetID, imageURL, content, msgID string) error {
	return a.m.SendImage(ctx, accountID, targetType, targetID, imageURL, content, msgID)
}

func (a *botAPIAdapter) SendVoice(ctx context.Context, accountID, targetType, targetID, voiceBase64, ttsText, msgID string) error {
	return a.m.SendVoice(ctx, accountID, targetType, targetID, voiceBase64, ttsText, msgID)
}

func (a *botAPIAdapter) SendVideo(ctx context.Context, accountID, targetType, targetID, videoURL, videoBase64, content, msgID string) error {
	return a.m.SendVideo(ctx, accountID, targetType, targetID, videoURL, videoBase64, content, msgID)
}

func (a *botAPIAdapter) SendFile(ctx context.Context, accountID, targetType, targetID, fileBase64, fileURL, fileName, msgID string) error {
	return a.m.SendFile(ctx, accountID, targetType, targetID, fileBase64, fileURL, fileName, msgID)
}

func (a *botAPIAdapter) SendProactiveC2C(ctx context.Context, accountID, openid, content string) error {
	return a.m.SendProactiveC2C(ctx, accountID, openid, content)
}

func (a *botAPIAdapter) SendProactiveGroup(ctx context.Context, accountID, groupOpenID, content string) error {
	return a.m.SendProactiveGroup(ctx, accountID, groupOpenID, content)
}

func (a *botAPIAdapter) Broadcast(ctx context.Context, accountID, content string) (int, []error) {
	return a.m.Broadcast(ctx, accountID, content)
}

func (a *botAPIAdapter) BroadcastToGroups(ctx context.Context, accountID, content string) (int, []error) {
	return a.m.BroadcastToGroups(ctx, accountID, content)
}

func (a *botAPIAdapter) ListUsers(accountID string, opts store.ListOptions) []store.KnownUser {
	return a.m.ListUsers(accountID, opts)
}

func (a *botAPIAdapter) GetUserStats(accountID string) store.UserStats {
	return a.m.GetUserStats(accountID)
}

func (a *botAPIAdapter) ClearUsers(accountID string) int {
	return a.m.ClearUsers(accountID)
}

func (a *botAPIAdapter) AddReminder(job proactive.ReminderJob) (string, error) {
	return a.m.AddReminder(job)
}

func (a *botAPIAdapter) CancelReminder(accountID, jobID string) bool {
	return a.m.CancelReminder(accountID, jobID)
}

func (a *botAPIAdapter) GetReminders(accountID string) []proactive.ReminderJob {
	return a.m.GetReminders(accountID)
}

// Run starts the BotManager with graceful shutdown handling.
// It blocks until SIGINT/SIGTERM is received or the context is cancelled.
func Run(ctx context.Context, cfgPath string, healthAddr string, apiAddr string, version string) error {
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

	// Set up webhook dispatcher
	webhook := httpapi.NewWebhookDispatcher()
	mgr.SetWebhookDispatcher(webhook)

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("start manager: %w", err)
	}

	// API server
	var apiServer *httpapi.APIServer
	if apiAddr != "" {
		apiServer = httpapi.NewAPIServer(&botAPIAdapter{m: mgr}, webhook)
		if err := apiServer.Start(apiAddr); err != nil {
			log.Printf("[qqbot] WARNING: API server failed to start: %v", err)
		} else {
			defer apiServer.Stop()
			log.Printf("[qqbot] API server listening on %s", apiAddr)
		}
	}

	// Health server
	var healthServer *HealthServer
	if healthAddr != "" {
		healthServer = NewHealthServer(mgr, version)
		if err := healthServer.Start(healthAddr); err != nil {
			log.Printf("[qqbot] WARNING: health server failed to start: %v", err)
		} else {
			defer healthServer.Stop()
			log.Printf("[qqbot] health server listening on %s", healthAddr)
		}
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[qqbot] received signal %v, shutting down...", sig)
	case <-ctx.Done():
		log.Printf("[qqbot] context cancelled, shutting down...")
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		if apiServer != nil {
			apiServer.Stop()
		}
		if healthServer != nil {
			healthServer.Stop()
		}
		mgr.Stop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[qqbot] shutdown complete")
	case <-shutdownCtx.Done():
		log.Println("[qqbot] shutdown timed out, forcing exit")
	}

	return nil
}

// loadAndValidateConfig loads the config file and checks it's readable.
func loadAndValidateConfig(path string) (*config.QQBotConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
