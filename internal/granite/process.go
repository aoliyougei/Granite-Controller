package granite

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "fmt"
 "os/exec"
 "strconv"
 "sync"
 "sync/atomic"
 "time"
)

type ProcessConfig struct{Binary,Model string;Threads,ContextSize int;StartupTimeout time.Duration}
type Manager struct{cfg ProcessConfig;probe func(context.Context)bool;cmd *exec.Cmd;cancel context.CancelFunc;ready atomic.Bool;done chan error;mu sync.Mutex;key string}
func NewManager(cfg ProcessConfig,probe func(context.Context)bool)*Manager{return &Manager{cfg:cfg,probe:probe,done:make(chan error,1)}}
func(m *Manager)Start(ctx context.Context)error{
 keyBytes:=make([]byte,32);if _,err:=rand.Read(keyBytes);err!=nil{return err};m.key=hex.EncodeToString(keyBytes)
 processCtx,cancel:=context.WithCancel(context.Background());m.cancel=cancel
 args:=[]string{"--model",m.cfg.Model,"--host","127.0.0.1","--port","18080","--ctx-size",strconv.Itoa(m.cfg.ContextSize),"--parallel","1","--threads",strconv.Itoa(m.cfg.Threads),"--jinja","--api-key",m.key}
 m.cmd=exec.CommandContext(processCtx,m.cfg.Binary,args...);if err:=m.cmd.Start();err!=nil{cancel();return err}
 go func(){m.done<-m.cmd.Wait();close(m.done)}()
 deadline:=time.NewTimer(m.cfg.StartupTimeout);defer deadline.Stop();ticker:=time.NewTicker(10*time.Millisecond);defer ticker.Stop()
 for{select{case err:=<-m.done:cancel();return fmt.Errorf("llama-server exited: %w",err);case <-deadline.C:cancel();return fmt.Errorf("llama-server startup timeout");case <-ctx.Done():cancel();return ctx.Err();case <-ticker.C:if m.probe(ctx){m.ready.Store(true);return nil}}}
}
func(m *Manager)Ready()bool{return m.ready.Load()}
func(m *Manager)APIKey()string{return m.key}
func(m *Manager)Wait()<-chan error{return m.done}
func(m *Manager)Stop()error{m.mu.Lock();defer m.mu.Unlock();m.ready.Store(false);if m.cancel==nil{return nil};m.cancel();m.cancel=nil;return nil}
