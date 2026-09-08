package e2e

import (
	"context"
	"encoding/json"
	"needle-controller/internal/config"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"sync"
	"testing"
)

type recordingCompleter struct {
	mu        sync.Mutex
	requests  []native.Request
	responses []native.Envelope
}

func (r *recordingCompleter) State() native.State { return native.StateReady }
func (r *recordingCompleter) Submit(_ context.Context, request native.Request) (native.Envelope, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request)
	response := r.responses[0]
	r.responses = r.responses[1:]
	return response, nil
}

func TestOpenAIAgentLoopReplaysToolResultsWithoutExecutingTools(t *testing.T) {
	confidence := 0.9
	recorder := &recordingCompleter{responses: []native.Envelope{
		{Type: "call", Confidence: &confidence, FunctionCalls: []native.FunctionCall{{Name: "get_weather", Arguments: map[string]json.RawMessage{"city": json.RawMessage(`"Lagos"`)}}}},
		{Type: "respond", Confidence: &confidence, FunctionCalls: []native.FunctionCall{}, Reasoning: "done"},
	}}
	cfg := config.NativeConfig{ModelID: "needle-2", MaxNewTokens: 256, MaxReplaySteps: 32}
	service := openai.NewService(recorder, cfg, openai.DefaultSchemaLimits())
	tool := openai.FunctionTool{Type: "function", Function: openai.FunctionDefinition{Name: "get_weather", Parameters: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`)}}
	first := openai.ChatCompletionRequest{Model: "needle-2", Messages: []openai.Message{{Role: "user", Content: json.RawMessage(`"weather in Lagos"`)}}, Tools: []openai.FunctionTool{tool}}
	result, err := service.Complete(context.Background(), first)
	if err != nil || len(result.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	second := first
	second.Messages = append([]openai.Message{}, first.Messages...)
	second.Messages = append(second.Messages, openai.Message{Role: "assistant", Content: json.RawMessage(`null`)}, openai.Message{Role: "tool", ToolCallID: result.Choices[0].Message.ToolCalls[0].ID, Content: json.RawMessage(`"{\"city\":\"Lagos\",\"temp_c\":27}"`)})
	final, err := service.Complete(context.Background(), second)
	if err != nil || final.Choices[0].FinishReason != "stop" {
		t.Fatalf("final=%+v err=%v", final, err)
	}
	if len(recorder.requests) != 2 || len(recorder.requests[0].Turns) != 1 || len(recorder.requests[1].Turns) != 2 || recorder.requests[1].Turns[1].Kind != native.TurnToolResults || recorder.requests[1].Turns[1].Text != `[{"city":"Lagos","temp_c":27}]` {
		t.Fatalf("requests=%+v", recorder.requests)
	}
}

func TestSeparateAgentsCarrySeparateToolsAndHistory(t *testing.T) {
	confidence := 0.8
	recorder := &recordingCompleter{responses: []native.Envelope{{Type: "respond", Confidence: &confidence}, {Type: "respond", Confidence: &confidence}}}
	service := openai.NewService(recorder, config.NativeConfig{ModelID: "needle-2", MaxNewTokens: 256, MaxReplaySteps: 32}, openai.DefaultSchemaLimits())
	for _, name := range []string{"weather", "calendar"} {
		request := openai.ChatCompletionRequest{Model: "needle-2", Messages: []openai.Message{{Role: "user", Content: json.RawMessage(`"query"`)}}, Tools: []openai.FunctionTool{{Type: "function", Function: openai.FunctionDefinition{Name: name, Parameters: json.RawMessage(`{"type":"object"}`)}}}}
		if _, err := service.Complete(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if len(recorder.requests) != 2 || string(recorder.requests[0].ToolsJSON) == string(recorder.requests[1].ToolsJSON) || recorder.requests[0].ToolNames[0] != "weather" || recorder.requests[1].ToolNames[0] != "calendar" {
		t.Fatalf("requests=%+v", recorder.requests)
	}
}
