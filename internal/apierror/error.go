package apierror

import "net/http"

type Error struct {
	Code       string
	Message    string
	Param      *string
	Kind       string
	HTTPStatus int
	Cause      error
}

type Response struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

func OpenAI(code, message, param string, status int, cause error) *Error {
	return OpenAIKind(code, message, param, "invalid_request_error", status, cause)
}

func OpenAIKind(code, message, param, kind string, status int, cause error) *Error {
	var parameter *string
	if param != "" {
		parameter = &param
	}
	return &Error{Code: code, Message: message, Param: parameter, Kind: kind, HTTPStatus: status, Cause: cause}
}

func (e *Error) Error() string     { return e.Code + ": " + e.Message }
func (e *Error) Unwrap() error     { return e.Cause }
func (e *Error) ErrorCode() string { return e.Code }
func (e *Error) Response() Response {
	return Response{Error: ErrorBody{Message: e.Message, Type: e.Kind, Param: e.Param, Code: e.Code}}
}

func InvalidRequest(cause error) *Error {
	return OpenAI("invalid_request_error", "Invalid request body.", "", http.StatusBadRequest, cause)
}
