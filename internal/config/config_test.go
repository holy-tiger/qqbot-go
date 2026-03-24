package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openclaw/qqbot/internal/types"
)

// writeTestConfigFile creates a temporary YAML config file.
func writeTestConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

// writeTestSecretFile creates a temporary file with the given content.
func writeTestSecretFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	yaml := `
qqbot:
  appId: "app123"
  clientSecret: "secret456"
  name: "TestBot"
  enabled: true
  systemPrompt: "be helpful"
  allowFrom:
    - "*"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.QQBot == nil {
		t.Fatal("QQBot should not be nil")
	}
	if cfg.QQBot.AppID == nil || *cfg.QQBot.AppID != "app123" {
		t.Errorf("AppID = %v, want app123", cfg.QQBot.AppID)
	}
	if cfg.QQBot.Name == nil || *cfg.QQBot.Name != "TestBot" {
		t.Errorf("Name = %v, want TestBot", cfg.QQBot.Name)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := writeTestConfigFile(t, "invalid: yaml: content:\n  -")
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestListAccountIDs_DefaultOnly(t *testing.T) {
	yaml := `
qqbot:
  appId: "app123"
  clientSecret: "secret"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	ids := ListAccountIDs(cfg)
	if len(ids) != 1 {
		t.Fatalf("got %d account IDs, want 1", len(ids))
	}
	if ids[0] != "default" {
		t.Errorf("got %q, want default", ids[0])
	}
}

func TestListAccountIDs_MultiAccount(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  clientSecret: "sec1"
  accounts:
    bot2:
      appId: "app2"
    bot3:
      appId: "app3"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	ids := ListAccountIDs(cfg)
	if len(ids) != 3 {
		t.Fatalf("got %d account IDs, want 3", len(ids))
	}
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, want := range []string{"default", "bot2", "bot3"} {
		if !idSet[want] {
			t.Errorf("missing account ID %q", want)
		}
	}
}

func TestResolveAccount_DefaultAccount(t *testing.T) {
	yaml := `
qqbot:
  appId: "app123"
  clientSecret: "mysecret"
  name: "MyBot"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.AccountID != "default" {
		t.Errorf("AccountID = %q, want default", acc.AccountID)
	}
	if acc.AppID != "app123" {
		t.Errorf("AppID = %q, want app123", acc.AppID)
	}
	if acc.ClientSecret != "mysecret" {
		t.Errorf("ClientSecret = %q, want mysecret", acc.ClientSecret)
	}
	if acc.SecretSource != "config" {
		t.Errorf("SecretSource = %q, want config", acc.SecretSource)
	}
	if !acc.Enabled {
		t.Error("Enabled should be true")
	}
	if acc.Name == nil || *acc.Name != "MyBot" {
		t.Errorf("Name = %v, want MyBot", acc.Name)
	}
	if !acc.MarkdownSupport {
		t.Error("MarkdownSupport should default to true")
	}
}

func TestResolveAccount_NamedAccount(t *testing.T) {
	yaml := `
qqbot:
  appId: "default-app"
  clientSecret: "default-sec"
  accounts:
    bot2:
      appId: "app2"
      clientSecret: "sec2"
      name: "Bot Two"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "bot2")
	if acc.AccountID != "bot2" {
		t.Errorf("AccountID = %q, want bot2", acc.AccountID)
	}
	if acc.AppID != "app2" {
		t.Errorf("AppID = %q, want app2", acc.AppID)
	}
	if acc.ClientSecret != "sec2" {
		t.Errorf("ClientSecret = %q, want sec2", acc.ClientSecret)
	}
	if acc.SecretSource != "config" {
		t.Errorf("SecretSource = %q, want config", acc.SecretSource)
	}
}

func TestResolveAccount_DisabledAccount(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  accounts:
    bot2:
      appId: "app2"
      enabled: false
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "bot2")
	if acc.Enabled {
		t.Error("Enabled should be false")
	}
}

func TestResolveAccount_MarkdownDisabled(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  markdownSupport: false
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.MarkdownSupport {
		t.Error("MarkdownSupport should be false when explicitly set to false")
	}
}

