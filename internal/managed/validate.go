package managed

import (
	"math"
	"needle-controller/internal/apierror"
	"needle-controller/internal/native"
	"net/http"
	"strconv"
	"strings"
)

type ValidatedCall struct {
	VMID       int64
	Confidence float64
	Validation native.Validation
}

func ValidateCall(command Command, envelope native.Envelope, minConfidence float64) (ValidatedCall, *apierror.Error) {
	if !envelope.Success || envelope.Type != "call" || len(envelope.FunctionCalls) != 1 {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle did not select the managed tool exactly once.")
	}
	call := envelope.FunctionCalls[0]
	if call.Name != "pve_vm_start" || len(call.Arguments) != 1 {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle returned an invalid managed tool call.")
	}
	raw, ok := call.Arguments["vmid"]
	if !ok {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle returned an invalid VM ID.")
	}
	text := string(raw)
	if strings.ContainsAny(text, ".eE\"") || text == "true" || text == "false" || text == "null" {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle returned an invalid VM ID.")
	}
	vmid, err := strconv.ParseInt(text, 10, 64)
	if err != nil || vmid <= 0 {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle returned an invalid VM ID.")
	}
	if vmid != command.VMID {
		return ValidatedCall{}, validationError("argument_mismatch", "Needle returned a VM ID that differs from the user request.")
	}
	confidence := envelope.Confidence
	if confidence == nil || math.IsNaN(*confidence) || math.IsInf(*confidence, 0) || *confidence < 0 || *confidence > 1 || *confidence < minConfidence {
		return ValidatedCall{}, validationError("low_confidence", "Needle confidence is too low for managed execution.")
	}
	if !envelope.Validation.UngroundedPresent || !envelope.Validation.NegationPresent {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle safety metadata is incomplete.")
	}
	if len(envelope.Validation.Ungrounded) != 0 {
		return ValidatedCall{}, validationError("ungrounded_arguments", "Needle returned ungrounded managed arguments.")
	}
	if envelope.Validation.Negation {
		return ValidatedCall{}, validationError("managed_tool_validation_failed", "Needle detected negation.")
	}
	return ValidatedCall{VMID: vmid, Confidence: *confidence, Validation: envelope.Validation}, nil
}
func validationError(code, message string) *apierror.Error {
	return apierror.OpenAI(code, message, "messages", http.StatusUnprocessableEntity, nil)
}
