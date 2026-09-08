package native

import (
	"context"
	"errors"
	"runtime"
	"sync"
)

var(
	ErrNotReady=errors.New("native model is not ready")
	ErrQueueFull=errors.New("native engine queue is full")
	ErrClosed=errors.New("native dispatcher is closed")
)

type Executor interface{Execute(Request)(Envelope,error)}
type ExecutorFactory func()(Executor,error)

type job struct{ctx context.Context;request Request;response chan result}
type result struct{envelope Envelope;err error}

type Dispatcher struct{
	queue chan job
	stop chan struct{}
	done chan struct{}
	closeOnce sync.Once
	mu sync.RWMutex
	state State
	failure string
}

func NewDispatcher(factory ExecutorFactory,probe Request,queueDepth int)*Dispatcher{
	d:=&Dispatcher{queue:make(chan job,queueDepth),stop:make(chan struct{}),done:make(chan struct{}),state:StateLoading}
	go d.loop(factory,probe)
	return d
}

func(d *Dispatcher)loop(factory ExecutorFactory,probe Request){
	runtime.LockOSThread()
	defer runtime.UnlockOSThread();defer close(d.done)
	executor,err:=factory()
	if err!=nil{d.fail("native engine construction failed");return}
	if _,err=executor.Execute(probe);err!=nil{d.fail("native engine initialization probe failed");return}
	d.setState(StateReady,"")
	for{
		select{
		case <-d.stop:return
		case task:=<-d.queue:
			if err:=task.ctx.Err();err!=nil{task.response<-result{err:err};continue}
			envelope,err:=executor.Execute(task.request)
			if isFatal(err){d.fail("native engine entered a failed state")}
			task.response<-result{envelope:envelope,err:err}
		}
	}
}

func(d *Dispatcher)Submit(ctx context.Context,request Request)(Envelope,error){
	if d.State()!=StateReady{return Envelope{},ErrNotReady}
	response:=make(chan result,1);task:=job{ctx:ctx,request:request,response:response}
	select{case <-d.stop:return Envelope{},ErrClosed;case d.queue<-task:default:return Envelope{},ErrQueueFull}
	select{case <-ctx.Done():return Envelope{},ctx.Err();case got:=<-response:return got.envelope,got.err}
}

func(d *Dispatcher)State()State{d.mu.RLock();defer d.mu.RUnlock();return d.state}
func(d *Dispatcher)FailureSummary()string{d.mu.RLock();defer d.mu.RUnlock();return d.failure}
func(d *Dispatcher)QueueDepth()int{return len(d.queue)}
func(d *Dispatcher)setState(state State,failure string){d.mu.Lock();defer d.mu.Unlock();d.state=state;d.failure=failure}
func(d *Dispatcher)fail(summary string){d.setState(StateFailed,summary)}
func(d *Dispatcher)Close(){d.closeOnce.Do(func(){close(d.stop);<-d.done})}

func isFatal(err error)bool{var nativeErr *NativeError;return errors.As(err,&nativeErr)&&nativeErr.Fatal}
