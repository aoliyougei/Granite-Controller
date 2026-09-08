package managed

import (
	"encoding/json"
	"needle-controller/internal/openai"
	"strings"
	"testing"
)

func raw(v string) json.RawMessage { b, _ := json.Marshal(v); return b }
func TestExtractInputUsesOnlyFinalUserAndWarnsByCategory(t *testing.T) {
	temperature := 0.7
	req := openai.ChatCompletionRequest{Model: "needle-2", Messages: []openai.Message{{Role: "system", Content: raw("secret system")}, {Role: "user", Content: raw("启动 VM 1111")}, {Role: "assistant", Content: raw("history")}, {Role: "tool", Content: raw("result")}, {Role: "user", Content: raw(" 开启 VM 3052 ")}}, Tools: []openai.FunctionTool{{Type: "bad", Function: openai.FunctionDefinition{Name: "agent_private_tool_xyz", Parameters: json.RawMessage(`{`)}}}, ToolChoice: json.RawMessage(`"required"`), ResponseFormat: json.RawMessage(`{"type":"json_object"}`), Temperature: &temperature}
	got, err := ExtractInput(req, 512)
	if err != nil {
		t.Fatal(err)
	}
	if got.Original != "开启 VM 3052" {
		t.Fatalf("input=%+v", got)
	}
	joined := strings.Join(got.Warnings, "|")
	for _, category := range []string{"earlier messages", "client-provided tools", "tool_choice", "response_format", "temperature"} {
		if !strings.Contains(joined, category) {
			t.Fatalf("warnings=%v", got.Warnings)
		}
	}
	for _, secret := range []string{"1111", "secret system", "agent_private_tool_xyz"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("warning leaked ignored content: %v", got.Warnings)
		}
	}
}
func TestExtractInputAcceptsAndJoinsPlainTextParts(t *testing.T) {
	req := openai.ChatCompletionRequest{Messages: []openai.Message{{Role: "user", Content: json.RawMessage(`[{"type":"text","text":"开启 VM "},{"type":"text","text":"3052"}]`)}}}
	got, err := ExtractInput(req, 512)
	if err != nil || got.Original != "开启 VM 3052" {
		t.Fatalf("input=%+v error=%v", got, err)
	}
}

func TestExtractInputRejectsInvalidFinalMessageWithoutFallback(t *testing.T) {
	tests := []openai.Message{{Role: "assistant", Content: raw("x")}, {Role: "tool", Content: raw("x")}, {Role: "user", Content: nil}, {Role: "user", Content: raw("  ")}, {Role: "user", Content: json.RawMessage(`{"text":"x"}`)}, {Role: "user", Content: json.RawMessage(`[]`)}, {Role: "user", Content: json.RawMessage(`[{"type":"image_url","image_url":{"url":"x"}}]`)}, {Role: "user", Content: json.RawMessage(`[{"type":"audio","data":"x"}]`)}, {Role: "user", Content: json.RawMessage(`[{"type":"file","file_id":"x"}]`)}, {Role: "user", Content: json.RawMessage(`[{"type":"unknown","text":"x"}]`)}, {Role: "user", Content: json.RawMessage(`[{"type":"text","text":""}]`)}}
	for _, last := range tests {
		req := openai.ChatCompletionRequest{Model: "needle-2", Messages: []openai.Message{{Role: "user", Content: raw("开启 VM 3052")}, last}}
		_, err := ExtractInput(req, 512)
		if err == nil || err.Code != "user_message_required" {
			t.Fatalf("last=%s error=%+v", last.Role, err)
		}
	}
}
func TestExtractInputRejectsMissingAndOverlongFinalUser(t *testing.T) {
	if _, err := ExtractInput(openai.ChatCompletionRequest{}, 2); err == nil || err.Code != "user_message_required" {
		t.Fatalf("missing error=%+v", err)
	}
	req := openai.ChatCompletionRequest{Messages: []openai.Message{{Role: "user", Content: raw("开启机")}}}
	if _, err := ExtractInput(req, 2); err == nil || err.Code != "user_message_required" {
		t.Fatalf("length error=%+v", err)
	}
}
