package managed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/aoliyougei/granite-controller/internal/apierror"
	"github.com/aoliyougei/granite-controller/internal/dedup"
	"github.com/aoliyougei/granite-controller/internal/granite"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/openai"
	"github.com/aoliyougei/granite-controller/internal/render"
)

type Selector interface{Select(context.Context,string,[]openai.FunctionTool)(granite.ToolCall,*apierror.Error)}
type Infrastructure interface{GetVM(context.Context,string,int64)(infracontrol.VM,*infracontrol.Error);Mutate(context.Context,string,string,int64)(infracontrol.Result,*infracontrol.Error)}
type ServiceConfig struct{ModelID string;MaxMessageLength int;AllowForceStop bool;DedupWindow time.Duration}
type Service struct{selector Selector;infra Infrastructure;cfg ServiceConfig;dedup *dedup.Store}

func NewService(selector Selector,infra Infrastructure,cfg ServiceConfig)*Service{return &Service{selector,infra,cfg,dedup.New(cfg.DedupWindow,time.Now)}}
func(s *Service)ModelList()openai.ModelListResponse{return openai.ModelList(s.cfg.ModelID)}
func(s *Service)Ready()bool{return true}
func(s *Service)Complete(ctx context.Context,requestID string,request openai.ChatCompletionRequest)(ResponseBody,*apierror.Error){
	input,err:=ExtractInput(request,s.cfg.MaxMessageLength);if err!=nil{return ResponseBody{},err}
	if request.Model!=s.cfg.ModelID{return ResponseBody{},apierror.OpenAI("model_not_found","The requested model does not exist.","model",http.StatusNotFound,nil)}
	warnings:=input.Warnings
	input,err=AnalyzeInput(input.Original);if err!=nil{return ResponseBody{},err}
	input.Warnings=warnings
	call,err:=s.selector.Select(ctx,input.Original,GraniteTools());if err!=nil{return ResponseBody{},err}
	actionCall,err:=ValidateToolCall(input,call,s.cfg.AllowForceStop);if err!=nil{return ResponseBody{},err}
	if actionCall.Action==ActionGet{
		vm,up:=s.infra.GetVM(ctx,requestID,actionCall.VMID);if up!=nil{return ResponseBody{},toAPIError(up)}
		return NewResponse(s.cfg.ModelID,actionCall,render.VMStatus(vm.VMID,vm.Name,vm.Status,vm.Uptime),MetadataResult{Result:&vm,UpstreamStatus:200},input.Warnings),nil
	}
	key:=string(actionCall.Action)+":"+call.Arguments
	result:=s.dedup.Do(ctx,key,func(ctx context.Context)(any,dedup.CachePolicy,error){
		vm,up:=s.infra.GetVM(ctx,requestID,actionCall.VMID);if up!=nil{return up,dedup.DoNotCache,up}
		decision:=CheckState(actionCall.Action,vm.Status,actionCall.VMID);if !decision.Execute{return execution{Message:decision.Message,VM:&vm},dedup.Cache,nil}
		upstream,up:=s.infra.Mutate(ctx,requestID,string(actionCall.Action),actionCall.VMID);if up!=nil{policy:=dedup.DoNotCache;if up.Unknown{policy=dedup.Cache};return up,policy,up}
		return execution{Message:render.Submitted(string(actionCall.Action),actionCall.VMID),Result:upstream,Executed:true},dedup.Cache,nil
	})
	if result.Err!=nil { if up,ok:=result.Err.(*infracontrol.Error);ok{return ResponseBody{},toAPIError(up)};return ResponseBody{},apierror.OpenAI("upstream_request_failed","Infrastructure 请求失败。","",http.StatusBadGateway,result.Err) }
	exec:=result.Value.(execution);meta:=MetadataResult{Executed:exec.Executed&&!result.Deduplicated,Deduplicated:result.Deduplicated,UpstreamStatus:exec.Result.Status,Result:exec.VM}
	return NewResponse(s.cfg.ModelID,actionCall,exec.Message,meta,input.Warnings),nil
}
type execution struct{Message string;VM *infracontrol.VM;Result infracontrol.Result;Executed bool}
func toAPIError(err *infracontrol.Error)*apierror.Error{status:=http.StatusBadGateway;if err.NetworkClass=="timeout"{status=http.StatusGatewayTimeout};return apierror.OpenAI(err.Code,err.Message,"",status,err)}

func newID()string{b:=make([]byte,12);if _,err:=rand.Read(b);err!=nil{return "chatcmpl_000000000000000000000000"};return "chatcmpl_"+hex.EncodeToString(b)}
