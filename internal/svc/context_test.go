//go:build !cgo

package svc

import (
	"needle-controller/internal/config"
	"needle-controller/internal/native"
	"testing"
	"time"
)

func TestNewServiceContextExposesFailedModelWhenNativeUnsupported(t *testing.T) {
	cfg := config.Config{APIKey: "api", Needle: config.NativeConfig{ModelID: "needle-2", MinConfidence: 0.6, MaxMessageLength: 512, MaxNewTokens: 256, MaxQueueDepth: 1, BufferSize: 65536, SlowCall: time.Second}, InfraControl: config.InfraControlConfig{BaseURL: "http://infra", APIToken: "token", Timeout: time.Second}}
	ctx := NewServiceContext(cfg)
	defer ctx.Dispatcher.Close()
	deadline := time.Now().Add(time.Second)
	for ctx.Managed.State() == native.StateLoading && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if ctx.Managed.State() != native.StateFailed {
		t.Fatalf("state=%s", ctx.Managed.State())
	}
}
