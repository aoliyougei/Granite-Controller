package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"needle-controller/internal/apierror"
	"needle-controller/internal/handler"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type streamService struct{ response openai.ChatCompletionResponse }

func (s *streamService) State() native.State                 { return native.StateReady }
func (s *streamService) ModelList() openai.ModelListResponse { return openai.ModelList("needle-2") }
func (s *streamService) Complete(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, *apierror.Error) {
	return s.response, nil
}

func TestHTTPStreamReassemblesToolCallAndEndsDone(t *testing.T) {
	confidence := 0.9
	response, err := openai.MapResponse("needle-2", native.Envelope{Type: "call", Confidence: &confidence, FunctionCalls: []native.FunctionCall{{Name: "weather", Arguments: map[string]json.RawMessage{"city": json.RawMessage(`"东京"`)}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler.ChatCompletions(&streamService{response: response}))
	defer server.Close()
	request, _ := http.NewRequest("POST", server.URL, strings.NewReader(`{"stream":true,"stream_options":{"include_usage":true}}`))
	request.Header.Set("Content-Type", "application/json")
	result, requestErr := http.DefaultClient.Do(request)
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	defer result.Body.Close()
	if !strings.HasPrefix(result.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type=%s", result.Header.Get("Content-Type"))
	}
	scanner := bufio.NewScanner(result.Body)
	var arguments strings.Builder
	done := false
	usage := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			done = true
			continue
		}
		var chunk map[string]any
		if json.Unmarshal([]byte(data), &chunk) != nil {
			t.Fatalf("chunk=%s", data)
		}
		if chunk["usage"] != nil {
			usage = true
		}
		choices, _ := chunk["choices"].([]any)
		if len(choices) > 0 {
			delta, _ := choices[0].(map[string]any)["delta"].(map[string]any)
			calls, _ := delta["tool_calls"].([]any)
			if len(calls) > 0 {
				function, _ := calls[0].(map[string]any)["function"].(map[string]any)
				if value, ok := function["arguments"].(string); ok {
					arguments.WriteString(value)
				}
			}
		}
	}
	if scanner.Err() != nil || !done || !usage || arguments.String() != `{"city":"东京"}` {
		t.Fatalf("done=%v usage=%v args=%q err=%v", done, usage, arguments.String(), scanner.Err())
	}
}
