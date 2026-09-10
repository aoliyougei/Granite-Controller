package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/managed"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

type serviceStub struct {
	response managed.ResponseBody
	err      *apierror.Error
}

func (serviceStub) Ready() bool                         { return true }
func (serviceStub) ModelList() openai.ModelListResponse { return openai.ModelList("granite-4.0-350m") }
func (s serviceStub) Complete(context.Context, string, openai.ChatCompletionRequest) (managed.ResponseBody, *apierror.Error) {
	return s.response, s.err
}
func TestModelsOwnedByIBMGranite(t *testing.T) {
	rr := httptest.NewRecorder()
	Models(serviceStub{}).ServeHTTP(rr, httptest.NewRequest("GET", "/v1/models", nil))
	if !strings.Contains(rr.Body.String(), `"owned_by":"ibm-granite"`) {
		t.Fatal(rr.Body.String())
	}
}
func TestChatStreamsFinalAssistantWithoutToolCalls(t *testing.T) {
	content := "▶️ VM 3052 的启动请求已提交。"
	response := managed.ResponseBody{ID: "chatcmpl_x", Model: "granite-4.0-350m", Choices: []managed.Choice{{Message: managed.AssistantMessage{Role: "assistant", Content: &content}, FinishReason: "stop"}}, XGranite: managed.GraniteMetadata{Tool: "pve_vm_start", Arguments: managed.Arguments{VMID: 3052}, Executed: true}}
	body := `{"model":"granite-4.0-350m","messages":[{"role":"user","content":"启动 VM 3052"}],"stream":true}`
	rr := httptest.NewRecorder()
	ChatCompletions(serviceStub{response: response}).ServeHTTP(rr, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	got := rr.Body.String()
	if rr.Code != 200 || !strings.Contains(got, "[DONE]") || !strings.Contains(got, "pve_vm_start") || strings.Contains(got, "tool_calls") {
		t.Fatalf("status=%d body=%s", rr.Code, got)
	}
}
