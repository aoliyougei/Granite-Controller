package managed

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/granite"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

type selectorStub struct {
	call  granite.ToolCall
	texts []string
}

func (s *selectorStub) Select(_ context.Context, text string, _ []openai.FunctionTool) (granite.ToolCall, *apierror.Error) {
	s.texts = append(s.texts, text)
	return s.call, nil
}

type infraStub struct {
	mu          sync.Mutex
	gets, posts int
	vm          infracontrol.VM
}

func (i *infraStub) GetVM(context.Context, string, int64) (infracontrol.VM, *infracontrol.Error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.gets++
	return i.vm, nil
}
func (i *infraStub) Mutate(context.Context, string, string, int64) (infracontrol.Result, *infracontrol.Error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.posts++
	return infracontrol.Result{Status: 202}, nil
}

func chat(text string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{Model: "granite-4.0-350m", Messages: []openai.Message{{Role: "user", Content: func() json.RawMessage { b, _ := json.Marshal(text); return b }()}}}
}
func TestServiceReadinessBlocksCompletion(t *testing.T) {
	sel := &selectorStub{}
	infra := &infraStub{}
	s := NewService(sel, infra, ServiceConfig{ModelID: "granite-4.0-350m", MaxMessageLength: 2048, DedupWindow: time.Minute, Ready: func() bool { return false }})
	if s.Ready() {
		t.Fatal("ready")
	}
	_, err := s.Complete(context.Background(), "rid", chat("查询 VM 3052 状态"))
	if err == nil || err.Code != "model_not_ready" {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceQueriesAndRendersWithoutMutation(t *testing.T) {
	sel := &selectorStub{call: granite.ToolCall{Type: "function", Name: "pve_vm_get", Arguments: `{"vmid":3052}`}}
	infra := &infraStub{vm: infracontrol.VM{VMID: 3052, Name: "db", Status: "running", Uptime: 12588}}
	s := NewService(sel, infra, ServiceConfig{ModelID: "granite-4.0-350m", MaxMessageLength: 2048, DedupWindow: 30 * time.Second})
	response, err := s.Complete(context.Background(), "rid", chat("查询 VM 3052 状态"))
	if err != nil || infra.gets != 1 || infra.posts != 0 || response.XGranite.Tool != "pve_vm_get" || response.Choices[0].Message.Content == nil {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}
func TestServiceDeduplicatesMutationBeforeStateQuery(t *testing.T) {
	sel := &selectorStub{call: granite.ToolCall{Type: "function", Name: "pve_vm_start", Arguments: `{"vmid":3052}`}}
	infra := &infraStub{vm: infracontrol.VM{VMID: 3052, Status: "stopped"}}
	s := NewService(sel, infra, ServiceConfig{ModelID: "granite-4.0-350m", MaxMessageLength: 2048, DedupWindow: time.Minute})
	first, e1 := s.Complete(context.Background(), "a", chat("启动 VM 3052"))
	sel.call.Arguments = `{ "vmid": 3052 }`
	second, e2 := s.Complete(context.Background(), "b", chat("启动 VM 3052"))
	if e1 != nil || e2 != nil || infra.gets != 1 || infra.posts != 1 || !first.XGranite.Executed || !second.XGranite.Deduplicated || second.XGranite.Executed {
		t.Fatalf("first=%+v second=%+v gets=%d posts=%d", first.XGranite, second.XGranite, infra.gets, infra.posts)
	}
}
