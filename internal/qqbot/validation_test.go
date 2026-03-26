package qqbot

import (
	"strings"
	"testing"

	"github.com/openclaw/qqbot/internal/config"
	"github.com/openclaw/qqbot/internal/types"
)

func strPtr(s string) *string {
	return &s
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID:        strPtr("app123"),
				ClientSecret: strPtr("secret"),
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.Valid {
		t.Error("expected config to be valid")
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidateConfig_NoAccounts(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{},
		},
	}

	result := ValidateConfig(cfg)
	if result.Valid {
		t.Error("expected config to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "appId") || strings.Contains(e, "account") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected appId/account error, got: %v", result.Errors)
	}
}

func TestValidateConfig_EmptyAppID(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID: strPtr(""),
			},
			Accounts: map[string]*types.QQBotAccountConfig{
				"bot2": {
					AppID: strPtr(""),
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if result.Valid {
		t.Error("expected config to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error for empty appId")
	}
}

func TestValidateConfig_MissingSecret(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID: strPtr("app123"),
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.Valid {
		t.Error("expected config to be valid even without secret")
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing client secret")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(strings.ToLower(w), "secret") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected clientSecret warning, got: %v", result.Warnings)
	}
}

func TestValidateConfig_MultiAccount(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID:        strPtr("app123"),
				ClientSecret: strPtr("secret1"),
			},
			Accounts: map[string]*types.QQBotAccountConfig{
				"bot2": {
					AppID:        strPtr("app456"),
					ClientSecret: strPtr("secret2"),
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	if !result.Valid {
		t.Errorf("expected config to be valid, errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidateConfig_NilConfig(t *testing.T) {
	result := ValidateConfig(nil)
	if result.Valid {
		t.Error("expected nil config to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error for nil config")
	}
}

func TestValidateConfig_DuplicateAccountID(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID:        strPtr("app123"),
				ClientSecret: strPtr("secret"),
			},
			Accounts: map[string]*types.QQBotAccountConfig{
				"default": {
					AppID: strPtr("app456"),
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	// "default" is both the top-level account and a named account
	if len(result.Errors) == 0 {
		t.Error("expected error for duplicate account ID 'default'")
	}
}

func TestValidateConfig_NoQQBotSection(t *testing.T) {
	cfg := &config.QQBotConfig{}

	result := ValidateConfig(cfg)
	if result.Valid {
		t.Error("expected config with no qqbot section to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestValidateConfig_AccountInMapWithEmptyAppID(t *testing.T) {
	cfg := &config.QQBotConfig{
		QQBot: &config.QQBotChannelConfig{
			QQBotAccountConfig: types.QQBotAccountConfig{
				AppID:        strPtr("app123"),
				ClientSecret: strPtr("secret"),
			},
			Accounts: map[string]*types.QQBotAccountConfig{
				"bot2": {
					AppID: nil,
				},
			},
		},
	}

	result := ValidateConfig(cfg)
	// Should still be valid since top-level account has appId
	// but the named account with nil appId should produce an error or warning
	// Actually, since the top-level has a valid appId, the config is usable
	// But accounts with empty appID should produce an error
	if result.Valid {
		t.Error("expected config with empty appId in named account to be invalid")
	}
}
