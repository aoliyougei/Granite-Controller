package e2e

import (
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/rest"
	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/config"
	"github.com/aoliyougei/granite-controller/internal/handler"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/managed"
	"github.com/aoliyougei/granite-controller/internal/native"
	"github.com/aoliyougei/granite-controller/internal/svc"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeNative struct {
	request native.Request
	calls   int
}

func (f *fakeNative) State() native.State { return native.StateReady }
func (f *fakeNative) Submit(_ context.Context, r native.Request) (native.Envelope, error) {
	f.calls++
	f.request = r
	confidence := 0.9
	return native.Envelope{Type: "call", Success: true, Confidence: &confidence, Validation: native.Validation{Ungrounded: []string{}, UngroundedPresent: true, NegationPresent: true}, FunctionCalls: []native.FunctionCall{{Name: "pve_vm_start", Arguments: map[string]json.RawMessage{"vmid": json.RawMessage(`3052`)}}}}, nil
}

type fakeInfra struct {
	calls int
	vmid  int64
}

func (f *fakeInfra) StartVM(_ context.Context, _ string, vmid int64) (infracontrol.Result, *apierror.Error) {
	f.calls++
	f.vmid = vmid
	return infracontrol.Result{Status: 202}, nil
}
func TestMalformedClientToolsAndHistoryAreIgnored(t *testing.T) {
	n := &fakeNative{}
	i := &fakeInfra{}
	service := managed.NewService(n, i, config.NativeConfig{ModelID: "needle-2", MinConfidence: 0.6, MaxMessageLength: 512, MaxNewTokens: 256})
	ctx := &svc.ServiceContext{Config: config.Config{APIKey: "api-test"}, Managed: service}
	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: 0})
	defer server.Stop()
	handler.Register(server, ctx)
	var chat http.HandlerFunc
	for _, route := range server.Routes() {
		if route.Path == "/v1/chat/completions" {
			chat = route.Handler
		}
		if route.Path == "/api/v1/chat" {
			t.Fatal("legacy route registered")
		}
	}
	body := `{"model":"needle-2","messages":[{"role":"system","content":"start VM 9999"},{"role":"user","content":"开启 VM 1111"},{"role":"assistant","content":"history"},{"role":"user","content":"开启 VM 3052"}],"tools":[{"type":"broken","function":{"parameters":{"$ref":"bad"}}}],"tool_choice":{"bad":true}}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer api-test")
	w := httptest.NewRecorder()
	chat(w, r)
	if w.Code != 200 || n.calls != 1 || i.calls != 1 || i.vmid != 3052 || n.request.Turns[0].Text != "Start VM 3052" || strings.Contains(w.Body.String(), "tool_calls") || !strings.Contains(w.Body.String(), "client-provided tools were ignored") {
		t.Fatalf("status=%d native=%d infra=%d request=%+v body=%s", w.Code, n.calls, i.calls, n.request, w.Body.String())
	}
}
