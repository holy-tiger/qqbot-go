package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/openclaw/qqbot/internal/api"
	"github.com/openclaw/qqbot/internal/audio"
	"github.com/openclaw/qqbot/internal/gateway"
	"github.com/openclaw/qqbot/internal/outbound"
	"github.com/openclaw/qqbot/internal/proactive"
	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

// WebhookDispatcher defines the interface for dispatching webhook events.
type WebhookDispatcher interface {
	Dispatch(accountID, eventType string, payload []byte)
	SetURL(accountID, url string)
}

// EventHandler is a callback for processing QQ Bot events.
type EventHandler func(accountID string, eventType string, payload []byte)

// Account represents a single QQ Bot account with all its isolated dependencies.
type Account struct {
	ID         string
	Config     types.ResolvedQQBotAccount
	Client     *api.APIClient
	Gateway    *gateway.Gateway
	Outbound   *outbound.OutboundHandler
	Proactive  *proactive.ProactiveManager
	Scheduler  *proactive.Scheduler
	KnownUsers *store.KnownUsersStore
	RefIndex   *store.RefIndexStore
	Sessions   *store.SessionStore
	TTS        *audio.TTSProvider
	DataDir    string

	eventHandler  gateway.EventHandler
	forwarders  []EventHandler
}

// GetID returns the account ID.
func (a *Account) GetID() string { return a.ID }

// IsConnected returns whether the gateway is connected.
func (a *Account) IsConnected() bool { return a.Gateway.IsConnected() }

// AccountStatus represents the current status of a managed account.
type AccountStatus struct {
	ID        string
	Connected bool
	Error     string
}

// GetID returns the account ID (implements httpapi status interface).
func (s AccountStatus) GetID() string { return s.ID }

// IsConnected returns whether the account is connected.
func (s AccountStatus) IsConnected() bool { return s.Connected }

// BotManager manages multiple QQ Bot accounts concurrently with per-account isolation.
type BotManager struct {
	accounts        map[string]*Account
	mu              sync.RWMutex
	dataDir         string
	db              *store.DB
	started         bool
	ctx             context.Context
	cancel          context.CancelFunc
	webhook         WebhookDispatcher
	eventForwarders []EventHandler
}

// NewBotManager creates a new BotManager that stores data in dataDir.
func NewBotManager(dataDir string) (*BotManager, error) {
	db, err := store.Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &BotManager{
		accounts: make(map[string]*Account),
		dataDir:  dataDir,
		db:       db,
	}, nil
}

