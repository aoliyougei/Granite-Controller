package safelog

import (
	"log"
	"net/http"
	"strconv"
	"time"
)

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(status int) { r.status = status; r.ResponseWriter.WriteHeader(status) }
func Access(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &recorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			logger.Printf("method=%s path=%s status=%d durationMs=%d contentLength=%s userAgent=%q", r.Method, r.URL.Path, rw.status, time.Since(start).Milliseconds(), strconv.FormatInt(r.ContentLength, 10), r.UserAgent())
		})
	}
}
