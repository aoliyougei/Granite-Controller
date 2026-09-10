package handler

import (
	"github.com/aoliyougei/granite-controller/internal/requestid"
	"net/http"
)

type readyResponse struct {
	Status string `json:"status"`
	Model  string `json:"model"`
}

func Ready(service interface{ Ready() bool }) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		state := "ready"
		if !service.Ready() {
			status = http.StatusServiceUnavailable
			state = "granite_loading"
		}
		WriteJSON(w, status, requestid.FromRequest(r), readyResponse{state, state})
	})
}
