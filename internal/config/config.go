package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

const (
	minBufferSize = 64 << 10
	maxBufferSize = 8 << 20
)

type Config struct {
	rest.RestConf
	APIKey string
	Needle NativeConfig
}

type NativeConfig struct {
	ModelID        string
	MaxNewTokens   int
	MaxQueueDepth  int
	MaxReplaySteps int
	BufferSize     int
	ToolIndexPath  string
	SlowCall       time.Duration
}

type fileConfig struct {
	rest.RestConf
	Needle struct {
		ModelID        string
		MaxNewTokens   int
		MaxQueueDepth  int
		MaxReplaySteps int
		BufferSize     int
		ToolIndexPath  string
		SlowCall       string
	}
}

func Load(path string) (Config, error) {
	var raw fileConfig
	if err := conf.Load(path, &raw); err != nil {
		return Config{}, fmt.Errorf("load configuration: %w", err)
	}
	maxNewTokens, err := envInt("NEEDLE_MAX_NEW_TOKENS", raw.Needle.MaxNewTokens)
	if err != nil {
		return Config{}, err
	}
	maxQueueDepth, err := envInt("NEEDLE_MAX_QUEUE_DEPTH", raw.Needle.MaxQueueDepth)
	if err != nil {
		return Config{}, err
	}
	maxReplaySteps, err := envInt("NEEDLE_MAX_REPLAY_STEPS", raw.Needle.MaxReplaySteps)
	if err != nil {
		return Config{}, err
	}
	bufferSize, err := envInt("NEEDLE_BUFFER_SIZE", raw.Needle.BufferSize)
	if err != nil {
		return Config{}, err
	}
	slowCall, err := envDuration("NEEDLE_ENGINE_SLOW_CALL", raw.Needle.SlowCall)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		RestConf: raw.RestConf,
		APIKey:   os.Getenv("NEEDLE_API_KEY"),
		Needle: NativeConfig{
			ModelID:        envOr("NEEDLE_MODEL_ID", raw.Needle.ModelID),
			MaxNewTokens:   maxNewTokens,
			MaxQueueDepth:  maxQueueDepth,
			MaxReplaySteps: maxReplaySteps,
			BufferSize:     bufferSize,
			ToolIndexPath:  envOr("NEEDLE_TOOL_INDEX_PATH", raw.Needle.ToolIndexPath),
			SlowCall:       slowCall,
		},
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("NEEDLE_API_KEY is required")
	}
	if strings.TrimSpace(c.Needle.ModelID) == "" {
		return fmt.Errorf("NEEDLE_MODEL_ID is required")
	}
	if c.Needle.MaxNewTokens <= 0 {
		return fmt.Errorf("NEEDLE_MAX_NEW_TOKENS must be positive")
	}
	if c.Needle.MaxQueueDepth <= 0 {
		return fmt.Errorf("NEEDLE_MAX_QUEUE_DEPTH must be positive")
	}
	if c.Needle.MaxReplaySteps <= 0 {
		return fmt.Errorf("NEEDLE_MAX_REPLAY_STEPS must be positive")
	}
	if c.Needle.BufferSize < minBufferSize || c.Needle.BufferSize > maxBufferSize {
		return fmt.Errorf("NEEDLE_BUFFER_SIZE must be between %d and %d", minBufferSize, maxBufferSize)
	}
	if c.Needle.SlowCall <= 0 {
		return fmt.Errorf("NEEDLE_ENGINE_SLOW_CALL must be positive")
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

func envDuration(name, fallback string) (time.Duration, error) {
	value := envOr(name, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid", name)
	}
	return parsed, nil
}
