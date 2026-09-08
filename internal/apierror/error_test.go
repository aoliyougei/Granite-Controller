package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestOpenAIErrorKeepsStableEnvelopeAndPrivateCause(t *testing.T) {
	cause := errors.New("private native buffer")
	err := OpenAI("tools_required", "needle-2 requires at least one function tool", "tools", http.StatusBadRequest, cause)
	if !errors.Is(err, cause) {
		t.Fatal("cause not available through errors.Is")
	}
	body, marshalErr := json.Marshal(err.Response())
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	want := `{"error":{"message":"needle-2 requires at least one function tool","type":"invalid_request_error","param":"tools","code":"tools_required"}}`
	if string(body) != want {
		t.Fatalf("body = %s", body)
	}
	if err.HTTPStatus != 400 || err.ErrorCode() != "tools_required" || err.Error() == cause.Error() {
		t.Fatalf("error = %+v", err)
	}
}

func TestOpenAIErrorSupportsAuthenticationKind(t *testing.T) {
	err := OpenAIKind("invalid_api_key", "Incorrect API key provided.", "", "authentication_error", http.StatusUnauthorized, nil)
	body, _ := json.Marshal(err.Response())
	want := `{"error":{"message":"Incorrect API key provided.","type":"authentication_error","param":null,"code":"invalid_api_key"}}`
	if string(body) != want {
		t.Fatalf("body = %s", body)
	}
}
