package dedup

import (
	"context"
	"sync"
	"time"
)

type CachePolicy bool

const (
	DoNotCache CachePolicy = false
	Cache      CachePolicy = true
)

type Result struct {
	Value        any
	Err          error
	Deduplicated bool
}
type entry struct {
	done    chan struct{}
	value   any
	err     error
	expires time.Time
}
type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
	window  time.Duration
	now     func() time.Time
}

func New(window time.Duration, now func() time.Time) *Store {
	return &Store{entries: map[string]*entry{}, window: window, now: now}
}
func (s *Store) Do(ctx context.Context, key string, fn func(context.Context) (any, CachePolicy, error)) Result {
	s.mu.Lock()
	if old := s.entries[key]; old != nil {
		if old.expires.IsZero() || s.now().Before(old.expires) {
			s.mu.Unlock()
			select {
			case <-old.done:
				return Result{old.value, old.err, true}
			case <-ctx.Done():
				return Result{Err: ctx.Err(), Deduplicated: true}
			}
		}
		delete(s.entries, key)
	}
	current := &entry{done: make(chan struct{})}
	s.entries[key] = current
	s.mu.Unlock()
	value, policy, err := fn(ctx)
	s.mu.Lock()
	current.value, current.err = value, err
	if policy == Cache {
		current.expires = s.now().Add(s.window)
	} else {
		delete(s.entries, key)
	}
	close(current.done)
	s.mu.Unlock()
	return Result{Value: value, Err: err}
}
