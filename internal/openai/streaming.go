package openai

import "encoding/json"

type SSEEvent struct{ Data []byte }

type streamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []streamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
	XNeedle *XNeedle       `json:"x_needle,omitempty"`
}
type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}
type streamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   *string          `json:"content,omitempty"`
	ToolCalls []streamToolCall `json:"tool_calls,omitempty"`
}
type streamToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function streamFunction `json:"function"`
}
type streamFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func StreamEvents(response ChatCompletionResponse, includeUsage bool) []SSEEvent {
	base := func() streamChunk {
		return streamChunk{ID: response.ID, Object: "chat.completion.chunk", Created: response.Created, Model: response.Model}
	}
	var events []SSEEvent
	chunk := base()
	chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{Role: "assistant"}, FinishReason: nil}}
	events = appendJSONEvent(events, chunk)
	for index, call := range response.Choices[0].Message.ToolCalls {
		chunk = base()
		chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{ToolCalls: []streamToolCall{{Index: index, ID: call.ID, Type: "function", Function: streamFunction{Name: call.Function.Name}}}}, FinishReason: nil}}
		events = appendJSONEvent(events, chunk)
		chunk = base()
		chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{ToolCalls: []streamToolCall{{Index: index, Function: streamFunction{Arguments: call.Function.Arguments}}}}, FinishReason: nil}}
		events = appendJSONEvent(events, chunk)
	}
	finish := response.Choices[0].FinishReason
	chunk = base()
	chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{}, FinishReason: &finish}}
	chunk.XNeedle = &response.XNeedle
	events = appendJSONEvent(events, chunk)
	if includeUsage {
		chunk = base()
		chunk.Choices = []streamChoice{}
		chunk.Usage = &response.Usage
		events = appendJSONEvent(events, chunk)
	}
	return append(events, SSEEvent{Data: []byte("[DONE]")})
}

func appendJSONEvent(events []SSEEvent, value any) []SSEEvent {
	data, _ := json.Marshal(value)
	return append(events, SSEEvent{Data: data})
}
