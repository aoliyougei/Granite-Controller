package apierror

import "net/http"

const (
	CodeRequestInvalid              = "REQUEST_INVALID"
	CodeMessageInvalid              = "MESSAGE_INVALID"
	CodeAuthUnauthorized            = "AUTH_UNAUTHORIZED"
	CodeNeedleNoToolCall            = "NEEDLE_NO_TOOL_CALL"
	CodeNeedleMultipleToolCalls     = "NEEDLE_MULTIPLE_TOOL_CALLS"
	CodeNeedleToolNotAllowed        = "NEEDLE_TOOL_NOT_ALLOWED"
	CodeNeedleArgumentsInvalid      = "NEEDLE_ARGUMENTS_INVALID"
	CodeNeedleVMIDInvalid           = "NEEDLE_VMID_INVALID"
	CodeNeedleSafetyMetadataInvalid = "NEEDLE_SAFETY_METADATA_INVALID"
	CodeNeedleLowConfidence         = "NEEDLE_LOW_CONFIDENCE"
	CodeNeedleArgumentsUngrounded   = "NEEDLE_ARGUMENTS_UNGROUNDED"
	CodeNeedleNegationDetected      = "NEEDLE_NEGATION_DETECTED"
	CodeNeedleUnavailable           = "NEEDLE_UNAVAILABLE"
	CodeNeedleBusy                  = "NEEDLE_BUSY"
	CodeInfraControlAuthFailed      = "INFRA_CONTROL_AUTH_FAILED"
	CodeInfraControlRequestFailed   = "INFRA_CONTROL_REQUEST_FAILED"
	CodeInfraControlTimeout         = "INFRA_CONTROL_TIMEOUT"
	CodeInternal                    = "INTERNAL_ERROR"
)

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	Cause      error
}

func New(code, message string, status int, cause error) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: cause}
}

func (e *Error) Error() string     { return e.Code + ": " + e.Message }
func (e *Error) Unwrap() error     { return e.Cause }
func (e *Error) ErrorCode() string { return e.Code }

func RequestInvalid(cause error) *Error {
	return New(CodeRequestInvalid, "请求格式无效", http.StatusBadRequest, cause)
}
