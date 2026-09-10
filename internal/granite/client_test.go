package granite_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aoliyougei/granite-controller/internal/granite"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

func TestClientSendsDeterministicUnchangedChineseRequest(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer internal-key" { t.Fatalf("request=%s auth=%q", r.URL.Path, r.Header.Get("Authorization")) }
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil { t.Fatal(err) }
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"type":"function","function":{"name":"pve_vm_get","arguments":"{\"vmid\":3052}"}}]}}]}`))
	}))
	defer server.Close()
	client := granite.NewClient(server.URL, "internal-key", "granite-4.0-350m", 256, time.Second)
	tools := make([]openai.FunctionTool, 5)
	call, err := client.Select(context.Background(), "查询 VM 3052 是否运行", tools)
	if err != nil || call.Name != "pve_vm_get" { t.Fatalf("call=%+v err=%v", call, err) }
	messages := got["messages"].([]any)
	if messages[0].(map[string]any)["content"] != "查询 VM 3052 是否运行" || got["temperature"].(float64) != 0 || got["top_p"].(float64) != 1 || got["seed"].(float64) != 42 || got["stream"] != false || len(got["tools"].([]any)) != 5 { t.Fatalf("body=%v", got) }
}

func TestClientRejectsMissingAndMultipleCalls(t *testing.T) {
	for _, tc := range []struct{ body, code string }{
		{`{"choices":[{"message":{"content":"不知道"}}]}`, "tool_call_required"},
		{`{"choices":[{"message":{"tool_calls":[{"type":"function","function":{"name":"a","arguments":"{}"}},{"type":"function","function":{"name":"b","arguments":"{}"}}]}}]}`, "multiple_tool_calls_not_supported"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.body)) })); defer s.Close()
			_, err := granite.NewClient(s.URL, "key", "model", 256, time.Second).Select(context.Background(), "查询 3052", nil)
			if err == nil || err.Code != tc.code { t.Fatalf("err=%v", err) }
		})
	}
}