func TestResolveAccount_SecretFromFile(t *testing.T) {
	secretPath := writeTestSecretFile(t, "file-secret-value")
	yaml := `
qqbot:
  appId: "app1"
  clientSecretFile: "` + secretPath + `"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.SecretSource != "file" {
		t.Errorf("SecretSource = %q, want file", acc.SecretSource)
	}
	if acc.ClientSecret != "file-secret-value" {
		t.Errorf("ClientSecret = %q, want file-secret-value", acc.ClientSecret)
	}
}

func TestResolveAccount_EnvFallback(t *testing.T) {
	t.Setenv("QQBOT_CLIENT_SECRET", "env-secret")
	t.Setenv("QQBOT_APP_ID", "env-app")

	yaml := `
qqbot:
  name: "EnvBot"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.AppID != "env-app" {
		t.Errorf("AppID = %q, want env-app", acc.AppID)
	}
	if acc.SecretSource != "env" {
		t.Errorf("SecretSource = %q, want env", acc.SecretSource)
	}
	if acc.ClientSecret != "env-secret" {
		t.Errorf("ClientSecret = %q, want env-secret", acc.ClientSecret)
	}
}

func TestResolveAccount_ImageServerBaseUrl(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  imageServerBaseUrl: "http://example.com:18765"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.ImageServerBaseUrl == nil || *acc.ImageServerBaseUrl != "http://example.com:18765" {
		t.Errorf("ImageServerBaseUrl = %v, want http://example.com:18765", acc.ImageServerBaseUrl)
	}
}

func TestResolveAccount_ImageServerBaseUrlEnvFallback(t *testing.T) {
	t.Setenv("QQBOT_IMAGE_SERVER_BASE_URL", "http://env-url:18765")

	yaml := `
qqbot:
  appId: "app1"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.ImageServerBaseUrl == nil || *acc.ImageServerBaseUrl != "http://env-url:18765" {
		t.Errorf("ImageServerBaseUrl = %v, want http://env-url:18765", acc.ImageServerBaseUrl)
	}
}

func TestResolveAccount_NoConfig(t *testing.T) {
	yaml := `{}`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.AccountID != "default" {
		t.Errorf("AccountID = %q, want default", acc.AccountID)
	}
	if acc.SecretSource != "none" {
		t.Errorf("SecretSource = %q, want none", acc.SecretSource)
	}
	if !acc.Enabled {
		t.Error("Enabled should default to true")
	}
}

func TestResolveAccount_WithAudioFormatPolicy(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  clientSecret: "sec1"
  audioFormatPolicy:
    stt_direct_formats:
      - ".silk"
      - ".amr"
    upload_direct_formats:
      - ".wav"
      - ".mp3"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.Config == nil || acc.Config.AudioFormatPolicy == nil {
		t.Fatal("AudioFormatPolicy should not be nil")
	}
	policy := acc.Config.AudioFormatPolicy
	if len(policy.STTDirectFormats) != 2 {
		t.Errorf("STTDirectFormats length = %d, want 2", len(policy.STTDirectFormats))
	}
	if len(policy.UploadDirectFormats) != 2 {
		t.Errorf("UploadDirectFormats length = %d, want 2", len(policy.UploadDirectFormats))
	}
}

func TestResolveAccount_SystemPrompt(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  systemPrompt: "You are a helpful bot."
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	acc := ResolveAccount(cfg, "default")
	if acc.SystemPrompt == nil || *acc.SystemPrompt != "You are a helpful bot." {
		t.Errorf("SystemPrompt = %v, want 'You are a helpful bot.'", acc.SystemPrompt)
	}
}

func TestResolveAccount_ReturnsTypesPackageType(t *testing.T) {
	yaml := `
qqbot:
  appId: "app1"
  clientSecret: "sec1"
`
	path := writeTestConfigFile(t, yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	var acc types.ResolvedQQBotAccount = ResolveAccount(cfg, "default")
	if acc.AccountID != "default" {
		t.Errorf("AccountID = %q", acc.AccountID)
	}
}
