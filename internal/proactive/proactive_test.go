package proactive

import (
	"context"
	"errors"
	"testing"

	"github.com/openclaw/qqbot/internal/store"
	"github.com/openclaw/qqbot/internal/types"
)

// mockSender implements MessageSender for testing.
type mockSender struct {
	sendC2CFn   func(ctx context.Context, openid, content string) (types.MessageResponse, error)
	sendGroupFn func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error)
}

func (m *mockSender) SendProactiveC2CMessage(ctx context.Context, openid, content string) (types.MessageResponse, error) {
	return m.sendC2CFn(ctx, openid, content)
}

func (m *mockSender) SendProactiveGroupMessage(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
	return m.sendGroupFn(ctx, groupOpenID, content)
}

func successResponse() types.MessageResponse {
	return types.MessageResponse{ID: "msg123", Timestamp: "2025-01-01T00:00:00Z"}
}

// TestSendC2C tests sending a proactive C2C message.
func TestSendC2C(t *testing.T) {
	sender := &mockSender{
		sendC2CFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
			if openid != "user1" {
				t.Errorf("expected openid user1, got %s", openid)
			}
			if content != "hello" {
				t.Errorf("expected content hello, got %s", content)
			}
			return successResponse(), nil
		},
	}

	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	err := mgr.SendC2C(context.Background(), "user1", "hello")
	if err != nil {
		t.Fatalf("SendC2C returned error: %v", err)
	}
}

// TestSendC2C_Error tests error propagation from the API.
func TestSendC2C_Error(t *testing.T) {
	expectedErr := errors.New("API error")
	sender := &mockSender{
		sendC2CFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
			return types.MessageResponse{}, expectedErr
		},
	}

	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	err := mgr.SendC2C(context.Background(), "user1", "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestSendGroup tests sending a proactive group message.
func TestSendGroup(t *testing.T) {
	sender := &mockSender{
		sendGroupFn: func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
			if groupOpenID != "group1" {
				t.Errorf("expected groupOpenID group1, got %s", groupOpenID)
			}
			if content != "group hello" {
				t.Errorf("expected content 'group hello', got %s", content)
			}
			return successResponse(), nil
		},
	}

	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	err := mgr.SendGroup(context.Background(), "group1", "group hello")
	if err != nil {
		t.Fatalf("SendGroup returned error: %v", err)
	}
}

// TestSendGroup_Error tests error propagation for group messages.
func TestSendGroup_Error(t *testing.T) {
	expectedErr := errors.New("group API error")
	sender := &mockSender{
		sendGroupFn: func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
			return types.MessageResponse{}, expectedErr
		},
	}

	mgr := NewProactiveManager(sender, store.NewKnownUsersStore(t.TempDir()))
	err := mgr.SendGroup(context.Background(), "group1", "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestSendToUser tests sending to a user with account validation.
func TestSendToUser(t *testing.T) {
	t.Run("known user succeeds", func(t *testing.T) {
		dir := t.TempDir()
		us := store.NewKnownUsersStore(dir)
		us.Record(store.KnownUser{
			OpenID:    "user1",
			Type:      "c2c",
			AccountID: "acct1",
		})
		defer us.Close()

		sender := &mockSender{
			sendC2CFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
				return successResponse(), nil
			},
		}

		mgr := NewProactiveManager(sender, us)
		err := mgr.SendToUser(context.Background(), "acct1", "user1", "hello")
		if err != nil {
			t.Fatalf("SendToUser returned error: %v", err)
		}
	})

	t.Run("unknown user returns error", func(t *testing.T) {
		dir := t.TempDir()
		us := store.NewKnownUsersStore(dir)
		defer us.Close()

		sender := &mockSender{
			sendC2CFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
				t.Fatal("should not be called for unknown user")
				return types.MessageResponse{}, nil
			},
		}

		mgr := NewProactiveManager(sender, us)
		err := mgr.SendToUser(context.Background(), "acct1", "unknown", "hello")
		if err == nil {
			t.Fatal("expected error for unknown user")
		}
	})
}

// TestSendToGroup tests sending to a group with account validation.
func TestSendToGroup(t *testing.T) {
	t.Run("known group succeeds", func(t *testing.T) {
		dir := t.TempDir()
		us := store.NewKnownUsersStore(dir)
		us.Record(store.KnownUser{
			OpenID:      "member1",
			Type:        "group",
			GroupOpenID: "group1",
			AccountID:   "acct1",
		})
		defer us.Close()

		sender := &mockSender{
			sendGroupFn: func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
				return successResponse(), nil
			},
		}

		mgr := NewProactiveManager(sender, us)
		err := mgr.SendToGroup(context.Background(), "acct1", "group1", "hello")
		if err != nil {
			t.Fatalf("SendToGroup returned error: %v", err)
		}
	})

	t.Run("unknown group returns error", func(t *testing.T) {
		dir := t.TempDir()
		us := store.NewKnownUsersStore(dir)
		defer us.Close()

		sender := &mockSender{
			sendGroupFn: func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
				t.Fatal("should not be called for unknown group")
				return types.MessageResponse{}, nil
			},
		}

		mgr := NewProactiveManager(sender, us)
		err := mgr.SendToGroup(context.Background(), "acct1", "unknown", "hello")
		if err == nil {
			t.Fatal("expected error for unknown group")
		}
	})
}

