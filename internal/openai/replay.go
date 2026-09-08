package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
	"needle-controller/internal/native"
)

func BuildReplay(req ChatCompletionRequest, effectiveTools []NativeTool, cfg config.NativeConfig) (native.Request, []string, *apierror.Error) {
	if req.Model != cfg.ModelID {
		return native.Request{}, nil, apierror.OpenAI("model_not_found", "The requested model does not exist.", "model", http.StatusNotFound, nil)
	}
	if len(req.Messages) == 0 {
		return native.Request{}, nil, messageError("invalid_messages", "messages must not be empty")
	}
	if len(bytes.TrimSpace(req.ResponseFormat)) != 0 && !bytes.Equal(bytes.TrimSpace(req.ResponseFormat), []byte("null")) {
		return native.Request{}, nil, apierror.OpenAI("response_format_not_supported", "response_format is not supported", "response_format", http.StatusBadRequest, nil)
	}
	if req.N != nil && *req.N > 1 {
		return native.Request{}, nil, apierror.OpenAI("invalid_request_error", "n must be 1", "n", http.StatusBadRequest, nil)
	}
	if req.MaxTokens != nil && req.MaxCompletionTokens != nil {
		return native.Request{}, nil, apierror.OpenAI("invalid_request_error", "max_tokens and max_completion_tokens cannot both be set", "max_tokens", http.StatusBadRequest, nil)
	}
	maxTokens := cfg.MaxNewTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if req.MaxCompletionTokens != nil {
		maxTokens = *req.MaxCompletionTokens
	}
	if maxTokens <= 0 || maxTokens > cfg.MaxNewTokens {
		return native.Request{}, nil, apierror.OpenAI("invalid_request_error", "token budget is invalid", "max_tokens", http.StatusBadRequest, nil)
	}

	var facts []string
	var turns []native.Turn
	var pendingTools []json.RawMessage
	flushTools := func() *apierror.Error {
		if len(pendingTools) == 0 {
			return nil
		}
		data, err := json.Marshal(pendingTools)
		if err != nil {
			return messageError("invalid_messages", "tool results are invalid")
		}
		turns = append(turns, native.Turn{Kind: native.TurnToolResults, Text: string(data)})
		pendingTools = nil
		return nil
	}
	for _, message := range req.Messages {
		switch message.Role {
		case "system", "developer":
			if err := flushTools(); err != nil {
				return native.Request{}, nil, err
			}
			value, err := flattenText(message.Content)
			if err != nil {
				return native.Request{}, nil, messageError("invalid_messages", err.Error())
			}
			if value != "" {
				facts = append(facts, value)
			}
		case "user":
			if err := flushTools(); err != nil {
				return native.Request{}, nil, err
			}
			value, err := flattenText(message.Content)
			if err != nil || value == "" {
				return native.Request{}, nil, messageError("invalid_messages", "user content must be non-empty text")
			}
			turns = append(turns, native.Turn{Kind: native.TurnUser, Text: value})
		case "tool", "function":
			value, err := flattenText(message.Content)
			if err != nil || value == "" {
				return native.Request{}, nil, messageError("invalid_messages", "tool content must be text")
			}
			var raw json.RawMessage
			if json.Unmarshal([]byte(value), &raw) != nil {
				raw, _ = json.Marshal(value)
			}
			pendingTools = append(pendingTools, raw)
		case "assistant":
			if err := flushTools(); err != nil {
				return native.Request{}, nil, err
			}
		default:
			return native.Request{}, nil, messageError("invalid_messages", "unsupported message role")
		}
	}
	if err := flushTools(); err != nil {
		return native.Request{}, nil, err
	}
	if len(turns) == 0 {
		return native.Request{}, nil, messageError("invalid_messages", "conversation has no user or tool turn")
	}
	if len(turns) > cfg.MaxReplaySteps {
		return native.Request{}, nil, apierror.OpenAI("replay_limit_exceeded", "conversation exceeds replay limit", "messages", http.StatusBadRequest, nil)
	}
	toolsJSON, err := json.Marshal(effectiveTools)
	if err != nil {
		return native.Request{}, nil, apierror.InvalidRequest(err)
	}
	names := make([]string, len(effectiveTools))
	for i, tool := range effectiveTools {
		names[i] = tool.Name
	}
	return native.Request{System: strings.Join(facts, "\n"), ToolsJSON: toolsJSON, ToolNames: names, Turns: turns, MaxNewTokens: maxTokens, ToolIndexPath: cfg.ToolIndexPath}, ignoredWarnings(req), nil
}

func flattenText(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("message content must be text")
	}
	var out strings.Builder
	for _, part := range parts {
		if part.Type != "text" {
			return "", fmt.Errorf("only text content is supported")
		}
		out.WriteString(part.Text)
	}
	return out.String(), nil
}

func ignoredWarnings(req ChatCompletionRequest) []string {
	var warnings []string
	if req.Temperature != nil {
		warnings = append(warnings, "'temperature' ignored: Needle decodes deterministically")
	}
	if req.TopP != nil {
		warnings = append(warnings, "'top_p' ignored: Needle decodes deterministically")
	}
	if req.Seed != nil {
		warnings = append(warnings, "'seed' ignored: Needle decodes deterministically")
	}
	if len(bytes.TrimSpace(req.Stop)) > 0 && !bytes.Equal(bytes.TrimSpace(req.Stop), []byte("null")) {
		warnings = append(warnings, "'stop' ignored: Needle exposes no stop control")
	}
	return warnings
}

func messageError(code, message string) *apierror.Error {
	return apierror.OpenAI(code, message, "messages", http.StatusBadRequest, nil)
}
