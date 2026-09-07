package requestid

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestFromRequestPreservesSafeID(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "req_3052:test-1")
	if got := FromRequest(r); got != "req_3052:test-1" {
		t.Fatalf("FromRequest() = %q", got)
	}
}

func TestFromRequestReplacesUnsafeID(t *testing.T) {
	for _, input := range []string{"", "with space", "line\nbreak", strings.Repeat("a", 129)} {
		r := httptest.NewRequest("GET", "/", nil)
		if input != "" {
			r.Header.Set("X-Request-ID", input)
		}
		got := FromRequest(r)
		if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(got) {
			t.Fatalf("FromRequest(%q) = %q, want generated hex ID", input, got)
		}
	}
}
