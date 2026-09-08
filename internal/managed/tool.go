package managed

import (
	"needle-controller/internal/config"
	"needle-controller/internal/native"
)

var managedToolJSON = []byte(`[{"name":"pve_vm_start","description":"Start a Proxmox VE virtual machine","parameters":{"type":"object","properties":{"vmid":{"type":"integer","description":"Numeric Proxmox VE virtual machine ID"}},"required":["vmid"],"additionalProperties":false}}]`)

func NativeRequest(command Command, cfg config.NativeConfig) native.Request {
	return native.Request{ToolsJSON: append([]byte(nil), managedToolJSON...), ToolNames: []string{"pve_vm_start"}, Turns: []native.Turn{{Kind: native.TurnUser, Text: command.Normalized}}, MaxNewTokens: cfg.MaxNewTokens, ToolIndexPath: cfg.ToolIndexPath}
}
