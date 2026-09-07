package needle

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"

	"needle-controller/internal/apierror"
)

type StartVMCommand struct {
	VMID       int64
	Confidence float64
	ToolCallID string
}

func ValidateStartVM(completion Completion, minConfidence float64) (StartVMCommand, error) {
	if len(completion.ToolCalls) == 0 {
		return StartVMCommand{}, validationError(apierror.CodeNeedleNoToolCall, "模型未选择可执行工具")
	}
	if len(completion.ToolCalls) != 1 {
		return StartVMCommand{}, validationError(apierror.CodeNeedleMultipleToolCalls, "模型返回了多个工具调用")
	}
	call := completion.ToolCalls[0]
	if call.Type != "function" || call.Function.Name != "pve_vm_start" {
		return StartVMCommand{}, validationError(apierror.CodeNeedleToolNotAllowed, "模型选择了不允许的工具")
	}

	vmid, code := parseVMID(call.Function.Arguments)
	if code != "" {
		message := "工具参数无效"
		if code == apierror.CodeNeedleVMIDInvalid {
			message = "虚拟机 ID 必须是正整数"
		}
		return StartVMCommand{}, validationError(code, message)
	}

	confidence := completion.Safety.Confidence
	if confidence == nil || math.IsNaN(*confidence) || math.IsInf(*confidence, 0) || *confidence < 0 || *confidence > 1 {
		return StartVMCommand{}, validationError(apierror.CodeNeedleSafetyMetadataInvalid, "模型安全元数据无效")
	}
	if *confidence < minConfidence {
		return StartVMCommand{}, validationError(apierror.CodeNeedleLowConfidence, "模型置信度不足，未执行操作")
	}
	if len(completion.Safety.Validation.Ungrounded) == 0 || bytes.Equal(bytes.TrimSpace(completion.Safety.Validation.Ungrounded), []byte("null")) {
		return StartVMCommand{}, validationError(apierror.CodeNeedleSafetyMetadataInvalid, "模型安全元数据无效")
	}
	var ungrounded []json.RawMessage
	if err := json.Unmarshal(completion.Safety.Validation.Ungrounded, &ungrounded); err != nil {
		return StartVMCommand{}, validationError(apierror.CodeNeedleSafetyMetadataInvalid, "模型安全元数据无效")
	}
	if len(ungrounded) != 0 {
		return StartVMCommand{}, validationError(apierror.CodeNeedleArgumentsUngrounded, "模型参数缺少输入依据，未执行操作")
	}
	if completion.Safety.Validation.Negation == nil {
		return StartVMCommand{}, validationError(apierror.CodeNeedleSafetyMetadataInvalid, "模型安全元数据无效")
	}
	if *completion.Safety.Validation.Negation {
		return StartVMCommand{}, validationError(apierror.CodeNeedleNegationDetected, "检测到否定意图，未执行操作")
	}
	return StartVMCommand{VMID: vmid, Confidence: *confidence, ToolCallID: call.ID}, nil
}

func parseVMID(data []byte) (int64, string) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return 0, apierror.CodeNeedleArgumentsInvalid
	}
	seen := false
	var number json.Number
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, apierror.CodeNeedleArgumentsInvalid
		}
		key, ok := keyToken.(string)
		if !ok || key != "vmid" {
			return 0, apierror.CodeNeedleArgumentsInvalid
		}
		if seen {
			return 0, apierror.CodeNeedleVMIDInvalid
		}
		seen = true
		value, err := decoder.Token()
		if err != nil {
			return 0, apierror.CodeNeedleVMIDInvalid
		}
		var numberOK bool
		number, numberOK = value.(json.Number)
		if !numberOK {
			return 0, apierror.CodeNeedleVMIDInvalid
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return 0, apierror.CodeNeedleArgumentsInvalid
	}
	if !seen {
		return 0, apierror.CodeNeedleVMIDInvalid
	}
	if token, err = decoder.Token(); err != io.EOF {
		return 0, apierror.CodeNeedleArgumentsInvalid
	}
	vmid, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil || vmid <= 0 {
		return 0, apierror.CodeNeedleVMIDInvalid
	}
	return vmid, ""
}

func validationError(code, message string) *apierror.Error {
	return apierror.New(code, message, http.StatusUnprocessableEntity, nil)
}
