package managed

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

type Input struct {
	Original    string
	VMID        int64
	HasNegation bool
	Warnings    []string
}

func ExtractInput(req openai.ChatCompletionRequest, maxRunes int) (Input, *apierror.Error) {
	if len(req.Messages) == 0 {
		return Input{}, userMessageError()
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" {
		return Input{}, userMessageError()
	}
	value, ok := finalUserText(last.Content)
	if !ok {
		return Input{}, userMessageError()
	}
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxRunes {
		return Input{}, userMessageError()
	}
	warnings := []string{}
	if len(req.Messages) > 1 {
		warnings = append(warnings, "earlier messages were ignored; only the final user message is processed")
	}
	if len(req.Tools) > 0 {
		warnings = append(warnings, "client-provided tools were ignored; the managed tool catalog is fixed")
	}
	if len(bytes.TrimSpace(req.ToolChoice)) > 0 {
		warnings = append(warnings, "tool_choice was ignored; the managed tool catalog is fixed")
	}
	if len(bytes.TrimSpace(req.ResponseFormat)) > 0 {
		warnings = append(warnings, "response_format was ignored in managed mode")
	}
	if req.Temperature != nil {
		warnings = append(warnings, "temperature was ignored in managed mode")
	}
	if req.TopP != nil {
		warnings = append(warnings, "top_p was ignored in managed mode")
	}
	if req.Seed != nil {
		warnings = append(warnings, "seed was ignored in managed mode")
	}
	if len(bytes.TrimSpace(req.Stop)) > 0 {
		warnings = append(warnings, "stop was ignored in managed mode")
	}
	return Input{Original: value, Warnings: warnings}, nil
}
func finalUserText(content json.RawMessage) (string, bool) {
	if len(bytes.TrimSpace(content)) == 0 {
		return "", false
	}
	var value string
	if json.Unmarshal(content, &value) == nil {
		return value, true
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&parts) != nil || len(parts) == 0 {
		return "", false
	}
	var joined strings.Builder
	for _, part := range parts {
		if part.Type != "text" || part.Text == "" {
			return "", false
		}
		joined.WriteString(part.Text)
	}
	return joined.String(), true
}

func userMessageError() *apierror.Error {
	return apierror.OpenAI("user_message_required", "A non-empty final user text message is required.", "messages", http.StatusBadRequest, nil)
}
