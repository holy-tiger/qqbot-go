package config

import (
	"os"

	"github.com/openclaw/qqbot/internal/types"
	"gopkg.in/yaml.v3"
)

const DefaultAccountID = "default"

// QQBotChannelConfig extends QQBotAccountConfig with a named accounts map.
// This maps to the channels.qqbot section in the YAML config.
type QQBotChannelConfig struct {
	types.QQBotAccountConfig          `yaml:",inline"`
	Accounts                 map[string]*types.QQBotAccountConfig `yaml:"accounts,omitempty"`
	DefaultWebhookURL        *string                            `yaml:"defaultWebhookUrl,omitempty"`
}

// QQBotConfig is the top-level configuration loaded from YAML.
type QQBotConfig struct {
	QQBot *QQBotChannelConfig `yaml:"qqbot"`
}

// LoadConfig reads and parses a YAML configuration file.
func LoadConfig(path string) (*QQBotConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg QQBotConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListAccountIDs returns all account IDs that have an appId configured.
func ListAccountIDs(cfg *QQBotConfig) []string {
	if cfg == nil || cfg.QQBot == nil {
		return nil
	}
	seen := make(map[string]bool)
	var ids []string

	if cfg.QQBot.AppID != nil && *cfg.QQBot.AppID != "" {
		seen[DefaultAccountID] = true
		ids = append(ids, DefaultAccountID)
	}
	for name, account := range cfg.QQBot.Accounts {
		if account != nil && account.AppID != nil && *account.AppID != "" {
			if !seen[name] {
				seen[name] = true
				ids = append(ids, name)
			}
		}
	}
	return ids
}

// ResolveAccount resolves a QQBot account configuration by account ID.
func ResolveAccount(cfg *QQBotConfig, accountID string) types.ResolvedQQBotAccount {
	if accountID == "" {
		accountID = DefaultAccountID
	}

	var accountConfig types.QQBotAccountConfig
	if cfg != nil && cfg.QQBot != nil {
		if accountID == DefaultAccountID {
			// Default account reads from top-level qqbot fields
			accountConfig = cfg.QQBot.QQBotAccountConfig
		} else {
			// Named account reads from accounts map
			if account, ok := cfg.QQBot.Accounts[accountID]; ok && account != nil {
				accountConfig = *account
			}
		}
	}

	appID := normalizeAppID(accountConfig.AppID)
	clientSecret := ""
	secretSource := "none"

	// Resolve clientSecret: config > file > env (default account only)
	if accountConfig.ClientSecret != nil && *accountConfig.ClientSecret != "" {
		clientSecret = *accountConfig.ClientSecret
		secretSource = "config"
	} else if accountConfig.ClientSecretFile != nil && *accountConfig.ClientSecretFile != "" {
		if data, err := os.ReadFile(*accountConfig.ClientSecretFile); err == nil {
			clientSecret = string(data)
		}
		secretSource = "file"
	} else if accountID == DefaultAccountID {
		if envSecret := os.Getenv("QQBOT_CLIENT_SECRET"); envSecret != "" {
			clientSecret = envSecret
			secretSource = "env"
		}
	}

	// AppID from env for default account
	if appID == "" && accountID == DefaultAccountID {
		if envAppID := os.Getenv("QQBOT_APP_ID"); envAppID != "" {
			appID = envAppID
		}
	}

	// ImageServerBaseUrl fallback
	imageServerBaseUrl := accountConfig.ImageServerBaseUrl
	if imageServerBaseUrl == nil || *imageServerBaseUrl == "" {
		if envURL := os.Getenv("QQBOT_IMAGE_SERVER_BASE_URL"); envURL != "" {
			imageServerBaseUrl = &envURL
		}
	}

	// Defaults
	enabled := true
	if accountConfig.Enabled != nil {
		enabled = *accountConfig.Enabled
	}
	markdownSupport := true
	if accountConfig.MarkdownSupport != nil {
		markdownSupport = *accountConfig.MarkdownSupport
	}

	// WebhookURL: per-account webhookUrl > defaultWebhookUrl > empty
	webhookURL := accountConfig.WebhookURL
	if (webhookURL == nil || *webhookURL == "") && cfg != nil && cfg.QQBot != nil && cfg.QQBot.DefaultWebhookURL != nil {
		webhookURL = cfg.QQBot.DefaultWebhookURL
	}
	resolvedWebhookURL := ""
	if webhookURL != nil {
		resolvedWebhookURL = *webhookURL
	}

	return types.ResolvedQQBotAccount{
		AccountID:         accountID,
		Name:              accountConfig.Name,
		Enabled:           enabled,
		AppID:             appID,
		ClientSecret:      clientSecret,
		SecretSource:      secretSource,
		SystemPrompt:      accountConfig.SystemPrompt,
		ImageServerBaseUrl: imageServerBaseUrl,
		MarkdownSupport:   markdownSupport,
		TTSVoice:          derefString(accountConfig.TTSVoice),
		WebhookURL:        resolvedWebhookURL,
		Config:            &accountConfig,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeAppID(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
