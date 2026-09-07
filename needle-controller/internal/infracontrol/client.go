package infracontrol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
)

const responseLimit = 1 << 20

type Service interface {
	StartVM(ctx context.Context, requestID string, vmid int64) (StartResult, error)
	Ready(ctx context.Context) error
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(cfg config.InfraControlConfig) *Client {
	return &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.APIToken,
		http: &http.Client{
			Timeout:       cfg.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) StartVM(ctx context.Context, requestID string, vmid int64) (StartResult, error) {
	path := c.baseURL + "/api/v1/pve/vms/" + strconv.FormatInt(vmid, 10) + "/start"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, nil)
	if err != nil {
		return StartResult{}, requestFailed(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return StartResult{}, apierror.New(apierror.CodeInfraControlTimeout, "Infrastructure Control 请求超时", http.StatusGatewayTimeout, err)
		}
		return StartResult{}, requestFailed(err)
	}
	defer resp.Body.Close()
	if _, err := readBounded(resp.Body); err != nil {
		return StartResult{}, requestFailed(err)
	}
	if resp.StatusCode == http.StatusAccepted {
		return StartResult{UpstreamStatus: resp.StatusCode, UpstreamRequestID: resp.Header.Get("X-Request-ID")}, nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return StartResult{}, apierror.New(apierror.CodeInfraControlAuthFailed, "Infrastructure Control 认证失败", http.StatusBadGateway, nil)
	}
	return StartResult{}, requestFailed(fmt.Errorf("infrastructure status %d", resp.StatusCode))
}

func (c *Client) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := readBounded(resp.Body); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("infrastructure control not ready")
	}
	return nil
}

func readBounded(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, responseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > responseLimit {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}

func requestFailed(cause error) *apierror.Error {
	return apierror.New(apierror.CodeInfraControlRequestFailed, "Infrastructure Control 请求失败", http.StatusBadGateway, cause)
}
