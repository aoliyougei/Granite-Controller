package infracontrol

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aoliyougei/granite-controller/internal/config"
)

func TestGetVMWhitelistsFields(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/pve/vms/3052" || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"vmid":3052,"name":"db","status":"running","uptime":12588,"secret":"drop"}`))
	}))
	defer s.Close()
	vm, err := NewClient(config.InfraControlConfig{BaseURL: s.URL, APIToken: "token", Timeout: time.Second}).GetVM(context.Background(), "rid", 3052)
	if err != nil || vm.VMID != 3052 || vm.Name != "db" || vm.Status != "running" || vm.Uptime != 12588 {
		t.Fatalf("vm=%+v err=%v", vm, err)
	}
}
func TestGetVMRejectsMismatchedVMID(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"vmid":9999,"status":"stopped"}`))
	}))
	defer s.Close()
	_, err := NewClient(config.InfraControlConfig{BaseURL: s.URL, APIToken: "token", Timeout: time.Second}).GetVM(context.Background(), "rid", 3052)
	if err == nil || err.Code != "invalid_upstream_response" {
		t.Fatalf("err=%v", err)
	}
}

func TestMutateUsesFixedPathsAndRequires202(t *testing.T) {
	for _, action := range []string{"start", "shutdown", "stop", "reboot"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/v1/pve/vms/3052/"+action {
					t.Fatalf("request=%s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(202)
			}))
			defer s.Close()
			result, err := NewClient(config.InfraControlConfig{BaseURL: s.URL, APIToken: "token", Timeout: time.Second}).Mutate(context.Background(), "rid", action, 3052)
			if err != nil || result.Status != 202 || calls != 1 {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}
func TestExplicitFailureIsKnownAndTransportFailureUnknown(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"code":"node_busy","message":"raw","details":{"secret":"x"},"requestId":"up-1"}`))
	}))
	_, known := NewClient(config.InfraControlConfig{BaseURL: s.URL, APIToken: "t", Timeout: time.Second}).Mutate(context.Background(), "rid", "start", 3052)
	s.Close()
	if known == nil || known.Unknown || known.Code != "node_busy" || known.RequestID != "up-1" {
		t.Fatalf("known=%+v", known)
	}
	_, unknown := NewClient(config.InfraControlConfig{BaseURL: "http://127.0.0.1:1", APIToken: "t", Timeout: time.Millisecond * 100}).Mutate(context.Background(), "rid", "start", 3052)
	if unknown == nil || !unknown.Unknown {
		t.Fatalf("unknown=%+v", unknown)
	}
}
