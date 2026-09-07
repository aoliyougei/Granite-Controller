package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"needle-controller/internal/apierror"
	"needle-controller/internal/requestid"
	"needle-controller/internal/types"
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
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", requestID)
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(types.ErrorResponse{Code: apierror.CodeAuthUnauthorized, Message: "认证失败", RequestID: requestID})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
