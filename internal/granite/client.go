package granite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/openai"
)

const responseLimit = 1 << 20

type ToolCall struct{ Type, Name, Arguments string }
type Client struct {
	endpoint, key, model string
	maxTokens            int
	http                 *http.Client
}

func NewClient(endpoint, key, model string, maxTokens int, timeout time.Duration) *Client {
	return &Client{strings.TrimRight(endpoint, "/") + "/v1/chat/completions", key, model, maxTokens, &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *Client) Select(ctx context.Context, text string, tools []openai.FunctionTool) (ToolCall, *apierror.Error) {
	body := struct {
		Model       string                `json:"model"`
		Messages    []map[string]string   `json:"messages"`
		Tools       []openai.FunctionTool `json:"tools"`
		Temperature float64               `json:"temperature"`
		TopP        float64               `json:"top_p"`
		Seed        int                   `json:"seed"`
		MaxTokens   int                   `json:"max_tokens"`
		ToolChoice  string                `json:"tool_choice"`
		Stream      bool                  `json:"stream"`
	}{c.model, []map[string]string{{"role": "user", "content": text}}, tools, 0, 1, 42, c.maxTokens, "required", false}
	encoded, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return ToolCall{}, upstream(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ToolCall{}, upstream(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil || len(data) > responseLimit || resp.StatusCode != http.StatusOK {
		return ToolCall{}, upstream(fmt.Errorf("invalid Granite response status %d", resp.StatusCode))
	}
	var wire struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &wire) != nil || len(wire.Choices) != 1 {
		return ToolCall{}, upstream(fmt.Errorf("invalid Granite JSON"))
	}
	calls := wire.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		return ToolCall{}, apierror.OpenAI("tool_call_required", "模型未能确定要执行的虚拟机操作，请明确说明查询、开启、正常关机、强制停止或重启。", "messages", http.StatusUnprocessableEntity, nil)
	}
	if len(calls) != 1 {
		return ToolCall{}, apierror.OpenAI("multiple_tool_calls_not_supported", "每次请求只能执行一个虚拟机操作。", "messages", http.StatusUnprocessableEntity, nil)
	}
	return ToolCall{Type: calls[0].Type, Name: calls[0].Function.Name, Arguments: calls[0].Function.Arguments}, nil
}
func upstream(err error) *apierror.Error {
	return apierror.OpenAI("granite_request_failed", "Granite 模型请求失败。", "", http.StatusBadGateway, err)
}
