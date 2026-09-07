package svc

import (
	"needle-controller/internal/config"
	"needle-controller/internal/infracontrol"
	"needle-controller/internal/logic"
	"needle-controller/internal/needle"
)

type ServiceContext struct {
	Config       config.Config
	Chat         logic.ChatService
	Needle       needle.Service
	InfraControl infracontrol.Service
}

func NewServiceContext(cfg config.Config) *ServiceContext {
	needleClient := needle.NewClient(cfg.Needle)
	infraClient := infracontrol.NewClient(cfg.InfraControl)
	return &ServiceContext{
		Config:       cfg,
		Chat:         logic.NewChatService(needleClient, infraClient, cfg.Needle.MinConfidence, cfg.Needle.MaxMessageLength),
		Needle:       needleClient,
		InfraControl: infraClient,
	}
}
