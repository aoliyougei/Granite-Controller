package infracontrol

import (
	"context"
	"needle-controller/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartVMSendsExactlyOneFixedAuthenticatedRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/v1/pve/vms/3052/start" || r.Header.Get("Authorization") != "Bearer infra-test-value" || r.Header.Get("Accept") != "application/json" || r.Header.Get("X-Request-ID") != "req-1" {
			t.Fatalf("request=%s %s headers=%v", r.Method, r.URL.Path, r.Header)
		}
		w.Header().Set("X-Request-ID", "upstream-1")
		w.WriteHeader(202)
	}))
	defer server.Close()
	client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "infra-test-value", Timeout: time.Second})
	got, err := client.StartVM(context.Background(), "req-1", 3052)
	if err != nil || got.Status != 202 || got.UpstreamRequestID != "upstream-1" || calls.Load() != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", got, err, calls.Load())
	}
}
func TestStartVMNeverRetriesFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		delay  time.Duration
		code   string
	}{{"ok not accepted", 200, 0, "upstream_request_failed"}, {"bad", 400, 0, "upstream_request_failed"}, {"auth", 401, 0, "upstream_auth_failed"}, {"forbidden", 403, 0, "upstream_auth_failed"}, {"rate", 429, 0, "upstream_request_failed"}, {"server", 500, 0, "upstream_request_failed"}, {"timeout", 202, 100 * time.Millisecond, "upstream_timeout"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				time.Sleep(tc.delay)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(strings.Repeat("x", responseLimit+1)))
			}))
			defer server.Close()
			timeout := time.Second
			if tc.delay > 0 {
				timeout = 10 * time.Millisecond
			}
			client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "token", Timeout: timeout})
			_, err := client.StartVM(context.Background(), "req", 3052)
			if err == nil || err.Code != tc.code || calls.Load() != 1 {
				t.Fatalf("error=%+v calls=%d", err, calls.Load())
			}
		})
	}
}
func TestStartVMRejectsRedirectWithoutFollowing(t *testing.T) {
	var destination atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { destination.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	client := NewClient(config.InfraControlConfig{BaseURL: source.URL, APIToken: "token", Timeout: time.Second})
	_, err := client.StartVM(context.Background(), "req", 3052)
	if err == nil || destination.Load() != 0 {
		t.Fatalf("error=%v destination=%d", err, destination.Load())
	}
}
func TestStartVMCanceledBeforeSendMakesNoRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "token", Timeout: time.Second})
	_, err := client.StartVM(ctx, "req", 3052)
	if err == nil || calls.Load() != 0 {
		t.Fatalf("error=%v calls=%d", err, calls.Load())
	}
}