// AddAccount adds a new account to the manager. Each account gets its own
// isolated APIClient, Gateway, stores, and handlers.
func (m *BotManager) AddAccount(account types.ResolvedQQBotAccount) error {
	if account.AppID == "" {
		return fmt.Errorf("invalid config: appId is required for account %q", account.AccountID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.accounts[account.AccountID]; exists {
		return fmt.Errorf("account %q already exists", account.AccountID)
	}

	// Resolve image server URL
	imgServerURL := ""
	if account.ImageServerBaseUrl != nil {
		imgServerURL = *account.ImageServerBaseUrl
	}

	client := api.NewAPIClient(api.WithMarkdownSupport(account.MarkdownSupport))
	userStore := store.NewKnownUsersStore(m.db)
	refStore := store.NewRefIndexStore(m.db)
	sessionStore := store.NewSessionStore(m.db)
	reminderStore := store.NewReminderStore(m.db)

	eventHandler := m.newEventHandler(userStore)

	gw := gateway.NewGateway(account.AccountID, client, eventHandler)
	gw.SetSessionStore(newGatewaySessionAdapter(sessionStore), account.AppID)
	outHandler := outbound.NewOutboundHandler(client, imgServerURL)
	proMgr := proactive.NewProactiveManager(client, userStore)
	scheduler := proactive.NewScheduler(proMgr)
	scheduler.SetStore(newReminderStoreAdapter(reminderStore))

	ttsProvider := audio.NewTTSProvider(audio.TTSConfig{
		Voice: account.TTSVoice,
	})

	acct := &Account{
		ID:           account.AccountID,
		Config:       account,
		Client:       client,
		Gateway:      gw,
		Outbound:     outHandler,
		Proactive:    proMgr,
		Scheduler:    scheduler,
		KnownUsers:   userStore,
		RefIndex:     refStore,
		Sessions:     sessionStore,
		TTS:          ttsProvider,
		DataDir:      m.dataDir,
		eventHandler: eventHandler,
	}

	// Register webhook URL if dispatcher is available
	if m.webhook != nil && account.WebhookURL != "" {
		m.webhook.SetURL(account.AccountID, account.WebhookURL)
	}

	m.accounts[account.AccountID] = acct
	return nil
}

// newEventHandler creates an event handler that records users in the known users store
// and forwards events to the webhook dispatcher.
func (m *BotManager) newEventHandler(userStore *store.KnownUsersStore) gateway.EventHandler {
	return func(accountID string, eventType string, payload []byte) {
		switch eventType {
		case "C2C_MESSAGE_CREATE":
			var event types.C2CMessageEvent
			if err := json.Unmarshal(payload, &event); err != nil {
				return
			}
			openid := event.Author.UserOpenID
			if openid == "" {
				openid = event.Author.UnionOpenID
			}
			if openid != "" {
				userStore.Record(store.KnownUser{
					OpenID:    openid,
					Type:      "c2c",
					AccountID: accountID,
				})
			}

		case "GROUP_AT_MESSAGE_CREATE":
			var event types.GroupMessageEvent
			if err := json.Unmarshal(payload, &event); err != nil {
				return
			}
			userStore.Record(store.KnownUser{
				OpenID:      event.Author.MemberOpenID,
				Type:        "group",
				GroupOpenID: event.GroupOpenID,
				AccountID:   accountID,
			})
		}

		// Forward all events to webhook dispatcher
		if m.webhook != nil {
			m.webhook.Dispatch(accountID, eventType, payload)
		}

		// Forward to additional event handlers (e.g., embedded channel)
		m.mu.RLock()
		forwarders := m.eventForwarders
		m.mu.RUnlock()
		for _, fn := range forwarders {
			fn(accountID, eventType, payload)
		}
	}
}

// RemoveAccount removes and cleans up an account.
func (m *BotManager) RemoveAccount(accountID string) error {
	m.mu.Lock()
	acct, exists := m.accounts[accountID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("account %q not found", accountID)
	}
	delete(m.accounts, accountID)
	m.mu.Unlock()

	if acct.Scheduler != nil {
		acct.Scheduler.Stop()
	}
	if acct.Gateway != nil {
		acct.Gateway.Close()
	}
	if acct.Client != nil {
		acct.Client.Close()
	}
	acct.KnownUsers.Close()
	acct.RefIndex.Close()
	acct.Sessions.Close()

	return nil
}

// Start initializes all account API clients and schedulers.
// It does not block — gateway connections are managed individually.
func (m *BotManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("manager is already started")
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true

	for _, acct := range m.accounts {
		acct.Client.Init(m.ctx, acct.Config.AppID, acct.Config.ClientSecret)
		acct.Scheduler.Start(m.ctx)
	}
	m.mu.Unlock()

	// Start gateway connections in background
	go func() {
		for _, acct := range m.accounts {
			go func(a *Account) {
				if err := a.Gateway.Connect(m.ctx); err != nil {
					// Connection errors are logged in gateway, just continue
				}
			}(acct)
		}
	}()

	return nil
}

// Stop gracefully shuts down all accounts: schedulers, gateways, flush stores, close clients.
func (m *BotManager) Stop() {
	m.mu.Lock()
	wasStarted := m.started
	m.started = false
	if wasStarted && m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	accounts := make([]*Account, 0, len(m.accounts))
	for _, acct := range m.accounts {
		accounts = append(accounts, acct)
	}
	m.mu.Unlock()

	for _, acct := range accounts {
		if acct.Scheduler != nil {
			acct.Scheduler.Stop()
		}
	}
	for _, acct := range accounts {
		if acct.Gateway != nil {
			acct.Gateway.Close()
		}
	}
	for _, acct := range accounts {
		acct.KnownUsers.Flush()
		acct.RefIndex.Flush()
		acct.Sessions.Flush()
	}
	for _, acct := range accounts {
		acct.KnownUsers.Close()
		acct.RefIndex.Close()
		acct.Sessions.Close()
		if acct.Client != nil {
			acct.Client.Close()
		}
	}
	if m.db != nil {
		m.db.Close()
		m.db = nil
	}
}

// GetAccount returns the account with the given ID, or nil.
func (m *BotManager) GetAccount(id string) *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accounts[id]
}

// GetAllStatuses returns connection status for all managed accounts.
func (m *BotManager) GetAllStatuses() []AccountStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]AccountStatus, 0, len(m.accounts))
	for _, acct := range m.accounts {
		statuses = append(statuses, AccountStatus{
			ID:        acct.ID,
			Connected: acct.Gateway.IsConnected(),
		})
	}
	return statuses
}

