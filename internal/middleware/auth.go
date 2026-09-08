package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"needle-controller/internal/apierror"
	"needle-controller/internal/requestid"
	"net/http"
	"strings"
)

func NewBearerAuth(token string) func(http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			values := r.Header.Values("Authorization")
			valid := len(values) == 1 && strings.HasPrefix(values[0], "Bearer ")
			presented := ""
			if valid {
				presented = strings.TrimPrefix(values[0], "Bearer ")
				valid = presented != "" && !strings.ContainsAny(presented, " \t\r\n")
			}
			digest := sha256.Sum256([]byte(presented))
			valid = valid && subtle.ConstantTimeCompare(digest[:], expected[:]) == 1
			if !valid {
				requestID := requestid.FromRequest(r)
				err := apierror.OpenAIKind("invalid_api_key", "Incorrect API key provided.", "", "authentication_error", http.StatusUnauthorized, nil)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", requestID)
				w.WriteHeader(err.HTTPStatus)
				_ = json.NewEncoder(w).Encode(err.Response())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
