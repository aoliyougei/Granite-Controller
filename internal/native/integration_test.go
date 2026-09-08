//go:build needle_native && linux && amd64 && cgo

package native

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestRealNeedleSelectsVMStartAndIsolatesRequests(t *testing.T) {
	abi, err := NewABI()
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(abi, 1<<20)
	tools := []byte(`[{"name":"pve_vm_start","description":"Start a Proxmox VE virtual machine","parameters":{"type":"object","properties":{"vmid":{"type":"integer"}},"required":["vmid"],"additionalProperties":false}}]`)

	for _, vmid := range []int{3052, 4053} {
		result, err := engine.Execute(Request{
			ToolsJSON:    tools,
			ToolNames:    []string{"pve_vm_start"},
			Turns:        []Turn{{Kind: TurnUser, Text: "Start VM " + strconv.Itoa(vmid)}},
			MaxNewTokens: 256,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.FunctionCalls) != 1 || result.FunctionCalls[0].Name != "pve_vm_start" || result.Confidence == nil {
			t.Fatalf("VM %d result = %+v", vmid, result)
		}
		var got int
		if err := json.Unmarshal(result.FunctionCalls[0].Arguments["vmid"], &got); err != nil || got != vmid {
			t.Fatalf("VM %d arguments = %s, error = %v", vmid, result.FunctionCalls[0].Arguments["vmid"], err)
		}
	}
}
