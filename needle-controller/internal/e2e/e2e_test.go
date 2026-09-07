package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"needle-controller/internal/config"
	"needle-controller/internal/handler"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/logic"
	"needle-controller/internal/middleware"
	"needle-controller/internal/needle"
)

func TestVMStartHappyPath(t *testing.T) {
	var infraCalls atomic.Int32
	infra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		infraCalls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/pve/vms/3052/start" {
			t.Errorf("infrastructure request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer infra-test-value" || r.Header.Get("X-Request-ID") != "req-e2e" {
			t.Errorf("infrastructure headers = %+v", r.Header)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer infra.Close()

	needleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("needle request = %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 1 || messages[0].(map[string]any)["content"] != "Start VM 3052" {
			t.Errorf("messages = %#v", payload["messages"])
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Errorf("tools = %#v", payload["tools"])
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"pve_vm_start","arguments":"{\"vmid\":3052}"}}]}}],"x_needle":{"confidence":0.92,"validation":{"ungrounded":[],"negation":false}}}`))
	}))
	defer needleServer.Close()

	h := buildHandler(needleServer.URL, infra.URL)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"开启 3052 这个 VM"}`))
	r.Header.Set("Authorization", "Bearer controller-test-value")
	r.Header.Set("X-Request-ID", "req-e2e")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted || infraCalls.Load() != 1 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, infraCalls.Load(), w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"vmid":3052`) || !strings.Contains(w.Body.String(), `"status":"accepted"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestRejectedModelOutputsNeverReachInfrastructure(t *testing.T) {
	responses := []struct{ name, body, code string }{
		{"no call", `{"choices":[{"message":{}}],"x_needle":{"confidence":0.92,"validation":{"ungrounded":[],"negation":false}}}`, "NEEDLE_NO_TOOL_CALL"},
		{"low confidence", completion(`pve_vm_start`, `{"vmid":3052}`, `0.2`, `[]`, `false`), "NEEDLE_LOW_CONFIDENCE"},
		{"ungrounded", completion(`pve_vm_start`, `{"vmid":3052}`, `0.92`, `["vmid"]`, `false`), "NEEDLE_ARGUMENTS_UNGROUNDED"},
		{"negation", completion(`pve_vm_start`, `{"vmid":3052}`, `0.92`, `[]`, `true`), "NEEDLE_NEGATION_DETECTED"},
		{"bad arguments", completion(`pve_vm_start`, `{"vmid":"3052"}`, `0.92`, `[]`, `false`), "NEEDLE_VMID_INVALID"},
		{"wrong tool", completion(`http_request`, `{"vmid":3052}`, `0.92`, `[]`, `false`), "NEEDLE_TOOL_NOT_ALLOWED"},
	}
	for _, tc := range responses {
		t.Run(tc.name, func(t *testing.T) {
			var infraCalls atomic.Int32
			infra := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { infraCalls.Add(1) }))
			defer infra.Close()
			n := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer n.Close()
			h := buildHandler(n.URL, infra.URL)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"启动 VM 3052"}`))
			r.Header.Set("Authorization", "Bearer controller-test-value")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), tc.code) || infraCalls.Load() != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, infraCalls.Load(), w.Body.String())
			}
		})
	}
}

func TestAuthenticationFailureCallsNoDependency(t *testing.T) {
	var needleCalls, infraCalls atomic.Int32
	n := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { needleCalls.Add(1) }))
	defer n.Close()
	infra := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { infraCalls.Add(1) }))
	defer infra.Close()
	w := httptest.NewRecorder()
	buildHandler(n.URL, infra.URL).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"start"}`)))
	if w.Code != http.StatusUnauthorized || needleCalls.Load() != 0 || infraCalls.Load() != 0 {
		t.Fatalf("status=%d needle=%d infra=%d", w.Code, needleCalls.Load(), infraCalls.Load())
	}
}

func buildHandler(needleURL, infraURL string) http.Handler {
	n := needle.NewClient(config.NeedleConfig{BaseURL: needleURL, Model: "needle-2", Timeout: time.Second, MaxTokens: 256})
	i := infracontrol.NewClient(config.InfraControlConfig{BaseURL: infraURL, APIToken: "infra-test-value", Timeout: time.Second})
	chat := logic.NewChatService(n, i, 0.6, 512)
	return middleware.NewBearerAuth("controller-test-value")(handler.Chat(chat))
}

func completion(name, arguments, confidence, ungrounded, negation string) string {
	encoded, _ := json.Marshal(arguments)
	return `{"choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"` + name + `","arguments":` + string(encoded) + `}}]}}],"x_needle":{"confidence":` + confidence + `,"validation":{"ungrounded":` + ungrounded + `,"negation":` + negation + `}}}`
}
