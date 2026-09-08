package handler

import (
	"context"
	"needle-controller/internal/apierror"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/managed"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeManaged struct {
	state     native.State
	response  managed.ResponseBody
	err       *apierror.Error
	request   openai.ChatCompletionRequest
	requestID string
	calls     int
}

func (f *fakeManaged) State() native.State                 { return f.state }
func (f *fakeManaged) ModelList() openai.ModelListResponse { return openai.ModelList("needle-2") }
func (f *fakeManaged) Complete(_ context.Context, id string, r openai.ChatCompletionRequest) (managed.ResponseBody, *apierror.Error) {
	f.calls++
	f.requestID = id
	f.request = r
	return f.response, f.err
}
func managedResponse() managed.ResponseBody {
	return managed.Response("needle-2", managed.ValidatedCall{VMID: 3052, Confidence: 0.9}, infracontrol.Result{Status: 202}, []string{"client-provided tools were ignored"})
}
func TestReadyReflectsModelState(t *testing.T) {
	for _, tc := range []struct {
		state  native.State
		status int
	}{{native.StateLoading, 503}, {native.StateReady, 200}, {native.StateFailed, 503}} {
		w := httptest.NewRecorder()
		Ready(&fakeManaged{state: tc.state}).ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
		if w.Code != tc.status {
			t.Fatalf("state=%s status=%d", tc.state, w.Code)
		}
	}
}
func TestModelsReturnsOpenAIList(t *testing.T) {
	w := httptest.NewRecorder()
	Models(&fakeManaged{state: native.StateReady}).ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"needle-2"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
}
func TestChatReturnsManagedFinalTextAndPassesRequestID(t *testing.T) {
	service := &fakeManaged{state: native.StateReady, response: managedResponse()}
	body := `{"model":"needle-2","messages":[{"role":"system","content":"ignored"},{"role":"user","content":"开启 VM 3052"}],"tools":[{"broken":true}],"vendor":true}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("X-Request-ID", "req-1")
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, r)
	if w.Code != 200 || service.requestID != "req-1" || service.request.Model != "needle-2" || !strings.Contains(w.Body.String(), "VM 3052 的启动请求已提交") || strings.Contains(w.Body.String(), "tool_calls") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestChatAcceptsLargeIgnoredAgentContext(t *testing.T) {
	service := &fakeManaged{state: native.StateReady, response: managedResponse()}
	ignored := strings.Repeat("x", 3<<20)
	body := `{"model":"needle-2","messages":[{"role":"system","content":"` + ignored + `"},{"role":"user","content":"开启 VM 3052"}]}`
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	if w.Code != 200 || service.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, service.calls, w.Body.String())
	}
}

func TestChatRejectsRequestOverEightMiBBeforeService(t *testing.T) {
	service := &fakeManaged{state: native.StateReady, response: managedResponse()}
	ignored := strings.Repeat("x", (8<<20)+1)
	body := `{"model":"needle-2","messages":[{"role":"system","content":"` + ignored + `"},{"role":"user","content":"开启 VM 3052"}]}`
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	if w.Code == 200 || service.calls != 0 {
		t.Fatalf("status=%d calls=%d", w.Code, service.calls)
	}
}

func TestChatReturnsManagedOpenAIError(t *testing.T) {
	service := &fakeManaged{err: apierror.OpenAI("user_message_required", "required", "messages", 400, nil)}
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"needle-2"}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "user_message_required") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestChatStreamsManagedContentWithoutToolCalls(t *testing.T) {
	service := &fakeManaged{response: managedResponse()}
	body := `{"model":"needle-2","messages":[{"role":"user","content":"开启 VM 3052"}],"stream":true,"stream_options":{"include_usage":true}}`
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") || !strings.Contains(w.Body.String(), "VM 3052 的启动请求已提交") || !strings.Contains(w.Body.String(), "data: [DONE]\n\n") || strings.Contains(w.Body.String(), "tool_calls") {
		t.Fatalf("body=%s", w.Body.String())
	}
}
