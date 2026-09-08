package managed

import (
	"encoding/json"
	"needle-controller/internal/config"
	"needle-controller/internal/native"
	"testing"
)

func TestNativeRequestUsesOnlyFixedVMStartTool(t *testing.T) {
	command := Command{Original: "开启 VM 3052", Normalized: "Start VM 3052", VMID: 3052}
	got := NativeRequest(command, config.NativeConfig{MaxNewTokens: 256, ToolIndexPath: "/tmp/index"})
	if got.System != "" || got.MaxNewTokens != 256 || got.ToolIndexPath != "/tmp/index" || len(got.Turns) != 1 || got.Turns[0] != (native.Turn{Kind: native.TurnUser, Text: "Start VM 3052"}) || len(got.ToolNames) != 1 || got.ToolNames[0] != "pve_vm_start" {
		t.Fatalf("request=%+v", got)
	}
	var tools []map[string]any
	if json.Unmarshal(got.ToolsJSON, &tools) != nil || len(tools) != 1 {
		t.Fatalf("tools=%s", got.ToolsJSON)
	}
	tool := tools[0]
	if tool["name"] != "pve_vm_start" || tool["description"] != "Start a Proxmox VE virtual machine" {
		t.Fatalf("tool=%v", tool)
	}
	schema := tool["parameters"].(map[string]any)
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("schema=%v", schema)
	}
	required := schema["required"].([]any)
	properties := schema["properties"].(map[string]any)
	if len(required) != 1 || required[0] != "vmid" || properties["vmid"].(map[string]any)["type"] != "integer" {
		t.Fatalf("schema=%v", schema)
	}
}
