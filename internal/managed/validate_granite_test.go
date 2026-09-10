package managed

import (
	"github.com/aoliyougei/granite-controller/internal/granite"
	"testing"
)

func TestValidateToolCall(t *testing.T) {
	tests := []struct {
		text, name, args string
		allow            bool
		want             Action
		code             string
	}{
		{"查询 VM 3052 状态", "pve_vm_get", `{"vmid":3052}`, false, ActionGet, ""},
		{"VM 3052 不是还在运行吗", "pve_vm_get", `{"vmid":3052}`, false, ActionGet, ""},
		{"不要启动 VM 3052", "pve_vm_start", `{"vmid":3052}`, false, "", "negated_mutation"},
		{"重新启动 VM 3052", "pve_vm_start", `{"vmid":3052}`, false, "", "ambiguous_managed_command"},
		{"处理一下 VM 3052", "pve_vm_reboot", `{"vmid":3052}`, false, "", "ambiguous_managed_command"},
		{"重启 VM 3052", "pve_vm_reboot", `{"vmid":9999}`, false, "", "vmid_mismatch"},
		{"强制停止 VM 3052", "pve_vm_stop", `{"vmid":3052}`, false, "", "force_stop_disabled"},
		{"强制停止 VM 3052", "pve_vm_stop", `{"vmid":3052}`, true, ActionStop, ""},
		{"启动 VM 3052", "unknown", `{"vmid":3052}`, false, "", "invalid_tool_call"},
		{"启动 VM 3052", "pve_vm_start", `{"vmid":3052,"x":1}`, false, "", "invalid_tool_call"},
		{"启动 VM 3052", "pve_vm_start", `{"vmid":3052} garbage`, false, "", "invalid_tool_call"},
	}
	for _, tc := range tests {
		t.Run(tc.text+tc.name, func(t *testing.T) {
			in, err := AnalyzeInput(tc.text)
			if err != nil {
				t.Fatal(err)
			}
			got, apiErr := ValidateToolCall(in, granite.ToolCall{Type: "function", Name: tc.name, Arguments: tc.args}, tc.allow)
			if tc.code == "" {
				if apiErr != nil || got.Action != tc.want || got.VMID != 3052 {
					t.Fatalf("got=%+v err=%v", got, apiErr)
				}
			} else if apiErr == nil || apiErr.Code != tc.code {
				t.Fatalf("err=%v", apiErr)
			}
		})
	}
}