// TestBroadcast tests broadcasting to all known C2C users.
func TestBroadcast(t *testing.T) {
	dir := t.TempDir()
	us := store.NewKnownUsersStore(dir)
	us.Record(store.KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u2", Type: "c2c", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u3", Type: "group", AccountID: "acct1"})

	sendCount := 0
	broadcastErr := errors.New("send failed")
	sender := &mockSender{
		sendC2CFn: func(ctx context.Context, openid, content string) (types.MessageResponse, error) {
			sendCount++
			if openid == "u2" {
				return types.MessageResponse{}, broadcastErr
			}
			return successResponse(), nil
		},
	}

	mgr := NewProactiveManager(sender, us)
	sent, errs := mgr.Broadcast(context.Background(), "acct1", "hello")
	if sent != 1 {
		t.Errorf("expected 1 sent, got %d", sent)
	}
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
	if sendCount != 2 {
		t.Errorf("expected 2 send attempts (c2c only), got %d", sendCount)
	}
}

// TestBroadcastToGroup tests broadcasting to all known groups.
func TestBroadcastToGroup(t *testing.T) {
	dir := t.TempDir()
	us := store.NewKnownUsersStore(dir)
	us.Record(store.KnownUser{OpenID: "m1", Type: "group", GroupOpenID: "g1", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "m2", Type: "group", GroupOpenID: "g2", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "m3", Type: "group", GroupOpenID: "g2", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "m4", Type: "c2c", AccountID: "acct1"})

	var sentGroups []string
	sender := &mockSender{
		sendGroupFn: func(ctx context.Context, groupOpenID, content string) (types.MessageResponse, error) {
			sentGroups = append(sentGroups, groupOpenID)
			return successResponse(), nil
		},
	}

	mgr := NewProactiveManager(sender, us)
	sent, errs := mgr.BroadcastToGroup(context.Background(), "acct1", "hello")
	if sent != 2 {
		t.Errorf("expected 2 groups sent, got %d", sent)
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
	// Should have unique groups
	seen := make(map[string]bool)
	for _, g := range sentGroups {
		if seen[g] {
			t.Errorf("duplicate group send to %s", g)
		}
		seen[g] = true
	}
	us.Close()
}

// TestGetUserStats tests retrieving user statistics.
func TestGetUserStats(t *testing.T) {
	dir := t.TempDir()
	us := store.NewKnownUsersStore(dir)
	us.Record(store.KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u2", Type: "c2c", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u3", Type: "group", AccountID: "acct1"})

	mgr := NewProactiveManager(&mockSender{}, us)
	stats := mgr.GetUserStats("acct1")
	if stats.TotalUsers != 3 {
		t.Errorf("expected total 3, got %d", stats.TotalUsers)
	}
	if stats.C2CUsers != 2 {
		t.Errorf("expected c2c 2, got %d", stats.C2CUsers)
	}
	if stats.GroupUsers != 1 {
		t.Errorf("expected group 1, got %d", stats.GroupUsers)
	}
}

// TestListUsers tests listing known users.
func TestListUsers(t *testing.T) {
	dir := t.TempDir()
	us := store.NewKnownUsersStore(dir)
	us.Record(store.KnownUser{OpenID: "u1", Type: "c2c", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u2", Type: "group", AccountID: "acct1"})
	us.Record(store.KnownUser{OpenID: "u3", Type: "c2c", AccountID: "acct2"})

	mgr := NewProactiveManager(&mockSender{}, us)

	t.Run("filter by account", func(t *testing.T) {
		users := mgr.ListUsers("acct1", store.ListOptions{})
		if len(users) != 2 {
			t.Errorf("expected 2 users for acct1, got %d", len(users))
		}
	})

	t.Run("filter by type", func(t *testing.T) {
		users := mgr.ListUsers("acct1", store.ListOptions{Type: "c2c"})
		if len(users) != 1 {
			t.Errorf("expected 1 c2c user, got %d", len(users))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		users := mgr.ListUsers("acct1", store.ListOptions{Limit: 1})
		if len(users) != 1 {
			t.Errorf("expected 1 user with limit, got %d", len(users))
		}
	})
}

// TestNewProactiveManager tests constructor.
func TestNewProactiveManager(t *testing.T) {
	mgr := NewProactiveManager(nil, nil)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}
