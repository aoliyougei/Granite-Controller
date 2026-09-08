package managed

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
	"time"
)

type ResponseBody struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []Choice        `json:"choices"`
	Usage   openai.Usage    `json:"usage"`
	XNeedle ManagedMetadata `json:"x_needle"`
}
type Choice struct {
	Index        int              `json:"index"`
	Message      AssistantMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}
type AssistantMessage struct {
	Role    string  `json:"role"`
	Content *string `json:"content"`
}
type Arguments struct {
	VMID int64 `json:"vmid"`
}
type ManagedMetadata struct {
	Type           string            `json:"type"`
	Tool           string            `json:"tool"`
	Arguments      Arguments         `json:"arguments"`
	Confidence     float64           `json:"confidence"`
	Validation     native.Validation `json:"validation"`
	Executed       bool              `json:"executed"`
	UpstreamStatus int               `json:"upstream_status"`
	Warnings       []string          `json:"warnings"`
}

func Response(modelID string, call ValidatedCall, result infracontrol.Result, warnings []string) ResponseBody {
	if warnings == nil {
		warnings = []string{}
	}
	content := fmt.Sprintf("VM %d 的启动请求已提交。", call.VMID)
	return ResponseBody{ID: id("chatcmpl"), Object: "chat.completion", Created: time.Now().Unix(), Model: modelID, Choices: []Choice{{Index: 0, Message: AssistantMessage{Role: "assistant", Content: &content}, FinishReason: "stop"}}, Usage: openai.Usage{Estimated: true}, XNeedle: ManagedMetadata{Type: "call", Tool: "pve_vm_start", Arguments: Arguments{VMID: call.VMID}, Confidence: call.Confidence, Validation: call.Validation, Executed: true, UpstreamStatus: result.Status, Warnings: warnings}}
}
func id(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return prefix + "_000000000000000000000000"
	}
	return prefix + "_" + hex.EncodeToString(buffer)
}
func marshal(value any) string { data, _ := json.Marshal(value); return string(data) }
