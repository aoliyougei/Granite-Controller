package handler

import (
	"errors"
	"net/http"

	"needle-controller/internal/apierror"
	"needle-controller/internal/logic"
	"needle-controller/internal/requestid"
	"needle-controller/internal/types"
)

func Chat(service logic.ChatService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestid.FromRequest(r)
		var input types.ChatRequest
		if err := DecodeJSON(w, r, &input, 8<<10); err != nil {
			WriteError(w, requestID, err)
			return
		}
		response, err := service.Execute(r.Context(), requestID, input.Message)
		if err != nil {
			var typed *apierror.Error
			if !errors.As(err, &typed) {
				typed = apierror.New(apierror.CodeInternal, "内部服务错误", http.StatusInternalServerError, err)
			}
			WriteError(w, requestID, typed)
			return
		}
		WriteJSON(w, http.StatusAccepted, requestID, response)
	})
}
