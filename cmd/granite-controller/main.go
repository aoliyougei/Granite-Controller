package main

import (
 "context"
 "flag"
 "fmt"
 "net/http"
 "os"
 "time"

 "github.com/zeromicro/go-zero/core/logx"
 "github.com/zeromicro/go-zero/rest"
 "github.com/aoliyougei/granite-controller/internal/config"
 "github.com/aoliyougei/granite-controller/internal/granite"
 "github.com/aoliyougei/granite-controller/internal/handler"
 "github.com/aoliyougei/granite-controller/internal/svc"
)
var configFile=flag.String("f","etc/granite-controller.yaml","configuration file")
func main(){
 if len(os.Args)>1&&os.Args[1]=="healthcheck"{if err:=checkHealth("http://127.0.0.1:8080/healthz",3*time.Second);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)};return}
 flag.Parse();cfg,err:=config.Load(*configFile);if err!=nil{logx.Must(err)}
 manager:=granite.NewManager(granite.ProcessConfig{Binary:"/opt/granite/llama-server",Model:"/opt/granite/granite-4.0-350m-Q4_K_M.gguf",Threads:cfg.Granite.Threads,ContextSize:cfg.Granite.ContextSize,StartupTimeout:cfg.Granite.StartupTimeout},func(ctx context.Context)bool{
  req,_:=http.NewRequestWithContext(ctx,http.MethodGet,"http://127.0.0.1:18080/health",nil);resp,e:=http.DefaultClient.Do(req);if e!=nil{return false};resp.Body.Close();return resp.StatusCode==http.StatusOK
 })
 if err:=manager.Start(context.Background());err!=nil{logx.Must(err)};defer manager.Stop()
 server:=rest.MustNewServer(cfg.RestConf);defer server.Stop();serviceContext:=svc.NewServiceContext(cfg,manager);handler.Register(server,serviceContext)
 fmt.Printf("granite-controller listening on %s:%d\n",cfg.Host,cfg.Port);server.Start()
}
func checkHealth(url string,timeout time.Duration)error{client:=&http.Client{Timeout:timeout,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}};response,err:=client.Get(url);if err!=nil{return fmt.Errorf("health request failed: %w",err)};defer response.Body.Close();if response.StatusCode!=http.StatusOK{return fmt.Errorf("health returned status %d",response.StatusCode)};return nil}
