package infracontrol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
	"net/http"
	"strconv"
)

const responseLimit = 1 << 20

type Result struct {
	Status            int
	UpstreamRequestID string
}
type Service interface {
	StartVM(context.Context, string, int64) (Result, *apierror.Error)
}
type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(cfg config.InfraControlConfig) *Client {
	return &Client{baseURL: cfg.BaseURL, token: cfg.APIToken, http: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) StartVM(ctx context.Context, requestID string, vmid int64) (Result, *apierror.Error) {
	url := c.baseURL + "/api/v1/pve/vms/" + strconv.FormatInt(vmid, 10) + "/start"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return Result{}, requestFailed(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	response, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, apierror.OpenAI("upstream_timeout", "Infrastructure request timed out; query VM status before repeating the command.", "", http.StatusGatewayTimeout, err)
		}
		return Result{}, requestFailed(err)
	}
	defer response.Body.Close()
	if _, err = io.Copy(io.Discard, io.LimitReader(response.Body, responseLimit+1)); err != nil {
		return Result{}, requestFailed(err)
	}
	if response.StatusCode == http.StatusAccepted {
		return Result{Status: response.StatusCode, UpstreamRequestID: response.Header.Get("X-Request-ID")}, nil
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Result{}, apierror.OpenAI("upstream_auth_failed", "Infrastructure authentication failed.", "", http.StatusBadGateway, nil)
	}
	return Result{}, requestFailed(fmt.Errorf("upstream status %d", response.StatusCode))
}
func requestFailed(cause error) *apierror.Error {
	return apierror.OpenAI("upstream_request_failed", "Infrastructure request failed; query VM status before repeating the command.", "", http.StatusBadGateway, cause)
}
