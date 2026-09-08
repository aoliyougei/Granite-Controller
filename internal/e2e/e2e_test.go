package e2e

import (
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest"
	"needle-controller/internal/config"
	"needle-controller/internal/handler"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"needle-controller/internal/svc"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCompleter struct {
	envelope native.Envelope
	calls    int
}

func (f *fakeCompleter) Submit(context.Context, native.Request) (native.Envelope, error) {
	f.calls++
	return f.envelope, nil
}
func (f *fakeCompleter) State() native.State { return native.StateReady }

func TestServerNeverExecutesToolsAndOldRouteIsGone(t *testing.T) {
	confidence := 0.9
	fake := &fakeCompleter{envelope: native.Envelope{Type: "call", Success: true, Confidence: &confidence, FunctionCalls: []native.FunctionCall{{Name: "pve_vm_start", Arguments: map[string]json.RawMessage{"vmid": json.RawMessage(`3052`)}}}}}
	service := openai.NewService(fake, config.NativeConfig{ModelID: "needle-2", MaxNewTokens: 256, MaxReplaySteps: 32}, openai.DefaultSchemaLimits())
	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: 0})
	defer server.Stop()
	ctx := &svc.ServiceContext{Config: config.Config{APIKey: "api-test-value"}, OpenAI: service}
	handler.Register(server, ctx)
	routes := server.Routes()
	var chat http.HandlerFunc
	for _, route := range routes {
		if route.Path == "/v1/chat/completions" {
			chat = route.Handler
		}
		if route.Path == "/api/v1/chat" {
			t.Fatal("legacy execution route still registered")
		}
	}
	if chat == nil {
		t.Fatal("OpenAI chat route missing")
	}
	body := `{"model":"needle-2","messages":[{"role":"user","content":"Start VM 3052"}],"tools":[{"type":"function","function":{"name":"pve_vm_start","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":false}}}]}`
	request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer api-test-value")
	response := httptest.NewRecorder()
	chat(response, request)
	if response.Code != 200 || fake.calls != 1 || !strings.Contains(response.Body.String(), `"name":"pve_vm_start"`) {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, fake.calls, response.Body.String())
	}
}
