package qqbot

import (
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

func makeResolvedAccountFull(accountID, appID string) types.ResolvedQQBotAccount {
	imgURL := "http://localhost:18765"
	markdown := true
	return types.ResolvedQQBotAccount{
		AccountID:         accountID,
		AppID:             appID,
		ClientSecret:      "test-secret",
		Enabled:           true,
		ImageServerBaseUrl: &imgURL,
		MarkdownSupport:   markdown,
	}
}

// === Issue 1+2: ImageServerBaseUrl should be passed to OutboundHandler ===

func TestAddAccount_ImageServerUrlPassed(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccountFull("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acct := m.GetAccount("acct1")
	if acct == nil {
		t.Fatal("expected account to exist")
	}

	// Verify the outbound handler has the image server URL configured
	imgURL := acct.Outbound.GetImageServerURL()
	expectedURL := "http://localhost:18765"
	if imgURL != expectedURL {
		t.Errorf("expected imageServerURL %q, got %q", expectedURL, imgURL)
	}
}

// === Issue 3: Markdown support should be configured on APIClient ===

func TestAddAccount_MarkdownSupportEnabled(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccountFull("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acct := m.GetAccount("acct1")
	if acct == nil {
		t.Fatal("expected account to exist")
	}

	if !acct.Client.GetMarkdownSupport() {
		t.Error("expected markdownSupport to be true when config.MarkdownSupport is true")
	}
}

func TestAddAccount_MarkdownSupportDisabled(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	imgURL := "http://localhost:18765"
	account := types.ResolvedQQBotAccount{
		AccountID:         "acct1",
		AppID:             "app123",
		ClientSecret:      "test-secret",
		Enabled:           true,
		ImageServerBaseUrl: &imgURL,
		MarkdownSupport:   false,
	}
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acct := m.GetAccount("acct1")
	if acct.Client.GetMarkdownSupport() {
		t.Error("expected markdownSupport to be false when config.MarkdownSupport is false")
	}
}

// === Issue 4: SessionStore should be wired to Gateway ===

func TestAddAccount_SessionStoreWired(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewBotManager(dir)

	account := makeResolvedAccountFull("acct1", "app123")
	err := m.AddAccount(account)
	if err != nil {
		t.Fatalf("AddAccount failed: %v", err)
	}

	acct := m.GetAccount("acct1")
	if acct == nil {
		t.Fatal("expected account to exist")
	}

	// Verify the gateway has a session store configured
	if !acct.Gateway.HasSessionStore() {
		t.Error("expected Gateway to have a SessionStore wired")
	}
}

// === Issue 7: User listing should support all query params ===
// (This is tested via the httpapi handler tests, but we verify the
// ListOptions are properly forwarded from query params)
