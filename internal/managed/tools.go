package managed

import (
	"encoding/json"

	"github.com/aoliyougei/granite-controller/internal/openai"
)

type Action string

const (
	ActionGet      Action = "get"
	ActionStart    Action = "start"
	ActionShutdown Action = "shutdown"
	ActionStop     Action = "stop"
	ActionReboot   Action = "reboot"
)

type ActionMetadata struct {
	ToolName, RequiredState string
	Mutation                bool
	Evidence                []string
}

var actions = map[Action]ActionMetadata{
	ActionGet:      {"pve_vm_get", "", false, []string{"查询", "查看", "状态", "运行吗", "开着吗"}},
	ActionStart:    {"pve_vm_start", "stopped", true, []string{"开启", "启动", "开机", "打开"}},
	ActionShutdown: {"pve_vm_shutdown", "running", true, []string{"正常关闭", "正常关机", "优雅关机"}},
	ActionStop:     {"pve_vm_stop", "running", true, []string{"强制停止", "强制关机", "强制断电"}},
	ActionReboot:   {"pve_vm_reboot", "running", true, []string{"重启", "重新启动"}},
}

func Metadata(action Action) (ActionMetadata, bool) { m, ok := actions[action]; return m, ok }

func GraniteTools(allowForceStop bool) []openai.FunctionTool {
	descriptions := []string{
		"Get the current status of a Proxmox VE virtual machine. 查询虚拟机当前状态，不改变虚拟机。",
		"Start a stopped Proxmox VE virtual machine. 开启或启动已停止的虚拟机。",
		"Gracefully shut down a running Proxmox VE virtual machine. Use for normal shutdown, power off, or graceful shutdown. 正常关闭、正常关机或优雅关机必须使用此工具；这不是强制断电。",
		"Force stop a running Proxmox VE virtual machine only when the user explicitly asks to force stop or cut power. 仅当用户原文明示强制停止、强制关机或强制断电时使用；普通关机禁止使用，且可能导致数据损坏。",
		"Reboot a running Proxmox VE virtual machine. 重启或重新启动正在运行的虚拟机。",
	}
	order := []Action{ActionGet, ActionStart, ActionShutdown, ActionStop, ActionReboot}
	params := json.RawMessage(`{"type":"object","properties":{"vmid":{"type":"integer","description":"Numeric VM ID. 虚拟机数字 ID。"}},"required":["vmid"],"additionalProperties":false}`)
	tools := make([]openai.FunctionTool, 0, len(order))
	for i, action := range order {
		if action == ActionStop && !allowForceStop {
			continue
		}
		m, _ := Metadata(action)
		tools = append(tools, openai.FunctionTool{Type: "function", Function: openai.FunctionDefinition{Name: m.ToolName, Description: descriptions[i], Parameters: params}})
	}
	return tools
}
