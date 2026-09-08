package handler

import (
	"needle-controller/internal/native"
	"needle-controller/internal/requestid"
	"net/http"
)

type readyResponse struct {
	Status string `json:"status"`
	Model  string `json:"model"`
}

func Ready(service interface{ State() native.State }) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := service.State()
		status := http.StatusOK
		if state != native.StateReady {
			status = http.StatusServiceUnavailable
		}
		requestID := requestid.FromRequest(r)
		WriteJSON(w, status, requestID, readyResponse{Status: string(state), Model: string(state)})
	})
}
