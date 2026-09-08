//go:build !cgo

package svc

import (
	"needle-controller/internal/config"
	"needle-controller/internal/native"
	"testing"
	"time"
)

func TestNewServiceContextExposesFailedModelWhenNativeUnsupported(t *testing.T) {
	ctx := NewServiceContext(config.Config{APIKey: "api", Needle: config.NativeConfig{ModelID: "needle-2", MaxNewTokens: 256, MaxQueueDepth: 1, MaxReplaySteps: 32, BufferSize: 65536, SlowCall: time.Second}})
	defer ctx.Dispatcher.Close()
	deadline := time.Now().Add(time.Second)
	for ctx.OpenAI.State() == native.StateLoading && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if ctx.OpenAI.State() != native.StateFailed {
		t.Fatalf("state=%s", ctx.OpenAI.State())
	}
}
