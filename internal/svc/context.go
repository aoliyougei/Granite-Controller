package svc

import (
	"needle-controller/internal/config"
	"needle-controller/internal/native"
	"needle-controller/internal/openai"
)

type ServiceContext struct {
	Config     config.Config
	Dispatcher *native.Dispatcher
	OpenAI     *openai.Service
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
	service := openai.NewService(dispatcher, cfg.Needle, openai.DefaultSchemaLimits())
	return &ServiceContext{Config: cfg, Dispatcher: dispatcher, OpenAI: service}
}
