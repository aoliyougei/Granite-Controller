package managed

import (
	"fmt"
	"needle-controller/internal/apierror"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	digits       = regexp.MustCompile(`[0-9]+`)
	standaloneVM = regexp.MustCompile(`(?i)(^|[^A-Za-z])VM([^A-Za-z]|$)`)
)
var starts = []string{"开启", "启动", "开机", "打开", "开起来"}
var rejects = []string{"不要", "别", "禁止", "无需", "取消", "关闭", "关机", "停止", "重启", "重新启动"}

type Command struct {
	Original   string
	Normalized string
	VMID       int64
}

func NormalizeVMStart(input Input) (Command, *apierror.Error) {
	value := input.Original
	if !hasHan(value) || contains(value, rejects) || !contains(value, starts) || !(strings.Contains(value, "虚拟机") || standaloneVM.MatchString(value)) {
		return Command{}, unsupported()
	}
	matches := digits.FindAllString(value, -1)
	if len(matches) != 1 {
		return Command{}, unsupported()
	}
	vmid, err := strconv.ParseInt(matches[0], 10, 64)
	if err != nil || vmid <= 0 {
		return Command{}, unsupported()
	}
	return Command{Original: value, Normalized: fmt.Sprintf("Start VM %d", vmid), VMID: vmid}, nil
}
func contains(value string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}
func hasHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
func unsupported() *apierror.Error {
	return apierror.OpenAI("unsupported_managed_command", "Only an explicit Chinese command to start one VM is supported.", "messages", http.StatusBadRequest, nil)
}
