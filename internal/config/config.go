package config

import (
	"fmt"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	minBufferSize = 64 << 10
	maxBufferSize = 8 << 20
)

type Config struct {
	rest.RestConf
	APIKey       string
	Needle       NativeConfig
	InfraControl InfraControlConfig
}
type NativeConfig struct {
	ModelID          string
	MinConfidence    float64
	MaxMessageLength int
	MaxNewTokens     int
	MaxQueueDepth    int
	MaxReplaySteps   int
	BufferSize       int
	ToolIndexPath    string
	SlowCall         time.Duration
}
type InfraControlConfig struct {
	BaseURL  string
	APIToken string
	Timeout  time.Duration
}
type fileConfig struct {
	rest.RestConf
	Needle struct {
		ModelID          string
		MinConfidence    float64
		MaxMessageLength int
		MaxNewTokens     int
		MaxQueueDepth    int
		BufferSize       int
		ToolIndexPath    string
		SlowCall         string
	}
	InfraControl struct {
		BaseURL string
		Timeout string
	}
}

func Load(path string) (Config, error) {
	var raw fileConfig
	if err := conf.Load(path, &raw); err != nil {
		return Config{}, fmt.Errorf("load configuration: %w", err)
	}
	maxTokens, err := envInt("NEEDLE_MAX_NEW_TOKENS", raw.Needle.MaxNewTokens)
	if err != nil {
		return Config{}, err
	}
	queue, err := envInt("NEEDLE_MAX_QUEUE_DEPTH", raw.Needle.MaxQueueDepth)
	if err != nil {
		return Config{}, err
	}
	buffer, err := envInt("NEEDLE_BUFFER_SIZE", raw.Needle.BufferSize)
	if err != nil {
		return Config{}, err
	}
	message, err := envInt("NEEDLE_MAX_MESSAGE_LENGTH", raw.Needle.MaxMessageLength)
	if err != nil {
		return Config{}, err
	}
	confidence, err := envFloat("NEEDLE_MIN_CONFIDENCE", raw.Needle.MinConfidence)
	if err != nil {
		return Config{}, err
	}
	slow, err := envDuration("NEEDLE_ENGINE_SLOW_CALL", raw.Needle.SlowCall)
	if err != nil {
		return Config{}, err
	}
	upstreamTimeout, err := envDuration("INFRA_CONTROL_TIMEOUT", raw.InfraControl.Timeout)
	if err != nil {
		return Config{}, err
	}
	base := strings.TrimRight(strings.TrimSpace(envOr("INFRA_CONTROL_API_BASE_URL", raw.InfraControl.BaseURL)), "/")
	cfg := Config{RestConf: raw.RestConf, APIKey: os.Getenv("NEEDLE_API_KEY"), Needle: NativeConfig{ModelID: envOr("NEEDLE_MODEL_ID", raw.Needle.ModelID), MinConfidence: confidence, MaxMessageLength: message, MaxNewTokens: maxTokens, MaxQueueDepth: queue, MaxReplaySteps: 1, BufferSize: buffer, ToolIndexPath: envOr("NEEDLE_TOOL_INDEX_PATH", raw.Needle.ToolIndexPath), SlowCall: slow}, InfraControl: InfraControlConfig{BaseURL: base, APIToken: os.Getenv("INFRA_CONTROL_API_TOKEN"), Timeout: upstreamTimeout}}
	return cfg, cfg.Validate()
}
func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("NEEDLE_API_KEY is required")
	}
	if c.InfraControl.APIToken == "" {
		return fmt.Errorf("INFRA_CONTROL_API_TOKEN is required")
	}
	if err := validOrigin(c.InfraControl.BaseURL); err != nil {
		return fmt.Errorf("INFRA_CONTROL_API_BASE_URL is invalid")
	}
	if strings.TrimSpace(c.Needle.ModelID) == "" {
		return fmt.Errorf("NEEDLE_MODEL_ID is required")
	}
	if math.IsNaN(c.Needle.MinConfidence) || math.IsInf(c.Needle.MinConfidence, 0) || c.Needle.MinConfidence < 0 || c.Needle.MinConfidence > 1 {
		return fmt.Errorf("NEEDLE_MIN_CONFIDENCE must be between 0 and 1")
	}
	if c.Needle.MaxMessageLength <= 0 {
		return fmt.Errorf("NEEDLE_MAX_MESSAGE_LENGTH must be positive")
	}
	if c.Needle.MaxNewTokens <= 0 {
		return fmt.Errorf("NEEDLE_MAX_NEW_TOKENS must be positive")
	}
	if c.Needle.MaxQueueDepth <= 0 {
		return fmt.Errorf("NEEDLE_MAX_QUEUE_DEPTH must be positive")
	}
	if c.Needle.BufferSize < minBufferSize || c.Needle.BufferSize > maxBufferSize {
		return fmt.Errorf("NEEDLE_BUFFER_SIZE is outside allowed range")
	}
	if c.Needle.SlowCall <= 0 {
		return fmt.Errorf("NEEDLE_ENGINE_SLOW_CALL must be positive")
	}
	if c.InfraControl.Timeout <= 0 {
		return fmt.Errorf("INFRA_CONTROL_TIMEOUT must be positive")
	}
	return nil
}
func validOrigin(value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid origin")
	}
	return nil
}
func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
func envInt(name string, fallback int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}
func envFloat(name string, fallback float64) (float64, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}
func envDuration(name, fallback string) (time.Duration, error) {
	value := envOr(name, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}
