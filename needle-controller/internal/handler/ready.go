package handler

import (
	"context"
	"net/http"
	"sync"

	"needle-controller/internal/requestid"
	"needle-controller/internal/types"
)

type ReadyService interface {
	Ready(context.Context) error
}

func Ready(needle, infra ReadyService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		var needleErr, infraErr error
		wg.Add(2)
		go func() { defer wg.Done(); needleErr = needle.Ready(r.Context()) }()
		go func() { defer wg.Done(); infraErr = infra.Ready(r.Context()) }()
		wg.Wait()

		status := http.StatusOK
		body := types.ReadyResponse{Status: "ready", RequestID: requestid.FromRequest(r), Dependencies: types.DependencyStatus{Needle: "ready", InfrastructureControl: "ready"}}
		if needleErr != nil {
			body.Status = "unavailable"
			body.Dependencies.Needle = "unavailable"
			status = http.StatusServiceUnavailable
		}
		if infraErr != nil {
			body.Status = "unavailable"
			body.Dependencies.InfrastructureControl = "unavailable"
			status = http.StatusServiceUnavailable
		}
		WriteJSON(w, status, body.RequestID, body)
	})
}
