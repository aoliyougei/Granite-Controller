package handler

import (
	"github.com/aoliyougei/granite-controller/internal/requestid"
	"net/http"
)

func Health() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, requestid.FromRequest(r), struct {
			Status string `json:"status"`
		}{Status: "ok"})
	})
}
