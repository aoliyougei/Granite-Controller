package logic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"needle-controller/internal/apierror"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/needle"
)

type fakeNeedle struct {
	completion needle.Completion
	err        error
	calls      int
	message    string
}

func (f *fakeNeedle) Complete(_ context.Context, message string) (needle.Completion, error) {
	f.calls++
	f.message = message
	return f.completion, f.err
}
func (f *fakeNeedle) Ready(context.Context) error { return nil }

type fakeInfra struct {
	result    infracontrol.StartResult
	err       error
	calls     int
	vmid      int64
	requestID string
}

func (f *fakeInfra) StartVM(_ context.Context, requestID string, vmid int64) (infracontrol.StartResult, error) {
	f.calls++
	f.requestID = requestID
	f.vmid = vmid
	return f.result, f.err
}
func (f *fakeInfra) Ready(context.Context) error { return nil }

func passingCompletion() needle.Completion {
	confidence, negation := 0.92, false
	return needle.Completion{
		ToolCalls: []needle.ToolCall{{ID: "call-1", Type: "function", Function: needle.FunctionCall{Name: "pve_vm_start", Arguments: json.RawMessage(`{"vmid":3052}`)}}},
		Safety:    needle.SafetyMetadata{Confidence: &confidence, Validation: needle.Validation{Ungrounded: json.RawMessage(`[]`), Negation: &negation}},
	}
}

func TestChatExecuteStartsValidatedVM(t *testing.T) {
	n := &fakeNeedle{completion: passingCompletion()}
	i := &fakeInfra{result: infracontrol.StartResult{UpstreamStatus: http.StatusAccepted}}
	service := NewChatService(n, i, 0.6, 512)

	got, err := service.Execute(context.Background(), "req-1", "  开启 3052 这个 VM  ")
	if err != nil {
		t.Fatal(err)
	}
	if n.message != "Start VM 3052" || n.calls != 1 {
		t.Fatalf("needle calls=%d message=%q", n.calls, n.message)
	}
	if i.calls != 1 || i.vmid != 3052 || i.requestID != "req-1" {
		t.Fatalf("infra=%+v", i)
	}
	if got.Status != "accepted" || got.Message != "VM 3052 的启动请求已提交" || got.Tool != "pve_vm_start" || got.Arguments.VMID != 3052 || got.Confidence != 0.92 || got.UpstreamStatus != 202 || got.RequestID != "req-1" {
		t.Fatalf("response=%+v", got)
	}
}

func TestChatExecuteShortCircuitsFailures(t *testing.T) {
	t.Run("empty message", func(t *testing.T) {
		n, i := &fakeNeedle{}, &fakeInfra{}
		_, err := NewChatService(n, i, 0.6, 5).Execute(context.Background(), "req", "  ")
		assertLogicCode(t, err, "MESSAGE_INVALID")
		if n.calls != 0 || i.calls != 0 {
			t.Fatal("dependencies called")
		}
	})
	t.Run("unicode length", func(t *testing.T) {
		n, i := &fakeNeedle{}, &fakeInfra{}
		_, err := NewChatService(n, i, 0.6, 2).Execute(context.Background(), "req", "开启机")
		assertLogicCode(t, err, "MESSAGE_INVALID")
		if n.calls != 0 || i.calls != 0 {
			t.Fatal("dependencies called")
		}
	})
	t.Run("needle error", func(t *testing.T) {
		n := &fakeNeedle{err: apierror.New(apierror.CodeNeedleUnavailable, "unavailable", 502, errors.New("network"))}
		i := &fakeInfra{}
		_, err := NewChatService(n, i, 0.6, 512).Execute(context.Background(), "req", "启动 VM 3052")
		assertLogicCode(t, err, "NEEDLE_UNAVAILABLE")
		if i.calls != 0 {
			t.Fatal("infra called")
		}
	})
	t.Run("validation error", func(t *testing.T) {
		c := passingCompletion()
		c.ToolCalls[0].Function.Name = "bad"
		n, i := &fakeNeedle{completion: c}, &fakeInfra{}
		_, err := NewChatService(n, i, 0.6, 512).Execute(context.Background(), "req", "启动 VM 3052")
		assertLogicCode(t, err, "NEEDLE_TOOL_NOT_ALLOWED")
		if i.calls != 0 {
			t.Fatal("infra called")
		}
	})
	t.Run("infra error", func(t *testing.T) {
		n := &fakeNeedle{completion: passingCompletion()}
		want := apierror.New(apierror.CodeInfraControlRequestFailed, "failed", 502, nil)
		i := &fakeInfra{err: want}
		_, err := NewChatService(n, i, 0.6, 512).Execute(context.Background(), "req", "启动 VM 3052")
		if !errors.Is(err, want) {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("model vmid mismatch", func(t *testing.T) {
		c := passingCompletion()
		c.ToolCalls[0].Function.Arguments = json.RawMessage(`{"vmid":3053}`)
		n, i := &fakeNeedle{completion: c}, &fakeInfra{}
		_, err := NewChatService(n, i, 0.6, 512).Execute(context.Background(), "req", "启动 VM 3052")
		assertLogicCode(t, err, "NEEDLE_ARGUMENTS_UNGROUNDED")
		if i.calls != 0 {
			t.Fatal("infra called for mismatched VM ID")
		}
	})
}

func assertLogicCode(t *testing.T, err error, want string) {
	t.Helper()
	typed, ok := err.(interface{ ErrorCode() string })
	if !ok || typed.ErrorCode() != want {
		t.Fatalf("error=%v, want %s", err, want)
	}
}
