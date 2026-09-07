package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needle-controller/internal/types"
)

func TestBearerAuthRejectsInvalidCredentialsIdentically(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
	}{
		{"missing", nil},
		{"wrong scheme", []string{"Basic abc"}},
		{"empty", []string{"Bearer "}},
		{"wrong", []string{"Bearer wrong"}},
		{"multiple", []string{"Bearer controller-test-value", "Bearer controller-test-value"}},
		{"extra whitespace", []string{"Bearer  controller-test-value"}},
	}
	var first types.ErrorResponse
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			handler := NewBearerAuth("controller-test-value")(next)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
			r.Header.Set("X-Request-ID", "req-auth")
			for _, value := range tt.headers {
				r.Header.Add("Authorization", value)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if called || w.Code != http.StatusUnauthorized {
				t.Fatalf("called=%v status=%d", called, w.Code)
			}
			var got types.ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Code != "AUTH_UNAUTHORIZED" || got.RequestID != "req-auth" {
				t.Fatalf("response=%+v", got)
			}
			if first.Code == "" {
				first = got
			} else if got != first {
				t.Fatalf("response differs: %+v vs %+v", got, first)
			}
		})
	}
}

func TestBearerAuthAllowsExactToken(t *testing.T) {
	called := false
	handler := NewBearerAuth("controller-test-value")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
	r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	r.Header.Set("Authorization", "Bearer controller-test-value")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !called || w.Code != http.StatusNoContent {
		t.Fatalf("called=%v status=%d", called, w.Code)
	}
}
