package safelog

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLoggerNeverLogsRequestSecretsOrBody(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = errors.New("internal sentinel-response-secret")
		http.Error(w, "safe", 502)
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("sentinel-body-system-history-tools"))
	req.Header.Set("Authorization", "Bearer sentinel-token")
	req.Header.Set("User-Agent", "pi-test")
	rr := httptest.NewRecorder()
	Access(logger)(next).ServeHTTP(rr, req)
	got := output.String()
	for _, secret := range []string{"sentinel-token", "sentinel-body", "Authorization", "system-history-tools"} {
		if strings.Contains(got, secret) {
			t.Fatalf("leaked %q in %q", secret, got)
		}
	}
	for _, safe := range []string{"POST", "/v1/chat/completions", "502", "pi-test"} {
		if !strings.Contains(got, safe) {
			t.Fatalf("missing %q in %q", safe, got)
		}
	}
}
