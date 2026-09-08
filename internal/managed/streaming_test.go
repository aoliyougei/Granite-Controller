package managed

import (
	"encoding/json"
	"needle-controller/internal/infracontrol"
	"strings"
	"testing"
)

func TestStreamEventsEmitFinalTextAndNoToolCalls(t *testing.T) {
	response := Response("needle-2", ValidatedCall{VMID: 3052, Confidence: 0.9}, infracontrol.Result{Status: 202}, nil)
	events := StreamEvents(response, true)
	if len(events) != 5 || string(events[len(events)-1].Data) != "[DONE]" {
		t.Fatalf("events=%+v", events)
	}
	for _, event := range events[:len(events)-1] {
		if strings.Contains(string(event.Data), "tool_calls") {
			t.Fatalf("tool call emitted: %s", event.Data)
		}
		var chunk map[string]any
		if json.Unmarshal(event.Data, &chunk) != nil {
			t.Fatalf("invalid chunk: %s", event.Data)
		}
		if chunk["id"] != response.ID || chunk["model"] != response.Model {
			t.Fatalf("identity=%v", chunk)
		}
	}
	if !strings.Contains(string(events[1].Data), "VM 3052 的启动请求已提交。") || !strings.Contains(string(events[2].Data), `"finish_reason":"stop"`) || !strings.Contains(string(events[3].Data), `"usage"`) {
		t.Fatalf("events=%+v", events)
	}
}
func TestStreamEventsOmitUsageWhenNotRequested(t *testing.T) {
	response := Response("needle-2", ValidatedCall{VMID: 1}, infracontrol.Result{Status: 202}, nil)
	events := StreamEvents(response, false)
	if len(events) != 4 {
		t.Fatalf("events=%d", len(events))
	}
	for _, event := range events {
		if strings.Contains(string(event.Data), `"usage"`) {
			t.Fatalf("usage emitted: %s", event.Data)
		}
	}
}
