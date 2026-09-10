package managed

import "fmt"

type StateDecision struct {
	Execute bool
	Message string
}

func CheckState(action Action, state string, vmid int64) StateDecision {
	meta, ok := Metadata(action)
	if !ok || !meta.Mutation {
		return StateDecision{}
	}
	if state == meta.RequiredState {
		return StateDecision{Execute: true}
	}
	verbs := map[Action]string{ActionStart: "启动", ActionShutdown: "关闭", ActionStop: "强制停止", ActionReboot: "重启"}
	icon := "🟡"
	if state == "running" {
		icon = "🟢"
	}
	if state == "stopped" {
		icon = "🔴"
	}
	return StateDecision{Message: fmt.Sprintf("%s VM %d 已处于 %s 状态，无需重复%s。", icon, vmid, state, verbs[action])}
}
