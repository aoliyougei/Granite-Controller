package dedup

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentDuplicatesExecuteOnce(t *testing.T) {
	now := time.Unix(1,0)
	store := New(30*time.Second, func() time.Time { return now })
	var calls atomic.Int32
	start := make(chan struct{})
	fn := func(context.Context) (any, CachePolicy, error) { calls.Add(1); <-start; return "ok", Cache, nil }
	var wg sync.WaitGroup
	results := make([]Result,2)
	for i := range results { wg.Add(1); go func(i int) { defer wg.Done(); results[i] = store.Do(context.Background(), "start:3052", fn) }(i) }
	for calls.Load() != 1 { time.Sleep(time.Millisecond) }
	close(start); wg.Wait()
	if calls.Load()!=1 || results[0].Value!="ok" || results[1].Value!="ok" || results[0].Deduplicated==results[1].Deduplicated { t.Fatalf("calls=%d results=%+v", calls.Load(), results) }
}

func TestCachePolicyAndExpiry(t *testing.T) {
	now := time.Unix(1,0); store := New(30*time.Second, func() time.Time{return now}); calls:=0
	fn := func(context.Context)(any,CachePolicy,error){calls++; return calls, Cache,nil}
	if store.Do(context.Background(),"k",fn).Value != 1 || store.Do(context.Background(),"k",fn).Value != 1 { t.Fatal("not cached") }
	now=now.Add(31*time.Second)
	if store.Do(context.Background(),"k",fn).Value != 2 { t.Fatal("not expired") }
	noCache := func(context.Context)(any,CachePolicy,error){calls++; return nil, DoNotCache, errors.New("known")}
	store.Do(context.Background(),"e",noCache); store.Do(context.Background(),"e",noCache)
	if calls != 4 { t.Fatalf("calls=%d",calls) }
}
