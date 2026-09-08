package native

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Engine struct {
	abi    ABI
	buffer []byte
}

func NewEngine(abi ABI, bufferSize int) *Engine {
	return &Engine{abi: abi, buffer: make([]byte, bufferSize)}
}

func (e *Engine) Execute(request Request) (Envelope, error) {
	e.abi.Reset()
	if code := e.abi.Init([]byte(request.System), request.ToolsJSON, []byte(request.ToolIndexPath)); code < 0 {
		return Envelope{}, &NativeError{Code: "native_init_failed", Message: fmt.Sprintf("needle_init failed with code %d", code), Fatal: true}
	}
	var result Envelope
	for _, turn := range request.Turns {
		e.buffer[0] = 0
		if code := e.abi.Complete([]byte(turn.Text), request.MaxNewTokens, e.buffer); code < 0 {
			e.abi.Reset()
			return Envelope{}, &NativeError{Code: "native_complete_failed", Message: fmt.Sprintf("needle_complete failed with code %d", code)}
		}
		end := bytes.IndexByte(e.buffer, 0)
		if end < 0 {
			e.abi.Reset()
			return Envelope{}, &NativeError{Code: "native_output_truncated", Message: "native output is not NUL terminated"}
		}
		if err := json.Unmarshal(e.buffer[:end], &result); err != nil {
			e.abi.Reset()
			return Envelope{}, &NativeError{Code: "native_output_invalid", Message: "native output is not valid JSON", Cause: err}
		}
		if err := validateEnvelope(result, request.ToolNames); err != nil {
			e.abi.Reset()
			return Envelope{}, &NativeError{Code: "native_output_invalid", Message: err.Error()}
		}
	}
	return result, nil
}

func validateEnvelope(envelope Envelope, toolNames []string) error {
	allowed := make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		allowed[name] = true
	}
	for _, call := range envelope.FunctionCalls {
		if !allowed[call.Name] {
			return fmt.Errorf("native output named an undeclared tool")
		}
		if call.Arguments == nil {
			return fmt.Errorf("native output tool arguments are invalid")
		}
	}
	return nil
}
