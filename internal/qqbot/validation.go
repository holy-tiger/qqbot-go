package qqbot

import (
	"fmt"

	"github.com/openclaw/qqbot/internal/config"
)

// ValidationResult holds the result of validating a bot configuration.
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// ValidateConfig checks a QQBot configuration for errors and warnings.
func ValidateConfig(cfg *config.QQBotConfig) *ValidationResult {
	result := &ValidationResult{}

	if cfg == nil || cfg.QQBot == nil {
		result.Errors = append(result.Errors, "config: qqbot section is required")
		result.Valid = false
		return result
	}

	qqbot := cfg.QQBot

	// Check for duplicate account IDs (top-level "default" counts as an account ID)
	seenIDs := make(map[string]bool)
	hasValidAccount := false

	// Check top-level account
	topLevelAppID := ptrStr(qqbot.AppID)
	if topLevelAppID == "" {
		result.Errors = append(result.Errors, "config: top-level account has no appId configured")
	} else {
		hasValidAccount = true
		seenIDs[config.DefaultAccountID] = true
	}

	// Check named accounts
	for name, accountCfg := range qqbot.Accounts {
		if accountCfg == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("config: account %q is nil", name))
			continue
		}

		appID := ptrStr(accountCfg.AppID)
		if appID == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("config: account %q has no appId configured", name))
		} else {
			hasValidAccount = true
		}

		// Check for duplicate IDs
		if seenIDs[name] {
			result.Errors = append(result.Errors, fmt.Sprintf("config: duplicate account ID %q", name))
		}
		seenIDs[name] = true

		// Warn about missing client secret
		if ptrStr(accountCfg.ClientSecret) == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("config: account %q has no clientSecret configured", name))
		}
	}

	// At least one account must have a non-empty appId
	if !hasValidAccount {
		result.Errors = append(result.Errors, "config: at least one account must have a non-empty appId")
	}

	// Warn about top-level missing client secret
	if ptrStr(qqbot.ClientSecret) == "" && topLevelAppID != "" {
		result.Warnings = append(result.Warnings, "config: top-level account has no clientSecret configured")
	}

	result.Valid = len(result.Errors) == 0
	return result
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
