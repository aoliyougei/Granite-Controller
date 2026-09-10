package infracontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/aoliyougei/granite-controller/internal/config"
)

const responseLimit = 1 << 20

type VM struct {
	VMID   int64  `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Uptime int64  `json:"uptime"`
}
type Result struct {
	Status            int
	UpstreamRequestID string
}
type Error struct {
	Code, Message, RequestID, NetworkClass string
	Status                                 int
	Unknown                                bool
	Cause                                  error
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

type Service interface {
	GetVM(context.Context, string, int64) (VM, *Error)
	Mutate(context.Context, string, string, int64) (Result, *Error)
}
type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(cfg config.InfraControlConfig) *Client {
	return &Client{cfg.BaseURL, cfg.APIToken, &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) GetVM(ctx context.Context, requestID string, vmid int64) (VM, *Error) {
	resp, err := c.do(ctx, requestID, http.MethodGet, "/api/v1/pve/vms/"+strconv.FormatInt(vmid, 10))
	if err != nil {
		return VM{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return VM{}, parseError(resp, false)
	}
	var vm VM
	decoder := json.NewDecoder(io.LimitReader(resp.Body, responseLimit+1))
	if decoder.Decode(&vm) != nil || vm.VMID != vmid || vm.Status == "" {
		return VM{}, known("invalid_upstream_response", "Infrastructure 返回了无效响应。", resp.StatusCode)
	}
	return vm, nil
}
func (c *Client) Mutate(ctx context.Context, requestID, action string, vmid int64) (Result, *Error) {
	if action != "start" && action != "shutdown" && action != "stop" && action != "reboot" {
		return Result{}, known("invalid_action", "无效的虚拟机操作。", 0)
	}
	resp, err := c.do(ctx, requestID, http.MethodPost, "/api/v1/pve/vms/"+strconv.FormatInt(vmid, 10)+"/"+action)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return Result{}, parseError(resp, false)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseLimit+1))
	return Result{resp.StatusCode, resp.Header.Get("X-Request-ID")}, nil
}
func (c *Client) do(ctx context.Context, requestID, method, path string) (*http.Response, *Error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(nil))
	if err != nil {
		return nil, transport(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, transport(err)
	}
	return resp, nil
}
func parseError(resp *http.Response, unknown bool) *Error {
	var body struct {
		Code, Message, RequestID string
		Details                  json.RawMessage
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, responseLimit+1)).Decode(&body)
	if body.Code == "" {
		body.Code = "upstream_request_failed"
	}
	message := "Infrastructure 请求失败。"
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		message = "Infrastructure 认证失败。"
	}
	return &Error{Code: body.Code, Message: message, RequestID: body.RequestID, Status: resp.StatusCode, Unknown: unknown}
}
func known(code, message string, status int) *Error {
	return &Error{Code: code, Message: message, Status: status}
}
func transport(err error) *Error {
	class := "unknown"
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		class = "timeout"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		class = "dns"
	}
	return &Error{Code: "upstream_request_failed", Message: fmt.Sprintf("Infrastructure 网络请求失败；操作结果未知，请先查询状态。"), NetworkClass: class, Unknown: true, Cause: err}
}
