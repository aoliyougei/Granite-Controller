package openai

import (
	"context"
	"errors"
	"net/http"

	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
	"needle-controller/internal/native"
)

type NativeCompleter interface {
	Submit(context.Context, native.Request) (native.Envelope, error)
	State() native.State
}

type Service struct {
	native NativeCompleter
	config config.NativeConfig
	limits SchemaLimits
}

func NewService(completer NativeCompleter, cfg config.NativeConfig, limits SchemaLimits) *Service {
	return &Service{native: completer, config: cfg, limits: limits}
}
func (s *Service) ModelList() ModelListResponse { return ModelList(s.config.ModelID) }
func (s *Service) State() native.State          { return s.native.State() }

func (s *Service) Complete(ctx context.Context, request ChatCompletionRequest) (ChatCompletionResponse, *apierror.Error) {
	if s.native.State() != native.StateReady {
		return ChatCompletionResponse{}, apierror.OpenAI("model_not_ready", "The model is not ready.", "", http.StatusServiceUnavailable, nil)
	}
	tools, toolWarnings, err := ValidateAndSelectTools(request, s.limits)
	if err != nil {
		return ChatCompletionResponse{}, err
	}
	replay, replayWarnings, err := BuildReplay(request, tools, s.config)
	if err != nil {
		return ChatCompletionResponse{}, err
	}
	envelope, nativeErr := s.native.Submit(ctx, replay)
	if nativeErr != nil {
		if errors.Is(nativeErr, context.Canceled) || errors.Is(nativeErr, context.DeadlineExceeded) {
			return ChatCompletionResponse{}, apierror.OpenAI("request_canceled", "Request canceled.", "", 499, nativeErr)
		}
		if errors.Is(nativeErr, native.ErrQueueFull) {
			return ChatCompletionResponse{}, apierror.OpenAI("engine_busy", "The model queue is full.", "", http.StatusTooManyRequests, nativeErr)
		}
		if errors.Is(nativeErr, native.ErrNotReady) || errors.Is(nativeErr, native.ErrClosed) {
			return ChatCompletionResponse{}, apierror.OpenAI("model_not_ready", "The model is not ready.", "", http.StatusServiceUnavailable, nativeErr)
		}
		return ChatCompletionResponse{}, apierror.OpenAI("engine_error", "The native model failed to complete the request.", "", http.StatusBadGateway, nativeErr)
	}
	warnings := append(toolWarnings, replayWarnings...)
	return MapResponse(s.config.ModelID, envelope, warnings)
}
