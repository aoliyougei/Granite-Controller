package handler

import (
	"needle-controller/internal/requestid"
	"net/http"
)

func Models(service OpenAIService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestid.FromRequest(r)
		WriteJSON(w, http.StatusOK, requestID, service.ModelList())
	})
}
