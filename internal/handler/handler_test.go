package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"needle-controller/internal/apierror"
	"needle-controller/internal/types"
)

type fakeChat struct {
	response types.ChatResponse
	err      error
	calls    int
}

func (f *fakeChat) Execute(_ context.Context, requestID, message string) (types.ChatResponse, error) {
	f.calls++
	f.response.RequestID = requestID
	return f.response, f.err
}

type fakeReady struct {
	err   error
	calls int
}

func (f *fakeReady) Ready(context.Context) error { f.calls++; return f.err }

func TestHealthHandlerIsPublicAndAlive(t *testing.T) {
	w := httptest.NewRecorder()
	Health().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReadyHandlerReportsDependencies(t *testing.T) {
	tests := []struct {
		name                string
		needleErr, infraErr error
		status              int
		overall             string
	}{
		{"ready", nil, nil, 200, "ready"},
		{"needle down", errors.New("down"), nil, 503, "unavailable"},
		{"infra down", nil, errors.New("down"), 503, "unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Ready(&fakeReady{err: tt.needleErr}, &fakeReady{err: tt.infraErr}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if w.Code != tt.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var got types.ReadyResponse
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.overall || got.RequestID == "" || w.Header().Get("X-Request-ID") != got.RequestID {
				t.Fatalf("response=%+v header=%q", got, w.Header().Get("X-Request-ID"))
			}
		})
	}
}

func TestChatHandlerStrictlyDecodesAndMapsResults(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		service := &fakeChat{response: types.ChatResponse{Status: "accepted", UpstreamStatus: 202}}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"开启 3052"}`))
		r.Header.Set("X-Request-ID", "req-chat")
		Chat(service).ServeHTTP(w, r)
		if w.Code != http.StatusAccepted || service.calls != 1 || w.Header().Get("X-Request-ID") != "req-chat" {
			t.Fatalf("status=%d calls=%d headers=%v", w.Code, service.calls, w.Header())
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		service := &fakeChat{}
		w := httptest.NewRecorder()
		Chat(service).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"start","url":"bad"}`)))
		if w.Code != http.StatusBadRequest || service.calls != 0 {
			t.Fatalf("status=%d calls=%d", w.Code, service.calls)
		}
	})
	t.Run("typed error", func(t *testing.T) {
		service := &fakeChat{err: apierror.New(apierror.CodeNeedleLowConfidence, "模型置信度不足，未执行操作", 422, nil)}
		w := httptest.NewRecorder()
		Chat(service).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"start"}`)))
		if w.Code != 422 || !strings.Contains(w.Body.String(), "NEEDLE_LOW_CONFIDENCE") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("unexpected error hidden", func(t *testing.T) {
		service := &fakeChat{err: errors.New("private secret detail")}
		w := httptest.NewRecorder()
		Chat(service).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(`{"message":"start"}`)))
		if w.Code != 500 || strings.Contains(w.Body.String(), "private secret detail") || !strings.Contains(w.Body.String(), "INTERNAL_ERROR") {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
}
