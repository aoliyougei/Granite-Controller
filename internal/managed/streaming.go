package managed

import (
	"encoding/json"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

type SSEEvent struct{ Data []byte }
type streamChunk struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []streamChoice   `json:"choices"`
	Usage   *openai.Usage    `json:"usage,omitempty"`
	XNeedle *ManagedMetadata `json:"x_needle,omitempty"`
}
type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}
type streamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func StreamEvents(response ResponseBody, includeUsage bool) []SSEEvent {
	base := func() streamChunk {
		return streamChunk{ID: response.ID, Object: "chat.completion.chunk", Created: response.Created, Model: response.Model}
	}
	events := []SSEEvent{}
	chunk := base()
	chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{Role: "assistant"}}}
	events = appendEvent(events, chunk)
	chunk = base()
	chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{Content: *response.Choices[0].Message.Content}}}
	events = appendEvent(events, chunk)
	finish := "stop"
	chunk = base()
	chunk.Choices = []streamChoice{{Index: 0, Delta: streamDelta{}, FinishReason: &finish}}
	chunk.XNeedle = &response.XNeedle
	events = appendEvent(events, chunk)
	if includeUsage {
		chunk = base()
		chunk.Choices = []streamChoice{}
		chunk.Usage = &response.Usage
		events = appendEvent(events, chunk)
	}
	return append(events, SSEEvent{Data: []byte("[DONE]")})
}
func appendEvent(events []SSEEvent, value any) []SSEEvent {
	data, _ := json.Marshal(value)
	return append(events, SSEEvent{Data: data})
}
