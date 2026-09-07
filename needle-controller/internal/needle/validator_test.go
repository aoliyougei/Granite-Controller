package needle

import (
	"encoding/json"
	"math"
	"testing"
)

func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }

func validCompletion(arguments string) Completion {
	return Completion{
		ToolCalls: []ToolCall{{ID: "call-1", Type: "function", Function: FunctionCall{Name: "pve_vm_start", Arguments: json.RawMessage(arguments)}}},
		Safety:    SafetyMetadata{Confidence: floatPtr(0.92), Validation: Validation{Ungrounded: json.RawMessage(`[]`), Negation: boolPtr(false)}},
	}
}

func TestValidateStartVMAcceptsStrictCommand(t *testing.T) {
	got, err := ValidateStartVM(validCompletion(`{"vmid":3052}`), 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if got.VMID != 3052 || got.Confidence != 0.92 || got.ToolCallID != "call-1" {
		t.Fatalf("command = %+v", got)
	}
}

func TestValidateStartVMRejectsInvalidCallStructure(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Completion)
		code   string
	}{
		{"no call", func(c *Completion) { c.ToolCalls = nil }, "NEEDLE_NO_TOOL_CALL"},
		{"multiple calls", func(c *Completion) { c.ToolCalls = append(c.ToolCalls, c.ToolCalls[0]) }, "NEEDLE_MULTIPLE_TOOL_CALLS"},
		{"wrong type", func(c *Completion) { c.ToolCalls[0].Type = "custom" }, "NEEDLE_TOOL_NOT_ALLOWED"},
		{"wrong function", func(c *Completion) { c.ToolCalls[0].Function.Name = "http_request" }, "NEEDLE_TOOL_NOT_ALLOWED"},
		{"malformed", func(c *Completion) { c.ToolCalls[0].Function.Arguments = json.RawMessage(`{`) }, "NEEDLE_ARGUMENTS_INVALID"},
		{"missing vmid", func(c *Completion) { c.ToolCalls[0].Function.Arguments = json.RawMessage(`{}`) }, "NEEDLE_VMID_INVALID"},
		{"extra field", func(c *Completion) {
			c.ToolCalls[0].Function.Arguments = json.RawMessage(`{"vmid":3052,"url":"http://bad"}`)
		}, "NEEDLE_ARGUMENTS_INVALID"},
		{"trailing value", func(c *Completion) { c.ToolCalls[0].Function.Arguments = json.RawMessage(`{"vmid":3052}{}`) }, "NEEDLE_ARGUMENTS_INVALID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCompletion(`{"vmid":3052}`)
			tt.mutate(&c)
			_, err := ValidateStartVM(c, 0.6)
			assertErrorCode(t, err, tt.code)
		})
	}
}

func TestValidateStartVMRejectsInvalidVMID(t *testing.T) {
	for _, arguments := range []string{
		`{"vmid":"3052"}`, `{"vmid":true}`, `{"vmid":30.52}`, `{"vmid":0}`,
		`{"vmid":-1}`, `{"vmid":3e-1}`, `{"vmid":9223372036854775808}`,
		`{"vmid":3052,"vmid":3053}`,
	} {
		t.Run(arguments, func(t *testing.T) {
			_, err := ValidateStartVM(validCompletion(arguments), 0.6)
			assertErrorCode(t, err, "NEEDLE_VMID_INVALID")
		})
	}
}

func TestValidateStartVMRejectsUnsafeMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Completion)
		code   string
	}{
		{"missing confidence", func(c *Completion) { c.Safety.Confidence = nil }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"nan confidence", func(c *Completion) { c.Safety.Confidence = floatPtr(math.NaN()) }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"negative confidence", func(c *Completion) { c.Safety.Confidence = floatPtr(-0.1) }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"high confidence", func(c *Completion) { c.Safety.Confidence = floatPtr(1.1) }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"low confidence", func(c *Completion) { c.Safety.Confidence = floatPtr(0.59) }, "NEEDLE_LOW_CONFIDENCE"},
		{"missing ungrounded", func(c *Completion) { c.Safety.Validation.Ungrounded = nil }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"invalid ungrounded", func(c *Completion) { c.Safety.Validation.Ungrounded = json.RawMessage(`{}`) }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"nonempty ungrounded", func(c *Completion) { c.Safety.Validation.Ungrounded = json.RawMessage(`["vmid"]`) }, "NEEDLE_ARGUMENTS_UNGROUNDED"},
		{"missing negation", func(c *Completion) { c.Safety.Validation.Negation = nil }, "NEEDLE_SAFETY_METADATA_INVALID"},
		{"negation", func(c *Completion) { c.Safety.Validation.Negation = boolPtr(true) }, "NEEDLE_NEGATION_DETECTED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validCompletion(`{"vmid":3052}`)
			tt.mutate(&c)
			_, err := ValidateStartVM(c, 0.6)
			assertErrorCode(t, err, tt.code)
		})
	}
}

func assertErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	typed, ok := err.(interface{ ErrorCode() string })
	if !ok || typed.ErrorCode() != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}
