package logic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"needle-controller/internal/apierror"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/needle"
	"needle-controller/internal/types"
)

type ChatService interface {
	Execute(ctx context.Context, requestID, message string) (types.ChatResponse, error)
}

type chatService struct {
	needle           needle.Service
	infra            infracontrol.Service
	minConfidence    float64
	maxMessageLength int
}

func NewChatService(needleService needle.Service, infraService infracontrol.Service, minConfidence float64, maxMessageLength int) ChatService {
	return &chatService{needle: needleService, infra: infraService, minConfidence: minConfidence, maxMessageLength: maxMessageLength}
}

func (s *chatService) Execute(ctx context.Context, requestID, message string) (types.ChatResponse, error) {
	message = strings.TrimSpace(message)
	if message == "" || utf8.RuneCountInString(message) > s.maxMessageLength {
		return types.ChatResponse{}, apierror.New(apierror.CodeMessageInvalid, "消息不能为空或超过长度限制", http.StatusBadRequest, nil)
	}
	normalized, err := NormalizeChineseVMStart(message)
	if err != nil {
		return types.ChatResponse{}, err
	}
	completion, err := s.needle.Complete(ctx, normalized.NeedleMessage)
	if err != nil {
		return types.ChatResponse{}, err
	}
	command, err := needle.ValidateStartVM(completion, s.minConfidence)
	if err != nil {
		return types.ChatResponse{}, err
	}
	if command.VMID != normalized.VMID {
		return types.ChatResponse{}, apierror.New(apierror.CodeNeedleArgumentsUngrounded, "模型返回的虚拟机 ID 与中文原始指令不一致，未执行操作", http.StatusUnprocessableEntity, nil)
	}
	result, err := s.infra.StartVM(ctx, requestID, command.VMID)
	if err != nil {
		return types.ChatResponse{}, err
	}
	return types.ChatResponse{
		Status:         "accepted",
		Message:        fmt.Sprintf("VM %d 的启动请求已提交", command.VMID),
		Tool:           "pve_vm_start",
		Arguments:      types.StartVMArguments{VMID: command.VMID},
		Confidence:     command.Confidence,
		UpstreamStatus: result.UpstreamStatus,
		RequestID:      requestID,
	}, nil
}
