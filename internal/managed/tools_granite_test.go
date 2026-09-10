package managed

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGraniteToolsAreFiveStrictBilingualFunctions(t *testing.T) {
	tools := GraniteTools(true)
	if len(tools) != 5 {
		t.Fatalf("tools=%d", len(tools))
	}
	want := []string{"pve_vm_get", "pve_vm_start", "pve_vm_shutdown", "pve_vm_stop", "pve_vm_reboot"}
	for i, tool := range tools {
		if tool.Type != "function" || tool.Function.Name != want[i] || !strings.Contains(tool.Function.Description, ".") || !strings.ContainsAny(tool.Function.Description, "查询开启关闭强制重启") {
			t.Fatalf("tool=%+v", tool)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("schema=%v", schema)
		}
	}
}

func TestGraniteStartToolDescribesEveryAcceptedStartPhrase(t *testing.T) {
	var description string
	for _, tool := range GraniteTools(false) {
		if tool.Function.Name == "pve_vm_start" {
			description = tool.Function.Description
		}
	}
	for _, phrase := range []string{"打开", "开启", "开机", "启动"} {
		if !strings.Contains(description, phrase) {
			t.Fatalf("start description %q lacks %q", description, phrase)
		}
	}
}

func TestGraniteToolsHideForceStopUnlessEnabled(t *testing.T) {
	withoutStop := GraniteTools(false)
	withStop := GraniteTools(true)
	if len(withoutStop) != 4 || len(withStop) != 5 {
		t.Fatalf("without=%d with=%d", len(withoutStop), len(withStop))
	}
	for _, tool := range withoutStop {
		if tool.Function.Name == "pve_vm_stop" {
			t.Fatal("force-stop tool exposed while disabled")
		}
	}
}

func TestActionMetadata(t *testing.T) {
	tests := []struct {
		action      Action
		name, state string
		mutation    bool
	}{
		{ActionGet, "pve_vm_get", "", false}, {ActionStart, "pve_vm_start", "stopped", true}, {ActionShutdown, "pve_vm_shutdown", "running", true}, {ActionStop, "pve_vm_stop", "running", true}, {ActionReboot, "pve_vm_reboot", "running", true},
	}
	for _, tc := range tests {
		m, ok := Metadata(tc.action)
		if !ok || m.ToolName != tc.name || m.RequiredState != tc.state || m.Mutation != tc.mutation {
			t.Fatalf("metadata=%+v ok=%v", m, ok)
		}
	}
}
