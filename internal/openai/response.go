package openai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"needle-controller/internal/apierror"
	"needle-controller/internal/native"
)

type ModelListResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	XNeedle XNeedle  `json:"x_needle"`
}
type Choice struct {
	Index        int              `json:"index"`
	Message      AssistantMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}
type AssistantMessage struct {
	Role      string             `json:"role"`
	Content   *string            `json:"content"`
	ToolCalls []ToolCallResponse `json:"tool_calls,omitempty"`
}
type ToolCallResponse struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function FunctionCallResponse `json:"function"`
}
type FunctionCallResponse struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type Usage struct {
	PromptTokens     int  `json:"prompt_tokens"`
	CompletionTokens int  `json:"completion_tokens"`
	TotalTokens      int  `json:"total_tokens"`
	Estimated        bool `json:"estimated"`
}
type XNeedle struct {
	Type       string            `json:"type"`
	Confidence *float64          `json:"confidence"`
	Reasoning  string            `json:"reasoning"`
	Validation native.Validation `json:"validation"`
	PrefillTPS float64           `json:"prefill_tps"`
	DecodeTPS  float64           `json:"decode_tps"`
	PeakRAMMB  float64           `json:"peak_ram_mb"`
	Warnings   []string          `json:"warnings"`
}

func ModelList(modelID string) ModelListResponse {
	return ModelListResponse{Object: "list", Data: []Model{{ID: modelID, Object: "model", Created: 0, OwnedBy: "cactus-compute"}}}
}

func MapResponse(modelID string, envelope native.Envelope, warnings []string) (ChatCompletionResponse, *apierror.Error) {
	if warnings == nil {
		warnings = []string{}
	}
	calls := make([]ToolCallResponse, 0, len(envelope.FunctionCalls))
	for _, call := range envelope.FunctionCalls {
		for _, raw := range call.Arguments {
			if !json.Valid(raw) {
				return ChatCompletionResponse{}, apierror.OpenAI("engine_error", "Native engine returned invalid tool arguments.", "", http.StatusBadGateway, nil)
			}
		}
		arguments, err := json.Marshal(call.Arguments)
		if err != nil {
			return ChatCompletionResponse{}, apierror.OpenAI("engine_error", "Native engine returned invalid tool arguments.", "", http.StatusBadGateway, err)
		}
		calls = append(calls, ToolCallResponse{ID: newID("call"), Type: "function", Function: FunctionCallResponse{Name: call.Name, Arguments: string(arguments)}})
	}
	finish := "stop"
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	return ChatCompletionResponse{ID: newID("chatcmpl"), Object: "chat.completion", Created: time.Now().Unix(), Model: modelID, Choices: []Choice{{Index: 0, Message: AssistantMessage{Role: "assistant", Content: nil, ToolCalls: calls}, FinishReason: finish}}, Usage: Usage{Estimated: true}, XNeedle: XNeedle{Type: envelope.Type, Confidence: envelope.Confidence, Reasoning: envelope.Reasoning, Validation: envelope.Validation, PrefillTPS: envelope.PrefillTPS, DecodeTPS: envelope.DecodeTPS, PeakRAMMB: envelope.PeakRAMMB, Warnings: warnings}}, nil
}

func newID(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "_000000000000000000000000"
	}
	return prefix + "_" + hex.EncodeToString(buffer)
}
