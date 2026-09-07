package needle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needle-controller/internal/config"
)

func TestClientSendsFixedToolAndParsesCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer needle-test-value" {
			t.Errorf("Authorization = %q", got)
		}
		var payload chatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "pve_vm_start" {
			t.Errorf("tools = %+v", payload.Tools)
		}
		if payload.Tools[0].Function.Parameters.AdditionalProperties {
			t.Error("additionalProperties must be false")
		}
		if payload.Messages[0].Content != "开启 3052 这个 VM" || payload.MaxTokens != 256 {
			t.Errorf("payload = %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"pve_vm_start","arguments":"{\"vmid\":3052}"}}]}}],"x_needle":{"confidence":0.92,"validation":{"ungrounded":[],"negation":false}}}`))
	}))
	defer server.Close()

	client := NewClient(config.NeedleConfig{BaseURL: server.URL, APIKey: "needle-test-value", Model: "needle-2", Timeout: time.Second, MaxTokens: 256})
	got, err := client.Complete(context.Background(), "开启 3052 这个 VM")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Function.Name != "pve_vm_start" || string(got.ToolCalls[0].Function.Arguments) != `{"vmid":3052}` {
		t.Fatalf("completion calls = %+v", got.ToolCalls)
	}
	if got.Safety.Confidence == nil || *got.Safety.Confidence != 0.92 || got.Safety.Validation.Negation == nil || *got.Safety.Validation.Negation {
		t.Fatalf("safety = %+v", got.Safety)
	}
}

func TestClientMapsFailuresAndReadiness(t *testing.T) {
	t.Run("busy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
		defer server.Close()
		client := NewClient(config.NeedleConfig{BaseURL: server.URL, Model: "needle-2", Timeout: time.Second, MaxTokens: 1})
		_, err := client.Complete(context.Background(), "start")
		assertCode(t, err, "NEEDLE_BUSY")
	})

	t.Run("malformed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{`)) }))
		defer server.Close()
		client := NewClient(config.NeedleConfig{BaseURL: server.URL, Model: "needle-2", Timeout: time.Second, MaxTokens: 1})
		_, err := client.Complete(context.Background(), "start")
		assertCode(t, err, "NEEDLE_UNAVAILABLE")
	})

	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", responseLimit+1)))
		}))
		defer server.Close()
		client := NewClient(config.NeedleConfig{BaseURL: server.URL, Model: "needle-2", Timeout: time.Second, MaxTokens: 1})
		_, err := client.Complete(context.Background(), "start")
		assertCode(t, err, "NEEDLE_UNAVAILABLE")
	})

	t.Run("ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("path=%s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()
		client := NewClient(config.NeedleConfig{BaseURL: server.URL, Timeout: time.Second})
		if err := client.Ready(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	typed, ok := err.(interface{ ErrorCode() string })
	if !ok || typed.ErrorCode() != want {
		t.Fatalf("error=%v, want code %s", err, want)
	}
}
