package granite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ProcessConfig struct {
	Binary, Model        string
	Threads, ContextSize int
	StartupTimeout       time.Duration
}
type Manager struct {
	cfg      ProcessConfig
	probe    func(context.Context) bool
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	ready    atomic.Bool
	stopping atomic.Bool
	exited   chan struct{}
	exitErr  error
	mu       sync.Mutex
	key      string
}

func NewManager(cfg ProcessConfig, probe func(context.Context) bool) *Manager {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		panic("cannot generate llama-server API key")
	}
	return &Manager{cfg: cfg, probe: probe, exited: make(chan struct{}), key: hex.EncodeToString(keyBytes)}
}
func (m *Manager) Start(ctx context.Context) error {
	processCtx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	args := []string{"--model", m.cfg.Model, "--host", "127.0.0.1", "--port", "18080", "--ctx-size", strconv.Itoa(m.cfg.ContextSize), "--parallel", "1", "--threads", strconv.Itoa(m.cfg.Threads), "--jinja", "--api-key", m.key}
	m.cmd = exec.CommandContext(processCtx, m.cfg.Binary, args...)
	if err := m.cmd.Start(); err != nil {
		cancel()
		return err
	}
	go func() {
		err := m.cmd.Wait()
		m.mu.Lock()
		m.exitErr = err
		m.mu.Unlock()
		close(m.exited)
	}()
	deadline := time.NewTimer(m.cfg.StartupTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.exited:
			cancel()
			return fmt.Errorf("llama-server exited: %w", m.ExitError())
		case <-deadline.C:
			cancel()
			return fmt.Errorf("llama-server startup timeout")
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-ticker.C:
			if m.probe(ctx) {
				m.ready.Store(true)
				return nil
			}
		}
	}
}
func (m *Manager) Ready() bool           { return m.ready.Load() }
func (m *Manager) Stopping() bool        { return m.stopping.Load() }
func (m *Manager) APIKey() string        { return m.key }
func (m *Manager) Wait() <-chan struct{} { return m.exited }
func (m *Manager) ExitError() error      { m.mu.Lock(); defer m.mu.Unlock(); return m.exitErr }
func (m *Manager) Stop() error {
	m.stopping.Store(true)
	m.ready.Store(false)
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
		<-m.exited
	}
	return nil
}
