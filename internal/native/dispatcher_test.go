package native

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeExecutor struct {
	mu        sync.Mutex
	calls     []Request
	started   chan struct{}
	release   chan struct{}
	err       error
	active    atomic.Int32
	maxActive atomic.Int32
}

func (f *fakeExecutor) Execute(r Request) (Envelope, error) {
	current := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		old := f.maxActive.Load()
		if current <= old || f.maxActive.CompareAndSwap(old, current) {
			break
		}
	}
	f.mu.Lock()
	f.calls = append(f.calls, r)
	f.mu.Unlock()
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		<-f.release
	}
	return Envelope{Type: "respond"}, f.err
}
func (f *fakeExecutor) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.calls) }

func probeRequest() Request {
	return Request{ToolsJSON: []byte(`[{"name":"probe"}]`), ToolNames: []string{"probe"}, Turns: []Turn{{Kind: TurnUser, Text: "probe"}}, MaxNewTokens: 1}
}
func waitState(t *testing.T, d *Dispatcher, want State) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if d.State() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state=%s want=%s failure=%s", d.State(), want, d.FailureSummary())
}

func TestDispatcherBackgroundLifecycle(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		f := &fakeExecutor{}
		d := NewDispatcher(func() (Executor, error) { return f, nil }, probeRequest(), 2)
		defer d.Close()
		if d.State() != StateLoading {
			t.Fatalf("initial=%s", d.State())
		}
		waitState(t, d, StateReady)
		if f.count() != 1 {
			t.Fatalf("probe calls=%d", f.count())
		}
	})
	t.Run("failed once", func(t *testing.T) {
		var factories atomic.Int32
		d := NewDispatcher(func() (Executor, error) { factories.Add(1); return nil, errors.New("private path /x") }, probeRequest(), 2)
		defer d.Close()
		waitState(t, d, StateFailed)
		time.Sleep(10 * time.Millisecond)
		if factories.Load() != 1 {
			t.Fatalf("factories=%d", factories.Load())
		}
		if d.FailureSummary() == "" || d.FailureSummary() == "private path /x" {
			t.Fatalf("summary=%q", d.FailureSummary())
		}
	})
}

func TestDispatcherSerializesQueuesAndCancels(t *testing.T) {
	f := &fakeExecutor{started: make(chan struct{}, 8), release: make(chan struct{})}
	d := NewDispatcher(func() (Executor, error) { return f, nil }, probeRequest(), 1)
	<-f.started
	f.release <- struct{}{}
	waitState(t, d, StateReady)

	firstDone := make(chan error, 1)
	go func() { _, err := d.Submit(context.Background(), Request{System: "first"}); firstDone <- err }()
	<-f.started
	secondDone := make(chan error, 1)
	go func() { _, err := d.Submit(context.Background(), Request{System: "second"}); secondDone <- err }()
	deadline := time.Now().Add(time.Second)
	for d.QueueDepth() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if _, err := d.Submit(context.Background(), Request{System: "overflow"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("overflow err=%v", err)
	}
	f.release <- struct{}{}
	<-f.started
	f.release <- struct{}{}
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	if f.maxActive.Load() != 1 {
		t.Fatalf("native overlap=%d", f.maxActive.Load())
	}

	ctx, cancel := context.WithCancel(context.Background())
	f.release = make(chan struct{})
	running := make(chan error, 1)
	go func() { _, err := d.Submit(context.Background(), Request{System: "blocking"}); running <- err }()
	<-f.started
	canceled := make(chan error, 1)
	go func() { _, err := d.Submit(ctx, Request{System: "canceled"}); canceled <- err }()
	cancel()
	if err := <-canceled; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
	f.release <- struct{}{}
	<-running
	time.Sleep(10 * time.Millisecond)
	if f.count() != 4 {
		t.Fatalf("calls=%d; canceled request executed", f.count())
	}
	d.Close()
}

func TestDispatcherCloseRejectsNewSubmissions(t *testing.T) {
	f := &fakeExecutor{}
	d := NewDispatcher(func() (Executor, error) { return f, nil }, probeRequest(), 1)
	waitState(t, d, StateReady)
	d.Close()
	done := make(chan error, 1)
	go func() { _, err := d.Submit(context.Background(), Request{}); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrClosed) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Submit blocked after Close")
	}
}

func TestDispatcherFatalExecutionMarksFailed(t *testing.T) {
	f := &fakeExecutor{}
	d := NewDispatcher(func() (Executor, error) { return f, nil }, probeRequest(), 1)
	defer d.Close()
	waitState(t, d, StateReady)
	f.err = &NativeError{Code: "native_init_failed", Message: "bad", Fatal: true}
	if _, err := d.Submit(context.Background(), Request{}); err == nil {
		t.Fatal("expected error")
	}
	waitState(t, d, StateFailed)
	if _, err := d.Submit(context.Background(), Request{}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("err=%v", err)
	}
}
