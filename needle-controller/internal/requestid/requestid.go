package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

var safe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func Valid(value string) bool { return safe.MatchString(value) }

func FromRequest(r *http.Request) string {
	value := r.Header.Get("X-Request-ID")
	if Valid(value) {
		return value
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return "00000000000000000000000000000000"
}
