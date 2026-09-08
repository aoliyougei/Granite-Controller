package svc

import (
	"needle-controller/internal/config"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/managed"
	"needle-controller/internal/native"
)

type ServiceContext struct {
	Config     config.Config
	Dispatcher *native.Dispatcher
	Managed    *managed.Service
}

func NewServiceContext(cfg config.Config) *ServiceContext {
	probe := native.Request{ToolsJSON: []byte(`[{"name":"readiness_probe","description":"Readiness probe","parameters":{"type":"object","properties":{"value":{"type":"integer"}},"required":["value"],"additionalProperties":false}}]`), ToolNames: []string{"readiness_probe"}, Turns: []native.Turn{{Kind: native.TurnUser, Text: "Set value to 1"}}, MaxNewTokens: 32, ToolIndexPath: cfg.Needle.ToolIndexPath}
	dispatcher := native.NewDispatcher(func() (native.Executor, error) {
		abi, err := native.NewABI()
		if err != nil {
			return nil, err
		}
		return native.NewEngine(abi, cfg.Needle.BufferSize), nil
	}, probe, cfg.Needle.MaxQueueDepth)
	service := managed.NewService(dispatcher, infracontrol.NewClient(cfg.InfraControl), cfg.Needle)
	return &ServiceContext{Config: cfg, Dispatcher: dispatcher, Managed: service}
}
