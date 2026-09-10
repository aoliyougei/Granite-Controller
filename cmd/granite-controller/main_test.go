package main

import (
	"errors"
	"testing"
)

func TestGraniteFailureExitsNonzero(t *testing.T) {
	code := 0
	exitOnGraniteFailure(errors.New("child exited"), func(got int) { code = got })
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
}
