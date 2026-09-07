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
  BaseURL: http://needle-openai:8000
  Model: needle-2
  Timeout: 2m
  MinConfidence: 0.6
  MaxTokens: 256
  MaxMessageLength: 512
InfraControl:
  BaseURL: http://infrastructure-control:8080
  Timeout: 30s
`

func configPath(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("CONTROLLER_API_TOKEN", "controller-test-value")
	t.Setenv("INFRA_CONTROL_API_TOKEN", "infra-test-value")
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	validEnvironment(t)
	t.Setenv("NEEDLE_BASE_URL", "http://needle.example:8000///")
	t.Setenv("NEEDLE_API_KEY", "needle-test-value")
	t.Setenv("NEEDLE_MODEL_ID", "custom-needle")
	t.Setenv("NEEDLE_MIN_CONFIDENCE", "0.75")
	t.Setenv("NEEDLE_TIMEOUT", "45s")
	t.Setenv("NEEDLE_MAX_TOKENS", "128")
	t.Setenv("CONTROLLER_MAX_MESSAGE_LENGTH", "300")
	t.Setenv("INFRA_CONTROL_BASE_URL", "https://infra.example/")
	t.Setenv("INFRA_CONTROL_TIMEOUT", "10s")

	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Timeout != 130000 {
		t.Errorf("REST Timeout = %d, want 130000 milliseconds", cfg.Timeout)
	}
	if cfg.Needle.BaseURL != "http://needle.example:8000" {
		t.Errorf("Needle.BaseURL = %q", cfg.Needle.BaseURL)
	}
	if cfg.Needle.APIKey != "needle-test-value" || cfg.Needle.Model != "custom-needle" {
		t.Errorf("Needle identity overrides not applied: %+v", cfg.Needle)
	}
	if cfg.Needle.MinConfidence != 0.75 || cfg.Needle.Timeout.String() != "45s" {
		t.Errorf("Needle numeric overrides not applied: %+v", cfg.Needle)
	}
	if cfg.Needle.MaxTokens != 128 || cfg.Needle.MaxMessageLength != 300 {
		t.Errorf("Needle limits not applied: %+v", cfg.Needle)
	}
	if cfg.InfraControl.BaseURL != "https://infra.example" || cfg.InfraControl.Timeout.String() != "10s" {
		t.Errorf("InfraControl overrides not applied: %+v", cfg.InfraControl)
	}
}

func TestLoadAllowsEmptyNeedleAPIKey(t *testing.T) {
	validEnvironment(t)
	cfg, err := Load(configPath(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Needle.APIKey != "" {
		t.Fatalf("Needle.APIKey = %q, want empty", cfg.Needle.APIKey)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "missing controller token", env: map[string]string{"CONTROLLER_API_TOKEN": "", "INFRA_CONTROL_API_TOKEN": "infra"}, wantErr: "CONTROLLER_API_TOKEN"},
		{name: "missing infra token", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": ""}, wantErr: "INFRA_CONTROL_API_TOKEN"},
		{name: "bad needle url", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_BASE_URL": "ftp://needle"}, wantErr: "NEEDLE_BASE_URL"},
		{name: "url userinfo", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_BASE_URL": "http://user:pass@needle"}, wantErr: "NEEDLE_BASE_URL"},
		{name: "url query", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "INFRA_CONTROL_BASE_URL": "http://infra?q=secret"}, wantErr: "INFRA_CONTROL_BASE_URL"},
		{name: "url path", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "INFRA_CONTROL_BASE_URL": "http://infra/api"}, wantErr: "INFRA_CONTROL_BASE_URL"},
		{name: "bad confidence", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_MIN_CONFIDENCE": "1.1"}, wantErr: "NEEDLE_MIN_CONFIDENCE"},
		{name: "nan confidence", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_MIN_CONFIDENCE": "NaN"}, wantErr: "NEEDLE_MIN_CONFIDENCE"},
		{name: "zero timeout", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_TIMEOUT": "0s"}, wantErr: "NEEDLE_TIMEOUT"},
		{name: "zero tokens", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "NEEDLE_MAX_TOKENS": "0"}, wantErr: "NEEDLE_MAX_TOKENS"},
		{name: "zero message limit", env: map[string]string{"CONTROLLER_API_TOKEN": "controller", "INFRA_CONTROL_API_TOKEN": "infra", "CONTROLLER_MAX_MESSAGE_LENGTH": "0"}, wantErr: "CONTROLLER_MAX_MESSAGE_LENGTH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			_, err := Load(configPath(t, validYAML))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want field %s", err, tt.wantErr)
			}
			for _, secret := range []string{"controller", "infra", "pass"} {
				if err != nil && strings.Contains(err.Error(), secret+"-test-value") {
					t.Fatalf("error leaked secret: %v", err)
				}
			}
		})
	}
}
