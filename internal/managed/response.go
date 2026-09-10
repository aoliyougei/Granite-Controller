package managed

import (
	"encoding/json"
	"time"

	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

type ResponseBody struct {
	ID       string          `json:"id"`
	Object   string          `json:"object"`
	Created  int64           `json:"created"`
	Model    string          `json:"model"`
	Choices  []Choice        `json:"choices"`
	Usage    openai.Usage    `json:"usage"`
	XGranite GraniteMetadata `json:"x_granite"`
}
type Choice struct {
	Index        int              `json:"index"`
	Message      AssistantMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}
type AssistantMessage struct {
	Role    string  `json:"role"`
	Content *string `json:"content"`
}
type Arguments struct {
	VMID int64 `json:"vmid"`
}
type GraniteMetadata struct {
	Tool           string           `json:"tool"`
	Arguments      Arguments        `json:"arguments"`
	Executed       bool             `json:"executed"`
	Deduplicated   bool             `json:"deduplicated"`
	UpstreamStatus int              `json:"upstream_status,omitempty"`
	Result         *infracontrol.VM `json:"result,omitempty"`
	Warnings       []string         `json:"warnings"`
}
type MetadataResult struct {
	Executed, Deduplicated bool
	UpstreamStatus         int
	Result                 *infracontrol.VM
}

func NewResponse(model string, call ActionCall, content string, result MetadataResult, warnings []string) ResponseBody {
	if warnings == nil {
		warnings = []string{}
	}
	meta, _ := Metadata(call.Action)
	return ResponseBody{newID(), "chat.completion", time.Now().Unix(), model, []Choice{{0, AssistantMessage{"assistant", &content}, "stop"}}, openai.Usage{Estimated: true}, GraniteMetadata{meta.ToolName, Arguments{call.VMID}, result.Executed, result.Deduplicated, result.UpstreamStatus, result.Result, warnings}}
}

type StreamEvent struct{ Data string }

func StreamEvents(response ResponseBody, includeUsage bool) []StreamEvent {
	content := *response.Choices[0].Message.Content
	meta, _ := json.Marshal(response.XGranite)
	events := []StreamEvent{{`{"choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}`}, {string(mustJSON(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": content}, "finish_reason": nil}}}))}, {string(mustJSON(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{}, "finish_reason": "stop"}}, "x_granite": json.RawMessage(meta)}))}}
	if includeUsage {
		events = append(events, StreamEvent{string(mustJSON(map[string]any{"choices": []any{}, "usage": response.Usage}))})
	}
	return append(events, StreamEvent{"[DONE]"})
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
