package openai

import (
	"encoding/json"
	"testing"

	"needle-controller/internal/native"
)

func nativeCallEnvelope() native.Envelope {
	confidence := 0.12
	return native.Envelope{
		Type: "call", Success: true, Reasoning: "route tools", Confidence: &confidence,
		Validation: native.Validation{Ungrounded: []string{"second.arg"}, Negation: true},
		PrefillTPS: 10.5, DecodeTPS: 20.5, PeakRAMMB: 30.5,
		FunctionCalls: []native.FunctionCall{
			{Name: "first", Arguments: map[string]json.RawMessage{"city": json.RawMessage(`"Lagos"`)}},
			{Name: "second", Arguments: map[string]json.RawMessage{"count": json.RawMessage(`2`)}},
		},
	}
}

func TestModelList(t *testing.T) {
	got := ModelList("needle-2")
	if got.Object != "list" || len(got.Data) != 1 || got.Data[0].ID != "needle-2" || got.Data[0].Object != "model" || got.Data[0].OwnedBy != "cactus-compute" {
		t.Fatalf("model list = %+v", got)
	}
}

func TestResponseMapsNativeToolCallsAndSafetySignals(t *testing.T) {
	got, err := MapResponse("needle-2", nativeCallEnvelope(), []string{"required warning", "temperature warning"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Object != "chat.completion" || got.Model != "needle-2" || got.Created == 0 || len(got.Choices) != 1 {
		t.Fatalf("response = %+v", got)
	}
	choice := got.Choices[0]
	if choice.FinishReason != "tool_calls" || choice.Message.Role != "assistant" || choice.Message.Content != nil || len(choice.Message.ToolCalls) != 2 {
		t.Fatalf("choice = %+v", choice)
	}
	if choice.Message.ToolCalls[0].Function.Name != "first" || choice.Message.ToolCalls[0].Function.Arguments != `{"city":"Lagos"}` || choice.Message.ToolCalls[0].ID == choice.Message.ToolCalls[1].ID {
		t.Fatalf("calls = %+v", choice.Message.ToolCalls)
	}
	if got.Usage.TotalTokens != 0 || !got.Usage.Estimated || got.XNeedle.Confidence == nil || *got.XNeedle.Confidence != 0.12 || !got.XNeedle.Validation.Negation || len(got.XNeedle.Validation.Ungrounded) != 1 || len(got.XNeedle.Warnings) != 2 {
		t.Fatalf("metadata = usage:%+v needle:%+v", got.Usage, got.XNeedle)
	}
}

func TestResponseDoesNotPromoteReasoningWhenNoToolSelected(t *testing.T) {
	confidence := 0.8
	got, err := MapResponse("needle-2", native.Envelope{Type: "respond", Success: true, Reasoning: "internal only", Confidence: &confidence, Validation: native.Validation{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	choice := got.Choices[0]
	if choice.FinishReason != "stop" || choice.Message.Content != nil || len(choice.Message.ToolCalls) != 0 || got.XNeedle.Reasoning != "internal only" {
		t.Fatalf("response = %+v", got)
	}
}

func TestResponseRejectsInvalidNativeArguments(t *testing.T) {
	envelope := nativeCallEnvelope()
	envelope.FunctionCalls[0].Arguments["bad"] = json.RawMessage(`{`)
	if _, err := MapResponse("needle-2", envelope, nil); err == nil || err.Code != "engine_error" {
		t.Fatalf("error = %+v", err)
	}
}
