package managed

import (
	"context"
	"encoding/json"
	"errors"
	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"strings"
	"testing"
)

type fakeNative struct {
	state    native.State
	envelope native.Envelope
	err      error
	calls    int
	request  native.Request
	order    *[]string
}

func (f *fakeNative) State() native.State { return f.state }
func (f *fakeNative) Submit(_ context.Context, r native.Request) (native.Envelope, error) {
	f.calls++
	f.request = r
	if f.order != nil {
		*f.order = append(*f.order, "needle")
	}
	return f.envelope, f.err
}

type fakeInfra struct {
	result    infracontrol.Result
	err       *apierror.Error
	calls     int
	vmid      int64
	requestID string
	order     *[]string
}

func (f *fakeInfra) StartVM(_ context.Context, requestID string, vmid int64) (infracontrol.Result, *apierror.Error) {
	f.calls++
	f.requestID = requestID
	f.vmid = vmid
	if f.order != nil {
		*f.order = append(*f.order, "infra")
	}
	return f.result, f.err
}
func managedRequest(content string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{Model: "needle-2", Messages: []openai.Message{{Role: "user", Content: raw(content)}}}
}
func serviceConfig() config.NativeConfig {
	return config.NativeConfig{ModelID: "needle-2", MinConfidence: 0.6, MaxMessageLength: 512, MaxNewTokens: 256}
}
func TestServiceRequiresNeedleThenExecutesExactlyOnce(t *testing.T) {
	order := []string{}
	n := &fakeNative{state: native.StateReady, envelope: validEnvelope(), order: &order}
	i := &fakeInfra{result: infracontrol.Result{Status: 202}, order: &order}
	service := NewService(n, i, serviceConfig())
	got, err := service.Complete(context.Background(), "req-1", managedRequest("开启 VM 3052"))
	if err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 || i.calls != 1 || n.request.Turns[0].Text != "Start VM 3052" || i.vmid != 3052 || i.requestID != "req-1" || len(order) != 2 || order[0] != "needle" || order[1] != "infra" {
		t.Fatalf("native=%+v infra=%+v order=%v", n, i, order)
	}
	encoded, _ := json.Marshal(got)
	if got.Choices[0].Message.Content == nil || string(encoded) == "" || strings.Contains(string(encoded), "tool_calls") || !strings.Contains(string(encoded), "VM 3052 的启动请求已提交") {
		t.Fatalf("response=%s", encoded)
	}
}
func TestServiceShortCircuitsBeforeInfrastructure(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeNative, *openai.ChatCompletionRequest)
		code  string
	}{
		{"invalid input", func(_ *fakeNative, r *openai.ChatCompletionRequest) { r.Messages = nil }, "user_message_required"}, {"unsupported", func(_ *fakeNative, r *openai.ChatCompletionRequest) { *r = managedRequest("关闭 VM 3052") }, "unsupported_managed_command"}, {"loading", func(n *fakeNative, _ *openai.ChatCompletionRequest) { n.state = native.StateLoading }, "model_not_ready"}, {"queue", func(n *fakeNative, _ *openai.ChatCompletionRequest) { n.err = native.ErrQueueFull }, "engine_busy"}, {"native", func(n *fakeNative, _ *openai.ChatCompletionRequest) {
			n.err = &native.NativeError{Code: "failed", Message: "x"}
		}, "engine_error"}, {"mismatch", func(n *fakeNative, _ *openai.ChatCompletionRequest) {
			n.envelope = validEnvelope()
			n.envelope.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`3053`)
		}, "argument_mismatch"}, {"low", func(n *fakeNative, _ *openai.ChatCompletionRequest) {
			n.envelope = validEnvelope()
			n.envelope.Confidence = floatPointer(0.2)
		}, "low_confidence"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := &fakeNative{state: native.StateReady, envelope: validEnvelope()}
			i := &fakeInfra{}
			request := managedRequest("开启 VM 3052")
			tc.setup(n, &request)
			_, err := NewService(n, i, serviceConfig()).Complete(context.Background(), "req", request)
			if err == nil || err.Code != tc.code || i.calls != 0 {
				t.Fatalf("error=%+v infra=%d", err, i.calls)
			}
		})
	}
}
func TestServiceReturnsUpstreamErrorWithoutRetry(t *testing.T) {
	n := &fakeNative{state: native.StateReady, envelope: validEnvelope()}
	want := apierror.OpenAI("upstream_request_failed", "failed", "", 502, errors.New("network"))
	i := &fakeInfra{err: want}
	_, err := NewService(n, i, serviceConfig()).Complete(context.Background(), "req", managedRequest("开启 VM 3052"))
	if err != want || i.calls != 1 {
		t.Fatalf("error=%v calls=%d", err, i.calls)
	}
}
