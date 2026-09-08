package openai

import "encoding/json"

type ChatCompletionRequest struct {
	Model               string          `json:"model"`
	Messages            []Message       `json:"messages"`
	Tools               []FunctionTool  `json:"tools"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
	MaxTokens           *int            `json:"max_tokens"`
	MaxCompletionTokens *int            `json:"max_completion_tokens"`
	Stream              bool            `json:"stream"`
	StreamOptions       *StreamOptions  `json:"stream_options"`
	ResponseFormat      json.RawMessage `json:"response_format"`
	N                   *int            `json:"n"`
	Temperature         *float64        `json:"temperature"`
	TopP                *float64        `json:"top_p"`
	Seed                *int64          `json:"seed"`
	Stop                json.RawMessage `json:"stop"`
}

type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type FunctionTool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type NativeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type SchemaLimits struct {
	MaxTools            int
	MaxCatalogBytes     int
	MaxDescriptionRunes int
	MaxDepth            int
	MaxProperties       int
}

func DefaultSchemaLimits() SchemaLimits {
	return SchemaLimits{MaxTools: 64, MaxCatalogBytes: 256 << 10, MaxDescriptionRunes: 1024, MaxDepth: 8, MaxProperties: 64}
}
