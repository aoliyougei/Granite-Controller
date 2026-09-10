package managed

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/granite"
)

type ActionCall struct { Action Action; VMID int64 }

func ValidateToolCall(input Input, call granite.ToolCall, allowForceStop bool) (ActionCall, *apierror.Error) {
	var action Action
	for candidate, meta := range actions { if meta.ToolName == call.Name { action = candidate; break } }
	if action == "" { return ActionCall{}, invalidCall() }
	var args struct{ VMID int64 `json:"vmid"` }
	decoder:=json.NewDecoder(bytes.NewBufferString(call.Arguments)); decoder.DisallowUnknownFields()
	if decoder.Decode(&args)!=nil || args.VMID<=0 || decoder.Decode(&struct{}{}) == nil { return ActionCall{}, invalidCall() }
	if args.VMID != input.VMID { return ActionCall{}, apierror.OpenAI("vmid_mismatch","模型返回的虚拟机 ID 与请求不一致。","messages",http.StatusUnprocessableEntity,nil) }
	meta,_:=Metadata(action)
	if input.HasNegation && meta.Mutation { return ActionCall{}, apierror.OpenAI("negated_mutation","否定指令不会执行虚拟机变更。","messages",http.StatusUnprocessableEntity,nil) }
	if !containsAny(input.Original,meta.Evidence) { return ActionCall{}, apierror.OpenAI("ambiguous_managed_command","请明确说明要执行的虚拟机操作。","messages",http.StatusUnprocessableEntity,nil) }
	if action==ActionStop && !allowForceStop { return ActionCall{}, apierror.OpenAI("force_stop_disabled","强制停止功能当前未启用。","messages",http.StatusForbidden,nil) }
	return ActionCall{action,args.VMID},nil
}
func invalidCall()*apierror.Error{return apierror.OpenAI("invalid_tool_call","模型返回了无效的工具调用。","messages",http.StatusUnprocessableEntity,nil)}
