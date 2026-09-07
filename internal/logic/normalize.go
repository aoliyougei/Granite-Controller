package logic

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"needle-controller/internal/apierror"
)

var (
	digitsPattern = regexp.MustCompile(`[0-9]+`)
	vmPattern     = regexp.MustCompile(`(?i)(^|[^A-Za-z])VM([^A-Za-z]|$)`)
)

var startPhrases = []string{"开启", "启动", "开机", "打开", "开起来"}
var rejectionPhrases = []string{"不要", "别", "禁止", "无需", "取消", "关闭", "关机", "停止", "重启", "重新启动"}

type NormalizedVMStart struct {
	VMID          int64
	NeedleMessage string
}

func NormalizeChineseVMStart(message string) (NormalizedVMStart, error) {
	message = strings.TrimSpace(message)
	if message == "" || !containsHan(message) || containsAny(message, rejectionPhrases) || !containsAny(message, startPhrases) || !containsVM(message) {
		return NormalizedVMStart{}, invalidChineseInstruction()
	}
	matches := digitsPattern.FindAllString(message, -1)
	if len(matches) != 1 {
		return NormalizedVMStart{}, invalidChineseInstruction()
	}
	vmid, err := strconv.ParseInt(matches[0], 10, 64)
	if err != nil || vmid <= 0 {
		return NormalizedVMStart{}, invalidChineseInstruction()
	}
	return NormalizedVMStart{VMID: vmid, NeedleMessage: fmt.Sprintf("Start VM %d", vmid)}, nil
}

func containsVM(value string) bool {
	return strings.Contains(value, "虚拟机") || vmPattern.MatchString(value)
}

func containsAny(value string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

func containsHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func invalidChineseInstruction() *apierror.Error {
	return apierror.New(apierror.CodeMessageInvalid, "仅支持包含单个正整数 VM ID 的明确中文虚拟机开机指令", http.StatusBadRequest, nil)
}
