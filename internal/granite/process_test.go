package granite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessArgumentsAndReadiness(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := filepath.Join(dir, "server")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$*\" > \"$ARGS_FILE\"\nsleep 5\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGS_FILE", argsFile)
	ready := false
	m := NewManager(ProcessConfig{Binary: script, Model: "/model.gguf", Threads: 4, ContextSize: 4096, StartupTimeout: time.Second}, func(context.Context) bool { ready = true; return true })
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	if !m.Ready() || !ready {
		t.Fatal("not ready")
	}
	data, _ := os.ReadFile(argsFile)
	got := string(data)
	for _, part := range []string{"--model /model.gguf", "--host 127.0.0.1", "--port 18080", "--ctx-size 4096", "--parallel 1", "--threads 4", "--jinja", "--api-key"} {
		if !strings.Contains(got, part) {
			t.Fatalf("args=%q", got)
		}
	}
}
func TestProcessStopMarksExpectedAndWaits(t *testing.T) {
	script := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0700); err != nil {
		t.Fatal(err)
	}
	m := NewManager(ProcessConfig{Binary: script, Model: "x", Threads: 1, ContextSize: 1024, StartupTimeout: time.Second}, func(context.Context) bool { return true })
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if !m.Stopping() {
		t.Fatal("stop was not marked expected")
	}
	select {
	case <-m.Wait():
	case <-time.After(time.Second):
		t.Fatal("child was not reaped")
	}
}

func TestProcessStartupTimeout(t *testing.T) {
	m := NewManager(ProcessConfig{Binary: "/bin/sleep", Model: "x", Threads: 1, ContextSize: 1024, StartupTimeout: 20 * time.Millisecond}, func(context.Context) bool { return false })
	if err := m.Start(context.Background()); err == nil {
		t.Fatal("want timeout")
	}
}
