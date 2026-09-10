package svc

import (
	"github.com/aoliyougei/granite-controller/internal/config"
	"github.com/aoliyougei/granite-controller/internal/granite"
	"github.com/aoliyougei/granite-controller/internal/infracontrol"
	"github.com/aoliyougei/granite-controller/internal/managed"
)

type ServiceContext struct {
	Config  config.Config
	Managed *managed.Service
	Granite *granite.Manager
}

func NewServiceContext(cfg config.Config, manager *granite.Manager) *ServiceContext {
	client := granite.NewClient("http://127.0.0.1:18080", manager.APIKey(), cfg.Granite.ModelID, cfg.Granite.MaxTokens, cfg.Granite.RequestTimeout)
	service := managed.NewService(client, infracontrol.NewClient(cfg.InfraControl), managed.ServiceConfig{ModelID: cfg.Granite.ModelID, MaxMessageLength: cfg.Granite.MaxMessageLength, AllowForceStop: cfg.Granite.AllowForceStop, DedupWindow: cfg.Granite.ActionDedupWindow, Ready: manager.Ready})
	return &ServiceContext{cfg, service, manager}
}
