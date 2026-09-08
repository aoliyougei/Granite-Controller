package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needle-controller/internal/apierror"
)

func TestBearerAuthRejectsInvalidCredentialsWithOpenAIEnvelope(t *testing.T) {
	tests := [][]string{nil, {"Basic abc"}, {"Bearer "}, {"Bearer wrong"}, {"Bearer api-test-value", "Bearer api-test-value"}, {"Bearer  api-test-value"}}
	for _, headers := range tests {
		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		handler := NewBearerAuth("api-test-value")(next)
		r := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		for _, value := range headers {
			r.Header.Add("Authorization", value)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if called || w.Code != 401 {
			t.Fatalf("headers=%v called=%v status=%d", headers, called, w.Code)
		}
		var got apierror.Response
		if json.NewDecoder(w.Body).Decode(&got) != nil || got.Error.Code != "invalid_api_key" || got.Error.Type != "authentication_error" || got.Error.Param != nil {
			t.Fatalf("body=%s", w.Body.String())
		}
	}
}

func TestBearerAuthAllowsExactToken(t *testing.T) {
	called := false
	handler := NewBearerAuth("api-test-value")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(204) }))
	r := httptest.NewRequest("GET", "/v1/models", nil)
	r.Header.Set("Authorization", "Bearer api-test-value")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !called || w.Code != 204 {
		t.Fatalf("called=%v status=%d", called, w.Code)
	}
}
