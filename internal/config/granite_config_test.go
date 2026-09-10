package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const graniteYAML = `
Name: granite-controller
Host: 0.0.0.0
Port: 8080
Timeout: 130000
MaxBytes: 8388608
Granite:
  ModelID: granite-4.0-350m
  StartupTimeout: 120s
  RequestTimeout: 60s
  Threads: 4
  ContextSize: 4096
  MaxTokens: 256
  MaxMessageLength: 2048
  AllowForceStop: false
  ActionDedupWindow: 30s
InfraControl:
  BaseURL: http://infrastructure-control:8080
  Timeout: 30s
`

func graniteConfigPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(graniteYAML), 0600); err != nil { t.Fatal(err) }
	return p
}

func graniteEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GRANITE_API_KEY", "granite-secret")
	t.Setenv("INFRA_CONTROL_API_TOKEN", "infra-secret")
}

func TestLoadGraniteConfig(t *testing.T) {
	graniteEnv(t)
	cfg, err := Load(graniteConfigPath(t))
	if err != nil { t.Fatal(err) }
	if cfg.APIKey != "granite-secret" || cfg.Granite.ModelID != "granite-4.0-350m" || cfg.Granite.Threads != 4 || cfg.Granite.ContextSize != 4096 || cfg.Granite.MaxTokens != 256 || cfg.Granite.MaxMessageLength != 2048 || cfg.Granite.AllowForceStop || cfg.Granite.StartupTimeout != 120*time.Second || cfg.Granite.ActionDedupWindow != 30*time.Second || cfg.MaxBytes != 8<<20 { t.Fatalf("cfg=%+v", cfg) }
}

func TestLoadGraniteEnvironmentOverrides(t *testing.T) {
	graniteEnv(t)
	t.Setenv("GRANITE_THREADS", "8")
	t.Setenv("GRANITE_CONTEXT_SIZE", "8192")
	t.Setenv("GRANITE_MAX_TOKENS", "512")
	t.Setenv("GRANITE_ALLOW_FORCE_STOP", "true")
	cfg, err := Load(graniteConfigPath(t))
	if err != nil { t.Fatal(err) }
	if cfg.Granite.Threads != 8 || cfg.Granite.ContextSize != 8192 || cfg.Granite.MaxTokens != 512 || !cfg.Granite.AllowForceStop { t.Fatalf("cfg=%+v", cfg.Granite) }
}

func TestLoadRejectsInvalidGraniteConfigWithoutLeakingSecrets(t *testing.T) {
	tests := []struct{ key, value string }{
		{"GRANITE_API_KEY", ""}, {"INFRA_CONTROL_API_TOKEN", ""}, {"GRANITE_THREADS", "0"}, {"GRANITE_THREADS", "17"}, {"GRANITE_CONTEXT_SIZE", "1023"}, {"GRANITE_CONTEXT_SIZE", "8193"}, {"GRANITE_MAX_TOKENS", "0"}, {"GRANITE_MAX_MESSAGE_LENGTH", "0"}, {"GRANITE_STARTUP_TIMEOUT", "0s"}, {"GRANITE_ACTION_DEDUP_WINDOW", "0s"}, {"INFRA_CONTROL_API_BASE_URL", "http://user:pass@infra"},
	}
	for _, tc := range tests { t.Run(tc.key+tc.value, func(t *testing.T) {
		graniteEnv(t); t.Setenv(tc.key, tc.value)
		_, err := Load(graniteConfigPath(t))
		if err == nil || strings.Contains(err.Error(), "granite-secret") || strings.Contains(err.Error(), "infra-secret") { t.Fatalf("err=%v", err) }
	}) }
}

func TestNeedleEnvironmentDoesNotConfigureGranite(t *testing.T) {
	t.Setenv("NEEDLE_API_KEY", "legacy")
	t.Setenv("INFRA_CONTROL_API_TOKEN", "infra-secret")
	_, err := Load(graniteConfigPath(t))
	if err == nil || !strings.Contains(err.Error(), "GRANITE_API_KEY") { t.Fatalf("err=%v", err) }
}
