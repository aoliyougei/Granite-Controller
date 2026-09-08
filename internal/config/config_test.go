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
  MinConfidence: 0.6
  MaxMessageLength: 512
  MaxNewTokens: 256
  MaxQueueDepth: 32
  BufferSize: 1048576
  ToolIndexPath: ""
  SlowCall: 30s
InfraControl:
  BaseURL: http://infrastructure-control:8080
  Timeout: 30s
`

func configPath(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if os.WriteFile(path, []byte(body), 0600) != nil {
		t.Fatal("write")
	}
	return path
}
func validEnv(t *testing.T) {
	t.Helper()
	t.Setenv("NEEDLE_API_KEY", "needle-test-value")
	t.Setenv("INFRA_CONTROL_API_TOKEN", "infra-test-value")
}

func TestLoadManagedDefaultsAndExactEnvironmentNames(t *testing.T) {
	validEnv(t)
	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Needle.MinConfidence != 0.6 || cfg.Needle.MaxMessageLength != 512 || cfg.InfraControl.BaseURL != "http://infrastructure-control:8080" || cfg.InfraControl.APIToken != "infra-test-value" || cfg.InfraControl.Timeout.String() != "30s" {
		t.Fatalf("config=%+v", cfg)
	}
}
func TestLoadManagedOverrides(t *testing.T) {
	validEnv(t)
	t.Setenv("NEEDLE_MIN_CONFIDENCE", "0.8")
	t.Setenv("NEEDLE_MAX_MESSAGE_LENGTH", "300")
	t.Setenv("INFRA_CONTROL_API_BASE_URL", "https://infra.example///")
	t.Setenv("INFRA_CONTROL_TIMEOUT", "10s")
	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Needle.MinConfidence != 0.8 || cfg.Needle.MaxMessageLength != 300 || cfg.InfraControl.BaseURL != "https://infra.example" || cfg.InfraControl.Timeout.String() != "10s" {
		t.Fatalf("config=%+v", cfg)
	}
}
func TestLoadRejectsInvalidManagedConfiguration(t *testing.T) {
	tests := []struct{ name, key, value, want string }{
		{"missing needle key", "NEEDLE_API_KEY", "", "NEEDLE_API_KEY"}, {"missing infra token", "INFRA_CONTROL_API_TOKEN", "", "INFRA_CONTROL_API_TOKEN"}, {"empty base", "INFRA_CONTROL_API_BASE_URL", "", "INFRA_CONTROL_API_BASE_URL"}, {"scheme", "INFRA_CONTROL_API_BASE_URL", "ftp://infra", "INFRA_CONTROL_API_BASE_URL"}, {"userinfo", "INFRA_CONTROL_API_BASE_URL", "http://u:p@infra", "INFRA_CONTROL_API_BASE_URL"}, {"path", "INFRA_CONTROL_API_BASE_URL", "http://infra/api", "INFRA_CONTROL_API_BASE_URL"}, {"query", "INFRA_CONTROL_API_BASE_URL", "http://infra?q=x", "INFRA_CONTROL_API_BASE_URL"}, {"confidence low", "NEEDLE_MIN_CONFIDENCE", "-0.1", "NEEDLE_MIN_CONFIDENCE"}, {"confidence high", "NEEDLE_MIN_CONFIDENCE", "1.1", "NEEDLE_MIN_CONFIDENCE"}, {"confidence nan", "NEEDLE_MIN_CONFIDENCE", "NaN", "NEEDLE_MIN_CONFIDENCE"}, {"message zero", "NEEDLE_MAX_MESSAGE_LENGTH", "0", "NEEDLE_MAX_MESSAGE_LENGTH"}, {"timeout zero", "INFRA_CONTROL_TIMEOUT", "0s", "INFRA_CONTROL_TIMEOUT"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			t.Setenv(tc.key, tc.value)
			_, err := Load(configPath(t, validYAML))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
			if strings.Contains(err.Error(), "test-value") {
				t.Fatalf("secret leaked: %v", err)
			}
		})
	}
}
func TestLegacyInfraBaseURLDoesNotSatisfyRequiredName(t *testing.T) {
	validEnv(t)
	t.Setenv("INFRA_CONTROL_BASE_URL", "http://legacy")
	yaml := strings.Replace(validYAML, "  BaseURL: http://infrastructure-control:8080", "  BaseURL: \"\"", 1)
	_, err := Load(configPath(t, yaml))
	if err == nil || !strings.Contains(err.Error(), "INFRA_CONTROL_API_BASE_URL") {
		t.Fatalf("error=%v", err)
	}
}
