package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"needle-controller/internal/config"
	"needle-controller/internal/native"
)

func text(value string) json.RawMessage { data, _ := json.Marshal(value); return data }

func replayConfig() config.NativeConfig {
	return config.NativeConfig{ModelID: "needle-2", MaxNewTokens: 256, MaxReplaySteps: 32, ToolIndexPath: "/tmp/index"}
}

func TestBuildReplayJoinsFactsAndReplaysTurns(t *testing.T) {
	req := ChatCompletionRequest{
		Model: "needle-2",
		Messages: []Message{
			{Role: "system", Content: text("date: 2026-09-08")},
			{Role: "developer", Content: text("device: server")},
			{Role: "user", Content: text("Find weather")},
			{Role: "assistant", Content: text("ignored")},
			{Role: "tool", ToolCallID: "a", Content: text(`{"city":"Lagos"}`)},
			{Role: "tool", ToolCallID: "b", Content: text(`{"temp":27}`)},
			{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"next"},{"type":"text","text":" turn"}]`)},
		},
	}
	tools := []NativeTool{{Name: "weather", Parameters: json.RawMessage(`{"type":"object"}`)}}
	got, warnings, err := BuildReplay(req, tools, replayConfig())
	if err != nil || len(warnings) != 0 {
		t.Fatalf("warnings=%v err=%v", warnings, err)
	}
	if got.System != "date: 2026-09-08\ndevice: server" || got.MaxNewTokens != 256 || got.ToolIndexPath != "/tmp/index" {
		t.Fatalf("request=%+v", got)
	}
	if len(got.Turns) != 3 || got.Turns[0] != (native.Turn{Kind: native.TurnUser, Text: "Find weather"}) || got.Turns[1].Kind != native.TurnToolResults || got.Turns[1].Text != `[{"city":"Lagos"},{"temp":27}]` || got.Turns[2].Text != "next turn" {
		t.Fatalf("turns=%+v", got.Turns)
	}
	if len(got.ToolNames) != 1 || got.ToolNames[0] != "weather" || len(got.ToolsJSON) == 0 {
		t.Fatalf("tools=%s names=%v", got.ToolsJSON, got.ToolNames)
	}
}

func TestBuildReplayRejectsInvalidMessagesAndOptions(t *testing.T) {
	newRequest := func() ChatCompletionRequest {
		return ChatCompletionRequest{Model: "needle-2", Messages: []Message{{Role: "user", Content: text("hello")}}}
	}
	tools := []NativeTool{{Name: "x", Parameters: json.RawMessage(`{"type":"object"}`)}}
	tests := []struct {
		name   string
		mutate func(*ChatCompletionRequest, *config.NativeConfig)
		code   string
	}{
		{"wrong model", func(r *ChatCompletionRequest, _ *config.NativeConfig) { r.Model = "other" }, "model_not_found"},
		{"empty messages", func(r *ChatCompletionRequest, _ *config.NativeConfig) { r.Messages = nil }, "invalid_messages"},
		{"unknown role", func(r *ChatCompletionRequest, _ *config.NativeConfig) { r.Messages[0].Role = "owner" }, "invalid_messages"},
		{"image", func(r *ChatCompletionRequest, _ *config.NativeConfig) {
			r.Messages[0].Content = json.RawMessage(`[{"type":"image_url","image_url":{"url":"x"}}]`)
		}, "invalid_messages"},
		{"tool object", func(r *ChatCompletionRequest, _ *config.NativeConfig) {
			r.Messages[0] = Message{Role: "tool", Content: json.RawMessage(`{"x":1}`)}
		}, "invalid_messages"},
		{"only assistant", func(r *ChatCompletionRequest, _ *config.NativeConfig) {
			r.Messages[0] = Message{Role: "assistant", Content: text("ignored")}
		}, "invalid_messages"},
		{"response format", func(r *ChatCompletionRequest, _ *config.NativeConfig) {
			r.ResponseFormat = json.RawMessage(`{"type":"json_object"}`)
		}, "response_format_not_supported"},
		{"n greater one", func(r *ChatCompletionRequest, _ *config.NativeConfig) { v := 2; r.N = &v }, "invalid_request_error"},
		{"token conflict", func(r *ChatCompletionRequest, _ *config.NativeConfig) {
			a, b := 1, 2
			r.MaxTokens = &a
			r.MaxCompletionTokens = &b
		}, "invalid_request_error"},
		{"replay limit", func(r *ChatCompletionRequest, c *config.NativeConfig) {
			r.Messages = append(r.Messages, Message{Role: "user", Content: text("again")})
			c.MaxReplaySteps = 1
		}, "replay_limit_exceeded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRequest()
			c := replayConfig()
			tc.mutate(&r, &c)
			_, _, err := BuildReplay(r, tools, c)
			if err == nil || err.Code != tc.code {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}

func TestBuildReplayUsesTokenAliasAndReportsIgnoredSampling(t *testing.T) {
	budget := 42
	temperature := 0.7
	topP := 0.9
	seed := int64(7)
	req := ChatCompletionRequest{Model: "needle-2", Messages: []Message{{Role: "user", Content: text("hello")}}, MaxCompletionTokens: &budget, Temperature: &temperature, TopP: &topP, Seed: &seed, Stop: json.RawMessage(`"END"`)}
	got, warnings, err := BuildReplay(req, []NativeTool{{Name: "x", Parameters: json.RawMessage(`{"type":"object"}`)}}, replayConfig())
	if err != nil || got.MaxNewTokens != 42 {
		t.Fatalf("request=%+v err=%v", got, err)
	}
	joined := strings.Join(warnings, " ")
	for _, name := range []string{"temperature", "top_p", "seed", "stop"} {
		if !strings.Contains(joined, name) {
			t.Fatalf("warnings=%v", warnings)
		}
	}
}
