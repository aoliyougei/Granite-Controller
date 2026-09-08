package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
Name: needle-controller
Host: 0.0.0.0
Port: 8080
Timeout: 130000
Needle:
  ModelID: needle-2
  MaxNewTokens: 256
  MaxQueueDepth: 32
  MaxReplaySteps: 32
  BufferSize: 1048576
  ToolIndexPath: ""
  SlowCall: 30s
`

func configPath(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadNativeDefaults(t *testing.T) {
	t.Setenv("NEEDLE_API_KEY", "api-test-value")
	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "api-test-value" || cfg.Needle.ModelID != "needle-2" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Needle.MaxNewTokens != 256 || cfg.Needle.MaxQueueDepth != 32 || cfg.Needle.MaxReplaySteps != 32 {
		t.Fatalf("limits = %+v", cfg.Needle)
	}
	if cfg.Needle.BufferSize != 1048576 || cfg.Needle.SlowCall.String() != "30s" {
		t.Fatalf("native config = %+v", cfg.Needle)
	}
}

func TestLoadAppliesNativeEnvironmentOverrides(t *testing.T) {
	t.Setenv("NEEDLE_API_KEY", "api-test-value")
	t.Setenv("NEEDLE_MODEL_ID", "needle-custom")
	t.Setenv("NEEDLE_MAX_NEW_TOKENS", "128")
	t.Setenv("NEEDLE_MAX_QUEUE_DEPTH", "7")
	t.Setenv("NEEDLE_MAX_REPLAY_STEPS", "9")
	t.Setenv("NEEDLE_BUFFER_SIZE", "65536")
	t.Setenv("NEEDLE_TOOL_INDEX_PATH", "/tmp/needle-index")
	t.Setenv("NEEDLE_ENGINE_SLOW_CALL", "5s")
	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Needle.ModelID != "needle-custom" || cfg.Needle.MaxNewTokens != 128 || cfg.Needle.MaxQueueDepth != 7 || cfg.Needle.MaxReplaySteps != 9 || cfg.Needle.BufferSize != 65536 || cfg.Needle.ToolIndexPath != "/tmp/needle-index" || cfg.Needle.SlowCall.String() != "5s" {
		t.Fatalf("overrides = %+v", cfg.Needle)
	}
}

func TestLoadRejectsInvalidNativeConfiguration(t *testing.T) {
	tests := []struct {
		name, key, value, want string
	}{
		{"missing api key", "NEEDLE_API_KEY", "", "NEEDLE_API_KEY"},
		{"empty model", "NEEDLE_MODEL_ID", " ", "NEEDLE_MODEL_ID"},
		{"zero tokens", "NEEDLE_MAX_NEW_TOKENS", "0", "NEEDLE_MAX_NEW_TOKENS"},
		{"zero queue", "NEEDLE_MAX_QUEUE_DEPTH", "0", "NEEDLE_MAX_QUEUE_DEPTH"},
		{"zero replay", "NEEDLE_MAX_REPLAY_STEPS", "0", "NEEDLE_MAX_REPLAY_STEPS"},
		{"small buffer", "NEEDLE_BUFFER_SIZE", "65535", "NEEDLE_BUFFER_SIZE"},
		{"large buffer", "NEEDLE_BUFFER_SIZE", "8388609", "NEEDLE_BUFFER_SIZE"},
		{"zero slow call", "NEEDLE_ENGINE_SLOW_CALL", "0s", "NEEDLE_ENGINE_SLOW_CALL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NEEDLE_API_KEY", "api-test-value")
			t.Setenv(tc.key, tc.value)
			_, err := Load(configPath(t, validYAML))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %s", err, tc.want)
			}
			if strings.Contains(err.Error(), "api-test-value") {
				t.Fatalf("error leaked key: %v", err)
			}
		})
	}
}

func TestRemovedEnvironmentVariablesDoNotSatisfyAPIKey(t *testing.T) {
	for _, key := range []string{"CONTROLLER_API_TOKEN", "NEEDLE_CONTROLLER_API_TOKEN", "NEEDLE_OPENAI_API_KEY", "INFRA_CONTROL_API_TOKEN"} {
		t.Setenv(key, "legacy-test-value")
	}
	_, err := Load(configPath(t, validYAML))
	if err == nil || !strings.Contains(err.Error(), "NEEDLE_API_KEY") {
		t.Fatalf("error = %v", err)
	}
}