// SendC2C sends a text message to a C2C user via the specified account.
func (m *BotManager) SendC2C(ctx context.Context, accountID, openid, content, msgID string) error {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("unknown account: %s", accountID)
	}
	return acct.Outbound.SendText(ctx, outbound.Target{Type: "c2c", OpenID: openid}, content, msgID)
}

// SendGroup sends a text message to a group via the specified account.
func (m *BotManager) SendGroup(ctx context.Context, accountID, groupOpenID, content, msgID string) error {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return fmt.Errorf("unknown account: %s", accountID)
	}
	return acct.Outbound.SendText(ctx, outbound.Target{Type: "group", OpenID: groupOpenID}, content, msgID)
}

// Broadcast sends a message to all known C2C users for an account.
func (m *BotManager) Broadcast(ctx context.Context, accountID, content string) (sent int, errs []error) {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return 0, []error{fmt.Errorf("unknown account: %s", accountID)}
	}
	return acct.Proactive.Broadcast(ctx, accountID, content)
}

// SendImage sends an image message to a target.
func (m *BotManager) SendImage(ctx context.Context, accountID, targetType, targetID, imageURL, content, msgID string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Outbound.SendImage(ctx, outbound.Target{Type: targetType, OpenID: targetID}, imageURL, content, msgID)
}

// SendVoice sends a voice message to a target.
// If voiceBase64 is empty but ttsText is provided, uses TTS to generate audio.
func (m *BotManager) SendVoice(ctx context.Context, accountID, targetType, targetID, voiceBase64, ttsText, msgID string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}

	// TTS fallback: generate audio from text if no voice data provided
	if voiceBase64 == "" && ttsText != "" {
		generated, err := acct.TTS.SynthesizeToSilkBase64(ttsText)
		if err != nil {
			return fmt.Errorf("TTS synthesis failed: %w", err)
		}
		voiceBase64 = generated
	}

	return acct.Outbound.SendVoice(ctx, outbound.Target{Type: targetType, OpenID: targetID}, voiceBase64, ttsText, msgID)
}

// SendVideo sends a video message to a target.
func (m *BotManager) SendVideo(ctx context.Context, accountID, targetType, targetID, videoURL, videoBase64, content, msgID string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Outbound.SendVideo(ctx, outbound.Target{Type: targetType, OpenID: targetID}, videoURL, videoBase64, content, msgID)
}

// SendFile sends a file message to a target.
func (m *BotManager) SendFile(ctx context.Context, accountID, targetType, targetID, fileBase64, fileURL, fileName, msgID string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Outbound.SendFile(ctx, outbound.Target{Type: targetType, OpenID: targetID}, fileBase64, fileURL, fileName, msgID)
}

