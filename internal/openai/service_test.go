package openai

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"needle-controller/internal/native"
)

type fakeCompleter struct {
	state    native.State
	envelope native.Envelope
	err      error
	request  native.Request
	calls    int
}

func (f *fakeCompleter) Submit(_ context.Context, request native.Request) (native.Envelope, error) {
	f.calls++
	f.request = request
	return f.envelope, f.err
}
func (f *fakeCompleter) State() native.State { return f.state }

func serviceRequest() ChatCompletionRequest {
	return ChatCompletionRequest{Model: "needle-2", Messages: []Message{{Role: "user", Content: text("Start VM 3052")}}, Tools: []FunctionTool{functionTool("pve_vm_start", `{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":false}`)}}
}

func TestServiceValidatesBuildsReplayAndMapsResponse(t *testing.T) {
	confidence := 0.9
	fake := &fakeCompleter{state: native.StateReady, envelope: native.Envelope{Type: "call", Success: true, Confidence: &confidence, FunctionCalls: []native.FunctionCall{{Name: "pve_vm_start", Arguments: map[string]json.RawMessage{"vmid": json.RawMessage(`3052`)}}}}}
	service := NewService(fake, replayConfig(), DefaultSchemaLimits())
	got, err := service.Complete(context.Background(), serviceRequest())
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || len(fake.request.Turns) != 1 || fake.request.Turns[0].Text != "Start VM 3052" || got.Choices[0].Message.ToolCalls[0].Function.Name != "pve_vm_start" {
		t.Fatalf("request=%+v response=%+v", fake.request, got)
	}
}

func TestServiceMapsNativeFailures(t *testing.T) {
	tests := []struct {
		name   string
		state  native.State
		err    error
		status int
		code   string
	}{
		{"loading", native.StateLoading, nil, 503, "model_not_ready"},
		{"failed", native.StateFailed, nil, 503, "model_not_ready"},
		{"queue", native.StateReady, native.ErrQueueFull, 429, "engine_busy"},
		{"engine", native.StateReady, &native.NativeError{Code: "native_complete_failed", Message: "private"}, 502, "engine_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeCompleter{state: tc.state, err: tc.err}
			_, err := NewService(fake, replayConfig(), DefaultSchemaLimits()).Complete(context.Background(), serviceRequest())
			if err == nil || err.HTTPStatus != tc.status || err.Code != tc.code {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}

func TestServicePreservesRequestValidationError(t *testing.T) {
	fake := &fakeCompleter{state: native.StateReady}
	req := serviceRequest()
	req.Tools = nil
	_, err := NewService(fake, replayConfig(), DefaultSchemaLimits()).Complete(context.Background(), req)
	if err == nil || err.Code != "tools_required" || fake.calls != 0 {
		t.Fatalf("error=%+v calls=%d", err, fake.calls)
	}
}

func TestServiceMapsCanceledContext(t *testing.T) {
	fake := &fakeCompleter{state: native.StateReady, err: context.Canceled}
	_, err := NewService(fake, replayConfig(), DefaultSchemaLimits()).Complete(context.Background(), serviceRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
