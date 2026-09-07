package apierror

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewKeepsStablePublicFieldsAndPrivateCause(t *testing.T) {
	cause := errors.New("private upstream body")
	err := New(CodeNeedleUnavailable, "Needle 服务不可用", http.StatusBadGateway, cause)
	if err.Code != CodeNeedleUnavailable || err.Message != "Needle 服务不可用" || err.HTTPStatus != 502 {
		t.Fatalf("unexpected public error: %+v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("cause is not available through errors.Is")
	}
	if err.Error() == cause.Error() {
		t.Fatal("Error() leaked the private cause")
	}
}
