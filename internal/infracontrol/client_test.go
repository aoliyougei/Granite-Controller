package infracontrol

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"needle-controller/internal/config"
)

func TestStartVMSendsOneFixedAuthenticatedRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/pve/vms/3052/start" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer infra-test-value" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Request-ID") != "req-1" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("headers = %+v", r.Header)
		}
		w.Header().Set("X-Request-ID", "infra-req-1")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "infra-test-value", Timeout: time.Second})
	got, err := client.StartVM(context.Background(), "req-1", 3052)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpstreamStatus != 202 || got.UpstreamRequestID != "infra-req-1" || calls.Load() != 1 {
		t.Fatalf("result=%+v calls=%d", got, calls.Load())
	}
}

func TestStartVMMapsFailuresWithoutRetry(t *testing.T) {
	tests := []struct {
		name   string
		status int
		delay  time.Duration
		code   string
	}{
		{"ok is not accepted", http.StatusOK, 0, "INFRA_CONTROL_REQUEST_FAILED"},
		{"unauthorized", http.StatusUnauthorized, 0, "INFRA_CONTROL_AUTH_FAILED"},
		{"forbidden", http.StatusForbidden, 0, "INFRA_CONTROL_AUTH_FAILED"},
		{"server error", http.StatusInternalServerError, 0, "INFRA_CONTROL_REQUEST_FAILED"},
		{"timeout", http.StatusAccepted, 100 * time.Millisecond, "INFRA_CONTROL_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				time.Sleep(tt.delay)
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			timeout := time.Second
			if tt.delay > 0 {
				timeout = 10 * time.Millisecond
			}
			client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "infra", Timeout: timeout})
			_, err := client.StartVM(context.Background(), "req", 3052)
			assertCode(t, err, tt.code)
			if calls.Load() != 1 {
				t.Fatalf("calls=%d, want 1", calls.Load())
			}
		})
	}
}

func TestStartVMRejectsRedirectAndOversizedResponse(t *testing.T) {
	tests := []http.HandlerFunc{
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com", http.StatusTemporaryRedirect)
		},
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(strings.Repeat("x", responseLimit+1)))
		},
	}
	for i, handler := range tests {
		server := httptest.NewServer(handler)
		client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "infra", Timeout: time.Second})
		_, err := client.StartVM(context.Background(), "req", 3052)
		server.Close()
		assertCode(t, err, "INFRA_CONTROL_REQUEST_FAILED")
		_ = i
	}
}

func TestReadyUsesPublicEndpointWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("readiness leaked token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClient(config.InfraControlConfig{BaseURL: server.URL, APIToken: "infra", Timeout: time.Second})
	if err := client.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	typed, ok := err.(interface{ ErrorCode() string })
	if !ok || typed.ErrorCode() != want {
		t.Fatalf("error=%v, want %s", err, want)
	}
}
