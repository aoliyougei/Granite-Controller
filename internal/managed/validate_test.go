package managed

import (
	"encoding/json"
	"math"
	"needle-controller/internal/native"
	"testing"
)

func floatPointer(v float64) *float64 { return &v }
func validEnvelope() native.Envelope {
	return native.Envelope{Type: "call", Success: true, Confidence: floatPointer(0.92), Validation: native.Validation{Ungrounded: []string{}, Negation: false, UngroundedPresent: true, NegationPresent: true}, FunctionCalls: []native.FunctionCall{{Name: "pve_vm_start", Arguments: map[string]json.RawMessage{"vmid": json.RawMessage(`3052`)}}}}
}
func TestValidateCallAcceptsExactNeedleDecision(t *testing.T) {
	got, err := ValidateCall(Command{VMID: 3052}, validEnvelope(), 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if got.VMID != 3052 || got.Confidence != 0.92 || got.Validation.Negation {
		t.Fatalf("call=%+v", got)
	}
}
func TestValidateCallRejectsUnsafeNeedleResults(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*native.Envelope)
		code   string
	}{
		{"not success", func(e *native.Envelope) { e.Success = false }, "managed_tool_validation_failed"}, {"wrong type", func(e *native.Envelope) { e.Type = "respond" }, "managed_tool_validation_failed"}, {"no calls", func(e *native.Envelope) { e.FunctionCalls = nil }, "managed_tool_validation_failed"}, {"multiple", func(e *native.Envelope) { e.FunctionCalls = append(e.FunctionCalls, e.FunctionCalls[0]) }, "managed_tool_validation_failed"}, {"wrong name", func(e *native.Envelope) { e.FunctionCalls[0].Name = "other" }, "managed_tool_validation_failed"}, {"nil args", func(e *native.Envelope) { e.FunctionCalls[0].Arguments = nil }, "managed_tool_validation_failed"}, {"extra arg", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["url"] = json.RawMessage(`"x"`) }, "managed_tool_validation_failed"}, {"missing vmid", func(e *native.Envelope) { delete(e.FunctionCalls[0].Arguments, "vmid") }, "managed_tool_validation_failed"}, {"string vmid", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`"3052"`) }, "managed_tool_validation_failed"}, {"fraction", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`30.52`) }, "managed_tool_validation_failed"}, {"boolean", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`true`) }, "managed_tool_validation_failed"}, {"zero", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`0`) }, "managed_tool_validation_failed"}, {"negative", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`-1`) }, "managed_tool_validation_failed"}, {"overflow", func(e *native.Envelope) {
			e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`9223372036854775808`)
		}, "managed_tool_validation_failed"}, {"mismatch", func(e *native.Envelope) { e.FunctionCalls[0].Arguments["vmid"] = json.RawMessage(`3053`) }, "argument_mismatch"}, {"missing confidence", func(e *native.Envelope) { e.Confidence = nil }, "low_confidence"}, {"nan", func(e *native.Envelope) { e.Confidence = floatPointer(math.NaN()) }, "low_confidence"}, {"inf", func(e *native.Envelope) { e.Confidence = floatPointer(math.Inf(1)) }, "low_confidence"}, {"out range", func(e *native.Envelope) { e.Confidence = floatPointer(1.1) }, "low_confidence"}, {"below", func(e *native.Envelope) { e.Confidence = floatPointer(0.59) }, "low_confidence"}, {"missing ungrounded", func(e *native.Envelope) { e.Validation.UngroundedPresent = false }, "managed_tool_validation_failed"}, {"missing negation", func(e *native.Envelope) { e.Validation.NegationPresent = false }, "managed_tool_validation_failed"}, {"ungrounded", func(e *native.Envelope) { e.Validation.Ungrounded = []string{"pve_vm_start.vmid"} }, "ungrounded_arguments"}, {"negation", func(e *native.Envelope) { e.Validation.Negation = true }, "managed_tool_validation_failed"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelope()
			tc.mutate(&envelope)
			_, err := ValidateCall(Command{VMID: 3052}, envelope, 0.6)
			if err == nil || err.Code != tc.code {
				t.Fatalf("error=%+v", err)
			}
		})
	}
}
