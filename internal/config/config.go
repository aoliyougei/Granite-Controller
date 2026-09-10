package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	APIKey       string
	Granite      GraniteConfig
	InfraControl InfraControlConfig
}

type GraniteConfig struct {
	ModelID          string
	StartupTimeout   time.Duration
	RequestTimeout   time.Duration
	Threads          int
	ContextSize      int
	MaxTokens        int
	MaxMessageLength int
	AllowForceStop   bool
	ActionDedupWindow time.Duration
}

type InfraControlConfig struct {
	BaseURL  string
	APIToken string
	Timeout  time.Duration
}

type fileConfig struct {
	rest.RestConf
	Granite struct {
		ModelID, StartupTimeout, RequestTimeout, ActionDedupWindow string
		Threads, ContextSize, MaxTokens, MaxMessageLength int
		AllowForceStop bool
	}
	InfraControl struct { BaseURL, Timeout string }
}

func Load(path string) (Config, error) {
	var raw fileConfig
	if err := conf.Load(path, &raw); err != nil { return Config{}, fmt.Errorf("load configuration: %w", err) }
	threads, err := envInt("GRANITE_THREADS", raw.Granite.Threads); if err != nil { return Config{}, err }
	contextSize, err := envInt("GRANITE_CONTEXT_SIZE", raw.Granite.ContextSize); if err != nil { return Config{}, err }
	maxTokens, err := envInt("GRANITE_MAX_TOKENS", raw.Granite.MaxTokens); if err != nil { return Config{}, err }
	maxMessage, err := envInt("GRANITE_MAX_MESSAGE_LENGTH", raw.Granite.MaxMessageLength); if err != nil { return Config{}, err }
	startup, err := envDuration("GRANITE_STARTUP_TIMEOUT", raw.Granite.StartupTimeout); if err != nil { return Config{}, err }
	request, err := envDuration("GRANITE_REQUEST_TIMEOUT", raw.Granite.RequestTimeout); if err != nil { return Config{}, err }
	dedup, err := envDuration("GRANITE_ACTION_DEDUP_WINDOW", raw.Granite.ActionDedupWindow); if err != nil { return Config{}, err }
	upstream, err := envDuration("INFRA_CONTROL_TIMEOUT", raw.InfraControl.Timeout); if err != nil { return Config{}, err }
	allowStop, err := envBool("GRANITE_ALLOW_FORCE_STOP", raw.Granite.AllowForceStop); if err != nil { return Config{}, err }
	cfg := Config{
		RestConf: raw.RestConf,
		APIKey: os.Getenv("GRANITE_API_KEY"),
		Granite: GraniteConfig{ModelID: envOr("GRANITE_MODEL_ID", raw.Granite.ModelID), StartupTimeout: startup, RequestTimeout: request, Threads: threads, ContextSize: contextSize, MaxTokens: maxTokens, MaxMessageLength: maxMessage, AllowForceStop: allowStop, ActionDedupWindow: dedup},
		InfraControl: InfraControlConfig{BaseURL: strings.TrimRight(strings.TrimSpace(envOr("INFRA_CONTROL_API_BASE_URL", raw.InfraControl.BaseURL)), "/"), APIToken: os.Getenv("INFRA_CONTROL_API_TOKEN"), Timeout: upstream},
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	checks := []struct{ bad bool; message string }{
		{c.APIKey == "", "GRANITE_API_KEY is required"},
		{c.InfraControl.APIToken == "", "INFRA_CONTROL_API_TOKEN is required"},
		{strings.TrimSpace(c.Granite.ModelID) == "", "GRANITE_MODEL_ID is required"},
		{c.Granite.Threads < 1 || c.Granite.Threads > 16, "GRANITE_THREADS must be between 1 and 16"},
		{c.Granite.ContextSize < 1024 || c.Granite.ContextSize > 8192, "GRANITE_CONTEXT_SIZE must be between 1024 and 8192"},
		{c.Granite.MaxTokens < 1 || c.Granite.MaxTokens > 512, "GRANITE_MAX_TOKENS must be between 1 and 512"},
		{c.Granite.MaxMessageLength < 1, "GRANITE_MAX_MESSAGE_LENGTH must be positive"},
		{c.Granite.StartupTimeout <= 0, "GRANITE_STARTUP_TIMEOUT must be positive"},
		{c.Granite.RequestTimeout <= 0, "GRANITE_REQUEST_TIMEOUT must be positive"},
		{c.Granite.ActionDedupWindow <= 0, "GRANITE_ACTION_DEDUP_WINDOW must be positive"},
		{c.InfraControl.Timeout <= 0, "INFRA_CONTROL_TIMEOUT must be positive"},
	}
	for _, check := range checks { if check.bad { return fmt.Errorf("%s", check.message) } }
	if err := validOrigin(c.InfraControl.BaseURL); err != nil { return fmt.Errorf("INFRA_CONTROL_API_BASE_URL is invalid") }
	return nil
}

func validOrigin(value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" { return fmt.Errorf("invalid origin") }
	return nil
}
func envOr(name, fallback string) string { if value, ok := os.LookupEnv(name); ok { return value }; return fallback }
func envInt(name string, fallback int) (int, error) { value, ok := os.LookupEnv(name); if !ok { return fallback, nil }; parsed, err := strconv.Atoi(value); if err != nil { return 0, fmt.Errorf("%s is invalid", name) }; return parsed, nil }
func envBool(name string, fallback bool) (bool, error) { value, ok := os.LookupEnv(name); if !ok { return fallback, nil }; parsed, err := strconv.ParseBool(value); if err != nil { return false, fmt.Errorf("%s is invalid", name) }; return parsed, nil }
func envDuration(name, fallback string) (time.Duration, error) { value := envOr(name, fallback); parsed, err := time.ParseDuration(value); if err != nil { return 0, fmt.Errorf("%s is invalid", name) }; return parsed, nil }
