package managed

import (
	"context"
	"errors"
	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/config"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/native"
	"github.com/aoliyougei/granite-controller/internal/openai"
	"net/http"
)

type NativeCompleter interface {
	Submit(context.Context, native.Request) (native.Envelope, error)
	State() native.State
}
type Service struct {
	native NativeCompleter
	infra  infracontrol.Service
	config config.NativeConfig
}

func NewService(n NativeCompleter, i infracontrol.Service, cfg config.NativeConfig) *Service {
	return &Service{native: n, infra: i, config: cfg}
}
func (s *Service) State() native.State                 { return s.native.State() }
func (s *Service) ModelList() openai.ModelListResponse { return openai.ModelList(s.config.ModelID) }
func (s *Service) Complete(ctx context.Context, requestID string, request openai.ChatCompletionRequest) (ResponseBody, *apierror.Error) {
	input, err := ExtractInput(request, s.config.MaxMessageLength)
	if err != nil {
		return ResponseBody{}, err
	}
	if request.Model != s.config.ModelID {
		return ResponseBody{}, apierror.OpenAI("model_not_found", "The requested model does not exist.", "model", http.StatusNotFound, nil)
	}
	command, err := NormalizeVMStart(input)
	if err != nil {
		return ResponseBody{}, err
	}
	if s.native.State() != native.StateReady {
		return ResponseBody{}, apierror.OpenAI("model_not_ready", "The model is not ready.", "", http.StatusServiceUnavailable, nil)
	}
	envelope, nativeErr := s.native.Submit(ctx, NativeRequest(command, s.config))
	if nativeErr != nil {
		if errors.Is(nativeErr, context.Canceled) || errors.Is(nativeErr, context.DeadlineExceeded) {
			return ResponseBody{}, apierror.OpenAI("request_canceled", "Request canceled.", "", 499, nativeErr)
		}
		if errors.Is(nativeErr, native.ErrQueueFull) {
			return ResponseBody{}, apierror.OpenAI("engine_busy", "The model queue is full.", "", http.StatusTooManyRequests, nativeErr)
		}
		if errors.Is(nativeErr, native.ErrNotReady) || errors.Is(nativeErr, native.ErrClosed) {
			return ResponseBody{}, apierror.OpenAI("model_not_ready", "The model is not ready.", "", http.StatusServiceUnavailable, nativeErr)
		}
		return ResponseBody{}, apierror.OpenAI("engine_error", "The native model failed to complete the request.", "", http.StatusBadGateway, nativeErr)
	}
	call, err := ValidateCall(command, envelope, s.config.MinConfidence)
	if err != nil {
		return ResponseBody{}, err
	}
	result, upstreamErr := s.infra.StartVM(ctx, requestID, call.VMID)
	if upstreamErr != nil {
		return ResponseBody{}, upstreamErr
	}
	return Response(s.config.ModelID, call, result, input.Warnings), nil
}
