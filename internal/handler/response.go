package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"needle-controller/internal/apierror"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) *apierror.Error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return apierror.InvalidRequest(err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return apierror.InvalidRequest(err)
	}
	return nil
}

func WriteJSON(w http.ResponseWriter, status int, requestID string, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func WriteError(w http.ResponseWriter, requestID string, err *apierror.Error) {
	WriteJSON(w, err.HTTPStatus, requestID, err.Response())
}
