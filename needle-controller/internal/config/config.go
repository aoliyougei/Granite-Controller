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
	ControllerAPIToken string
	Needle             NeedleConfig
	InfraControl       InfraControlConfig
}

type NeedleConfig struct {
	BaseURL          string
	APIKey           string
	Model            string
	Timeout          time.Duration
	MinConfidence    float64
	MaxTokens        int
	MaxMessageLength int
}

type InfraControlConfig struct {
	BaseURL  string
	APIToken string
	Timeout  time.Duration
}

type fileConfig struct {
	rest.RestConf
	Needle struct {
		BaseURL          string
		Model            string
		Timeout          string
		MinConfidence    float64
		MaxTokens        int
		MaxMessageLength int
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

	needleTimeout, err := durationValue("NEEDLE_TIMEOUT", raw.Needle.Timeout)
	if err != nil {
		return Config{}, err
	}
	infraTimeout, err := durationValue("INFRA_CONTROL_TIMEOUT", raw.InfraControl.Timeout)
	if err != nil {
		return Config{}, err
	}
	minConfidence, err := floatValue("NEEDLE_MIN_CONFIDENCE", raw.Needle.MinConfidence)
	if err != nil {
		return Config{}, err
	}
	maxTokens, err := intValue("NEEDLE_MAX_TOKENS", raw.Needle.MaxTokens)
	if err != nil {
		return Config{}, err
	}
	maxMessageLength, err := intValue("CONTROLLER_MAX_MESSAGE_LENGTH", raw.Needle.MaxMessageLength)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		RestConf:           raw.RestConf,
		ControllerAPIToken: os.Getenv("CONTROLLER_API_TOKEN"),
		Needle: NeedleConfig{
			BaseURL:          envOr("NEEDLE_BASE_URL", raw.Needle.BaseURL),
			APIKey:           os.Getenv("NEEDLE_API_KEY"),
			Model:            envOr("NEEDLE_MODEL_ID", raw.Needle.Model),
			Timeout:          needleTimeout,
			MinConfidence:    minConfidence,
			MaxTokens:        maxTokens,
			MaxMessageLength: maxMessageLength,
		},
		InfraControl: InfraControlConfig{
			BaseURL:  envOr("INFRA_CONTROL_BASE_URL", raw.InfraControl.BaseURL),
			APIToken: os.Getenv("INFRA_CONTROL_API_TOKEN"),
			Timeout:  infraTimeout,
		},
	}
	cfg.Needle.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Needle.BaseURL), "/")
	cfg.InfraControl.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.InfraControl.BaseURL), "/")
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.ControllerAPIToken == "" {
		return fmt.Errorf("CONTROLLER_API_TOKEN is required")
	}
	if c.InfraControl.APIToken == "" {
		return fmt.Errorf("INFRA_CONTROL_API_TOKEN is required")
	}
	if err := validBaseURL("NEEDLE_BASE_URL", c.Needle.BaseURL); err != nil {
		return err
	}
	if err := validBaseURL("INFRA_CONTROL_BASE_URL", c.InfraControl.BaseURL); err != nil {
		return err
	}
	if strings.TrimSpace(c.Needle.Model) == "" {
		return fmt.Errorf("NEEDLE_MODEL_ID is required")
	}
	if c.Needle.Timeout <= 0 {
		return fmt.Errorf("NEEDLE_TIMEOUT must be positive")
	}
	if c.InfraControl.Timeout <= 0 {
		return fmt.Errorf("INFRA_CONTROL_TIMEOUT must be positive")
	}
	if c.Needle.MinConfidence < 0 || c.Needle.MinConfidence > 1 {
		return fmt.Errorf("NEEDLE_MIN_CONFIDENCE must be between 0 and 1")
	}
	if c.Needle.MaxTokens <= 0 {
		return fmt.Errorf("NEEDLE_MAX_TOKENS must be positive")
	}
	if c.Needle.MaxMessageLength <= 0 {
		return fmt.Errorf("CONTROLLER_MAX_MESSAGE_LENGTH must be positive")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func durationValue(name, fallback string) (time.Duration, error) {
	value := envOr(name, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}

func floatValue(name string, fallback float64) (float64, error) {
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

func intValue(name string, fallback int) (int, error) {
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

func validBaseURL(name, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an http(s) origin without credentials, query, or fragment", name)
	}
	return nil
}
