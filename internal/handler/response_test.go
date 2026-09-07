package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONAcceptsOneStrictObject(t *testing.T) {
	type request struct {
		Message string `json:"message"`
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"message":"start"}`))
	w := httptest.NewRecorder()
	var got request
	if err := DecodeJSON(w, r, &got, 8192); err != nil || got.Message != "start" {
		t.Fatalf("DecodeJSON() got=%+v err=%v", got, err)
	}
}

func TestDecodeJSONRejectsInvalidBodies(t *testing.T) {
	tests := []string{
		`{"message":"start","extra":true}`,
		`{"message":"start"} {}`,
		``,
		`{"message":`,
		`{"message":"` + strings.Repeat("x", 9000) + `"}`,
	}
	for _, body := range tests {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		var dst struct {
			Message string `json:"message"`
		}
		err := DecodeJSON(w, r, &dst, 8192)
		if err == nil || err.Code != "REQUEST_INVALID" {
			t.Fatalf("body length %d: error=%v", len(body), err)
		}
		if strings.Contains(err.Message, body) && body != "" {
			t.Fatal("error echoed request body")
		}
	}
}
