package handler

import (
	"net/http"

	"needle-controller/internal/requestid"
	"needle-controller/internal/types"
)

func Health() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, requestid.FromRequest(r), types.HealthResponse{Status: "ok"})
	})
}
