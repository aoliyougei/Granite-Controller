package needle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"needle-controller/internal/apierror"
	"needle-controller/internal/config"
)

const responseLimit = 1 << 20

type Service interface {
	Complete(ctx context.Context, message string) (Completion, error)
	Ready(ctx context.Context) error
}

type Client struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
	http      *http.Client
}

func NewClient(cfg config.NeedleConfig) *Client {
	return &Client{
		baseURL: cfg.BaseURL, apiKey: cfg.APIKey, model: cfg.Model, maxTokens: cfg.MaxTokens,
		http: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

func (c *Client) Complete(ctx context.Context, message string) (Completion, error) {
	payload := chatRequest{
		Model: c.model, Messages: []chatMessage{{Role: "user", Content: message}}, MaxTokens: c.maxTokens,
		Tools: []tool{{Type: "function", Function: functionTool{
			Name:        "pve_vm_start",
			Description: "Start or power on a Proxmox VE virtual machine. 用于开启、启动或开机一个 PVE 虚拟机。",
			Parameters:  parameters{Type: "object", Properties: map[string]property{"vmid": {Type: "integer", Description: "Proxmox VE 虚拟机的数字 ID，例如 3052"}}, Required: []string{"vmid"}, AdditionalProperties: false},
		}}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Completion{}, unavailable(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Completion{}, unavailable(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Completion{}, unavailable(err)
	}
	defer resp.Body.Close()
	data, err := boundedRead(resp.Body)
	if err != nil {
		return Completion{}, unavailable(err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return Completion{}, apierror.New(apierror.CodeNeedleBusy, "Needle 服务繁忙", http.StatusServiceUnavailable, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Completion{}, unavailable(fmt.Errorf("needle status %d", resp.StatusCode))
	}
	var decoded chatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return Completion{}, unavailable(err)
	}
	var calls []ToolCall
	if len(decoded.Choices) > 0 {
		calls = decoded.Choices[0].Message.ToolCalls
	}
	return Completion{ToolCalls: calls, Safety: decoded.Safety}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := boundedRead(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("needle not ready")
	}
	var health struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &health) != nil || health.Status != "ok" {
		return fmt.Errorf("needle not ready")
	}
	return nil
}

func boundedRead(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, responseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > responseLimit {
		return nil, fmt.Errorf("response too large")
	}
	return data, nil
}

func unavailable(cause error) *apierror.Error {
	return apierror.New(apierror.CodeNeedleUnavailable, "Needle 服务不可用", http.StatusBadGateway, cause)
}
