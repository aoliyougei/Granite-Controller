package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckHealthRequiresOKStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		wantOK bool
	}{
		{name: "healthy", status: http.StatusOK, wantOK: true},
		{name: "unhealthy", status: http.StatusServiceUnavailable, wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			err := checkHealth(server.URL, time.Second)
			if (err == nil) != tc.wantOK {
				t.Fatalf("checkHealth() error = %v", err)
			}
		})
	}
}
