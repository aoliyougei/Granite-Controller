package svc

import (
	"testing"
	"time"

	"needle-controller/internal/config"
)

func TestNewServiceContextBuildsProductionDependencies(t *testing.T) {
	cfg := config.Config{
		ControllerAPIToken: "controller-test-value",
		Needle:             config.NeedleConfig{BaseURL: "http://needle", Model: "needle-2", Timeout: time.Second, MinConfidence: 0.6, MaxTokens: 256, MaxMessageLength: 512},
		InfraControl:       config.InfraControlConfig{BaseURL: "http://infra", APIToken: "infra-test-value", Timeout: time.Second},
	}
	ctx := NewServiceContext(cfg)
	if ctx.Chat == nil || ctx.Needle == nil || ctx.InfraControl == nil || ctx.Config.ControllerAPIToken == "" {
		t.Fatalf("context=%+v", ctx)
	}
}