// SendChannel sends a text message to a guild channel.
func (m *BotManager) SendChannel(ctx context.Context, accountID, channelID, content, msgID string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Outbound.SendText(ctx, outbound.Target{Type: "channel", OpenID: channelID}, content, msgID)
}

// SendProactiveC2C sends a proactive text message to a C2C user.
func (m *BotManager) SendProactiveC2C(ctx context.Context, accountID, openid, content string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Proactive.SendC2C(ctx, openid, content)
}

// SendProactiveGroup sends a proactive text message to a group.
func (m *BotManager) SendProactiveGroup(ctx context.Context, accountID, groupOpenID, content string) error {
	acct, err := m.lookupAccount(accountID)
	if err != nil {
		return err
	}
	return acct.Proactive.SendGroup(ctx, groupOpenID, content)
}

// BroadcastToGroups sends a message to all known groups for an account.
func (m *BotManager) BroadcastToGroups(ctx context.Context, accountID, content string) (sent int, errs []error) {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return 0, []error{fmt.Errorf("unknown account: %s", accountID)}
	}
	return acct.Proactive.BroadcastToGroup(ctx, accountID, content)
}

// ListUsers returns known users for an account with optional filtering.
func (m *BotManager) ListUsers(accountID string, opts store.ListOptions) []store.KnownUser {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return nil
	}
	return acct.Proactive.ListUsers(accountID, opts)
}

// GetUserStats returns user statistics for an account.
func (m *BotManager) GetUserStats(accountID string) store.UserStats {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return store.UserStats{}
	}
	return acct.Proactive.GetUserStats(accountID)
}

// ClearUsers removes all known users for an account. Returns count of removed users.
func (m *BotManager) ClearUsers(accountID string) int {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return 0
	}
	return acct.KnownUsers.Clear(accountID)
}

// AddReminder adds a scheduled reminder job for an account. Returns the job ID.
func (m *BotManager) AddReminder(job proactive.ReminderJob) (string, error) {
	m.mu.RLock()
	acct, exists := m.accounts[job.AccountID]
	m.mu.RUnlock()
	if !exists {
		return "", fmt.Errorf("unknown account: %s", job.AccountID)
	}
	return acct.Scheduler.AddReminder(job), nil
}

// CancelReminder removes a scheduled reminder job. Returns true if found.
func (m *BotManager) CancelReminder(accountID, jobID string) bool {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return false
	}
	return acct.Scheduler.CancelReminder(jobID)
}

// GetReminders returns all scheduled reminders for an account.
func (m *BotManager) GetReminders(accountID string) []proactive.ReminderJob {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return nil
	}
	return acct.Scheduler.GetReminders()
}

// ListAccountIDs returns all managed account IDs.
func (m *BotManager) ListAccountIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.accounts))
	for id := range m.accounts {
		ids = append(ids, id)
	}
	return ids
}

// SetWebhookDispatcher sets the webhook dispatcher for event forwarding.
// SetEventHandler sets additional event handlers that are called for every event
// on every account. Used by the embedded channel mode to bridge events to MCP.
func (m *BotManager) SetEventHandler(h EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventForwarders = append(m.eventForwarders, h)
}

func (m *BotManager) SetWebhookDispatcher(d WebhookDispatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhook = d
	// Register webhook URLs for existing accounts
	for id, acct := range m.accounts {
		if acct.Config.WebhookURL != "" {
			d.SetURL(id, acct.Config.WebhookURL)
		}
	}
}

// lookupAccount returns the account with the given ID under read lock.
func (m *BotManager) lookupAccount(accountID string) (*Account, error) {
	m.mu.RLock()
	acct, exists := m.accounts[accountID]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unknown account: %s", accountID)
	}
	return acct, nil
}
