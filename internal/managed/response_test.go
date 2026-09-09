package managed

import (
	"encoding/json"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/native"
	"strings"
	"testing"
)

func TestResponseReturnsFinalChineseAssistantWithoutToolCalls(t *testing.T) {
	call := ValidatedCall{VMID: 3052, Confidence: 0.92, Validation: native.Validation{Ungrounded: []string{}, Negation: false}}
	got := Response("needle-2", call, infracontrol.Result{Status: 202}, []string{"client tools ignored"})
	if got.ID == "" || got.Object != "chat.completion" || got.Model != "needle-2" || got.Created == 0 || len(got.Choices) != 1 {
		t.Fatalf("response=%+v", got)
	}
	choice := got.Choices[0]
	if choice.FinishReason != "stop" || choice.Message.Role != "assistant" || choice.Message.Content == nil || *choice.Message.Content != "VM 3052 的启动请求已提交。" {
		t.Fatalf("choice=%+v", choice)
	}
	encodedResponse, _ := json.Marshal(got)
	if strings.Contains(string(encodedResponse), `"tool_calls"`) {
		t.Fatalf("executed response exposed tool_calls: %s", encodedResponse)
	}
	if got.XNeedle.Tool != "pve_vm_start" || got.XNeedle.Arguments.VMID != 3052 || got.XNeedle.Confidence != 0.92 || !got.XNeedle.Executed || got.XNeedle.UpstreamStatus != 202 || len(got.XNeedle.Warnings) != 1 || !got.Usage.Estimated {
		t.Fatalf("metadata=%+v", got)
	}
}
func TestResponseUsesEmptyWarningsArray(t *testing.T) {
	got := Response("needle-2", ValidatedCall{VMID: 1}, infracontrol.Result{Status: 202}, nil)
	if got.XNeedle.Warnings == nil {
		t.Fatal("warnings nil")
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), `"warnings":null`) {
		t.Fatal("warnings serialized null")
	}
}
