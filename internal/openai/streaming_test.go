package openai

import (
	"encoding/json"
	"testing"

	"needle-controller/internal/native"
)

func TestStreamEventsMapsToolCallsAndUsage(t *testing.T) {
	response, err := MapResponse("needle-2", nativeCallEnvelope(), nil)
	if err != nil {
		t.Fatal(err)
	}
	events := StreamEvents(response, true)
	if len(events) != 8 {
		t.Fatalf("events=%d %#v", len(events), events)
	}
	if string(events[len(events)-1].Data) != "[DONE]" {
		t.Fatalf("last=%s", events[len(events)-1].Data)
	}
	var chunks []map[string]any
	for _, event := range events[:len(events)-1] {
		var value map[string]any
		if err := json.Unmarshal(event.Data, &value); err != nil {
			t.Fatalf("event=%s err=%v", event.Data, err)
		}
		chunks = append(chunks, value)
		if value["id"] != response.ID || value["model"] != "needle-2" || int64(value["created"].(float64)) != response.Created {
			t.Fatalf("identity=%v", value)
		}
	}
	choices := chunks[0]["choices"].([]any)
	delta := choices[0].(map[string]any)["delta"].(map[string]any)
	if delta["role"] != "assistant" {
		t.Fatalf("role=%v", delta)
	}
	for index := 0; index < 2; index++ {
		choice := chunks[1+index*2]["choices"].([]any)[0].(map[string]any)
		call := choice["delta"].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
		if int(call["index"].(float64)) != index {
			t.Fatalf("call=%v", call)
		}
		argsChoice := chunks[2+index*2]["choices"].([]any)[0].(map[string]any)
		args := argsChoice["delta"].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["function"].(map[string]any)["arguments"].(string)
		if args != response.Choices[0].Message.ToolCalls[index].Function.Arguments {
			t.Fatalf("args=%q", args)
		}
	}
	terminal := chunks[5]
	if terminal["x_needle"] == nil || terminal["choices"].([]any)[0].(map[string]any)["finish_reason"] != "tool_calls" {
		t.Fatalf("terminal=%v", terminal)
	}
	if chunks[6]["usage"] == nil {
		t.Fatalf("usage chunk=%v", chunks[6])
	}
}

func TestStreamEventsUsageIsOptionalAndNoCallStops(t *testing.T) {
	response, err := MapResponse("needle-2", native.Envelope{Type: "respond", Reasoning: "hidden"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	without := StreamEvents(response, false)
	with := StreamEvents(response, true)
	if len(without) != 3 || len(with) != 4 {
		t.Fatalf("without=%d with=%d", len(without), len(with))
	}
	for _, event := range without {
		if string(event.Data) == "[DONE]" {
			continue
		}
		if string(event.Data) == "" {
			t.Fatal("empty event")
		}
		if containsJSONText(event.Data, "hidden") {
			t.Fatalf("reasoning leaked as content: %s", event.Data)
		}
	}
}

func containsJSONText(data []byte, value string) bool {
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return false
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return false
	}
	choices, _ := object["choices"].([]any)
	for _, entry := range choices {
		choice, _ := entry.(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if delta["content"] == value {
			return true
		}
	}
	return false
}
