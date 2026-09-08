package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"needle-controller/internal/apierror"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
)

type fakeOpenAI struct {
	state    native.State
	response openai.ChatCompletionResponse
	err      *apierror.Error
	request  openai.ChatCompletionRequest
}

func (f *fakeOpenAI) State() native.State                 { return f.state }
func (f *fakeOpenAI) ModelList() openai.ModelListResponse { return openai.ModelList("needle-2") }
func (f *fakeOpenAI) Complete(_ context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, *apierror.Error) {
	f.request = r
	return f.response, f.err
}

func TestReadyReflectsOnlyModelState(t *testing.T) {
	for _, tc := range []struct {
		state  native.State
		status int
	}{{native.StateLoading, 503}, {native.StateReady, 200}, {native.StateFailed, 503}} {
		w := httptest.NewRecorder()
		Ready(&fakeOpenAI{state: tc.state}).ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"model":"`+string(tc.state)+`"`) {
			t.Fatalf("state=%s status=%d body=%s", tc.state, w.Code, w.Body.String())
		}
	}
}

func TestModelsReturnsOpenAIList(t *testing.T) {
	w := httptest.NewRecorder()
	Models(&fakeOpenAI{state: native.StateReady}).ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"needle-2"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func completionResponse() openai.ChatCompletionResponse {
	confidence := 0.9
	return openai.ChatCompletionResponse{ID: "chatcmpl-test", Object: "chat.completion", Created: 1, Model: "needle-2", Choices: []openai.Choice{{Index: 0, Message: openai.AssistantMessage{Role: "assistant", ToolCalls: []openai.ToolCallResponse{{ID: "call-test", Type: "function", Function: openai.FunctionCallResponse{Name: "x", Arguments: `{}`}}}}, FinishReason: "tool_calls"}}, Usage: openai.Usage{Estimated: true}, XNeedle: openai.XNeedle{Type: "call", Confidence: &confidence}}
}

func TestChatCompletionsReturnsJSONAndAllowsUnknownTopLevelFields(t *testing.T) {
	service := &fakeOpenAI{state: native.StateReady, response: completionResponse()}
	body := `{"model":"needle-2","messages":[{"role":"user","content":"x"}],"tools":[{"type":"function","function":{"name":"x","parameters":{"type":"object"}}}],"vendor_extension":true}`
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" || service.request.Model != "needle-2" || !strings.Contains(w.Body.String(), `"tool_calls"`) {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
}

func TestChatCompletionsReturnsOpenAIError(t *testing.T) {
	service := &fakeOpenAI{state: native.StateReady, err: apierror.OpenAI("tools_required", "tools required", "tools", 400, nil)}
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"needle-2"}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), `"code":"tools_required"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChatCompletionsStreamsSSE(t *testing.T) {
	service := &fakeOpenAI{state: native.StateReady, response: completionResponse()}
	body := `{"model":"needle-2","stream":true,"stream_options":{"include_usage":true}}`
	w := httptest.NewRecorder()
	ChatCompletions(service).ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") || !strings.Contains(w.Body.String(), "data: [DONE]\n\n") || !strings.Contains(w.Body.String(), `"usage"`) {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
}

func TestDecodeJSONRejectsTrailingDataButNotUnknownFields(t *testing.T) {
	var request openai.ChatCompletionRequest
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"needle-2","unknown":1} {}`))
	if err := DecodeJSON(w, r, &request, 8192); err == nil {
		t.Fatal("expected trailing-data error")
	}
	_ = json.Valid
}
