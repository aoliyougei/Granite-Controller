package handler

import (
	"context"
	"fmt"
	"needle-controller/internal/apierror"
	"needle-controller/internal/managed"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"needle-controller/internal/requestid"
	"net/http"
)

type OpenAIService interface {
	State() native.State
	ModelList() openai.ModelListResponse
	Complete(context.Context, string, openai.ChatCompletionRequest) (managed.ResponseBody, *apierror.Error)
}

func ChatCompletions(service OpenAIService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestid.FromRequest(r)
		var input openai.ChatCompletionRequest
		if err := DecodeJSON(w, r, &input, 1<<20); err != nil {
			WriteError(w, requestID, err)
			return
		}
		response, err := service.Complete(r.Context(), requestID, input)
		if err != nil {
			if err.Code == "request_canceled" {
				return
			}
			WriteError(w, requestID, err)
			return
		}
		if !input.Stream {
			WriteJSON(w, http.StatusOK, requestID, response)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Request-ID", requestID)
		w.WriteHeader(http.StatusOK)
		includeUsage := input.StreamOptions != nil && input.StreamOptions.IncludeUsage
		for _, event := range managed.StreamEvents(response, includeUsage) {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event.Data)
		}
	})
}
