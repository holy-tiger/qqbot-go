package proactive

import (
	"context"
	"fmt"

	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

// MessageSender abstracts the proactive message API.
type MessageSender interface {
	SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error)
	SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error)
}

// ProactiveManager handles proactive messaging and user queries.
type ProactiveManager struct {
	client    MessageSender
	userStore *store.KnownUsersStore
}

// NewProactiveManager creates a new ProactiveManager.
func NewProactiveManager(client MessageSender, userStore *store.KnownUsersStore) *ProactiveManager {
	return &ProactiveManager{
		client:    client,
		userStore: userStore,
	}
}

// SendC2C sends a proactive message to a C2C user.
func (m *ProactiveManager) SendC2C(ctx context.Context, openid, content string) error {
	_, err := m.client.SendProactiveC2CMessage(ctx, openid, content)
	return err
}

// SendGroup sends a proactive message to a group.
func (m *ProactiveManager) SendGroup(ctx context.Context, groupOpenID, content string) error {
	_, err := m.client.SendProactiveGroupMessage(ctx, groupOpenID, content)
	return err
}

// SendToUser sends to a user by accountID + openid. The user must be known.
func (m *ProactiveManager) SendToUser(ctx context.Context, accountID, openid, content string) error {
	user := m.userStore.Get(accountID, openid, "c2c", "")
	if user == nil {
		return fmt.Errorf("unknown user: %s/%s", accountID, openid)
	}
	_, err := m.client.SendProactiveC2CMessage(ctx, openid, content)
	return err
}

// SendToGroup sends to a group by accountID + groupOpenID. The group must be known.
func (m *ProactiveManager) SendToGroup(ctx context.Context, accountID, groupOpenID, content string) error {
	groups := m.userStore.GetGroupMembers(accountID, groupOpenID)
	if len(groups) == 0 {
		return fmt.Errorf("unknown group: %s/%s", accountID, groupOpenID)
	}
	_, err := m.client.SendProactiveGroupMessage(ctx, groupOpenID, content)
	return err
}

// Broadcast sends a message to all known C2C users for an account.
func (m *ProactiveManager) Broadcast(ctx context.Context, accountID, content string) (sent int, errs []error) {
	users := m.userStore.List(store.ListOptions{AccountID: accountID, Type: "c2c"})
	for _, u := range users {
		if _, err := m.client.SendProactiveC2CMessage(ctx, u.OpenID, content); err != nil {
			errs = append(errs, err)
		} else {
			sent++
		}
	}
	return sent, errs
}

// BroadcastToGroup sends to all known groups for an account (deduplicated).
func (m *ProactiveManager) BroadcastToGroup(ctx context.Context, accountID, content string) (sent int, errs []error) {
	users := m.userStore.List(store.ListOptions{AccountID: accountID, Type: "group"})
	seen := make(map[string]bool)
	for _, u := range users {
		if u.GroupOpenID == "" || seen[u.GroupOpenID] {
			continue
		}
		seen[u.GroupOpenID] = true
		if _, err := m.client.SendProactiveGroupMessage(ctx, u.GroupOpenID, content); err != nil {
			errs = append(errs, err)
		} else {
			sent++
		}
	}
	return sent, errs
}

// GetUserStats returns user statistics.
func (m *ProactiveManager) GetUserStats(accountID string) store.UserStats {
	return m.userStore.Stats(accountID)
}

// ListUsers returns known users with optional filtering.
func (m *ProactiveManager) ListUsers(accountID string, opts store.ListOptions) []store.KnownUser {
	opts.AccountID = accountID
	return m.userStore.List(opts)
}
