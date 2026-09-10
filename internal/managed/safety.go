package managed

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/aoliyougei/granite-controller/internal/apierror"
)

var decimalID = regexp.MustCompile(`[0-9]+`)
var conditions = []string{"如果", "假如", "要是", "确认后", "完成后"}
var batches = []string{"全部", "所有", "分别", "依次", "然后", "接着"}
var negations = []string{"不要", "别", "禁止", "取消", "无需", "不允许", "不是", "没有", "没开"}

func AnalyzeInput(text string) (Input, *apierror.Error) {
	original := strings.TrimSpace(text)
	matches := decimalID.FindAllString(original, -1)
	ids := map[int64]struct{}{}
	for _, match := range matches {
		id, err := strconv.ParseInt(match, 10, 64)
		if err != nil || id <= 0 {
			return Input{}, safetyError("invalid_vmid", "请求必须包含一个有效的虚拟机 ID。")
		}
		ids[id] = struct{}{}
	}
	if len(ids) != 1 {
		return Input{}, safetyError("single_vmid_required", "每次请求必须且只能指定一个虚拟机 ID。")
	}
	for _, marker := range append(conditions, batches...) {
		if strings.Contains(original, marker) {
			return Input{}, safetyError("complex_managed_command", "不支持条件、批量或顺序虚拟机操作。")
		}
	}
	if strings.Contains(original, "等") && strings.Contains(original, "再") {
		return Input{}, safetyError("complex_managed_command", "不支持条件、批量或顺序虚拟机操作。")
	}
	if explicitActionCount(original) > 1 {
		return Input{}, safetyError("multiple_actions_not_supported", "每次请求只能执行一个虚拟机操作。")
	}
	var vmid int64
	for id := range ids {
		vmid = id
	}
	return Input{Original: original, VMID: vmid, HasNegation: containsAny(original, negations)}, nil
}

func explicitActionCount(text string) int {
	count := 0
	remaining := text
	for _, markers := range [][]string{
		{"强制停止", "强制关机", "强制断电"},
		{"重新启动", "重启"},
		{"正常关闭", "正常关机", "优雅关机", "关闭", "关机"},
		{"开启", "启动", "开机", "打开"},
	} {
		matched := false
		for _, marker := range markers {
			if strings.Contains(remaining, marker) {
				remaining = strings.ReplaceAll(remaining, marker, "")
				matched = true
			}
		}
		if matched {
			count++
		}
	}
	return count
}
func containsAny(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
func safetyError(code, message string) *apierror.Error {
	return apierror.OpenAI(code, message, "messages", http.StatusUnprocessableEntity, nil)
}
