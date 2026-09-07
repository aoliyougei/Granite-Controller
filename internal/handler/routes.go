package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"

	"needle-controller/internal/middleware"
	"needle-controller/internal/svc"
)

func Register(server *rest.Server, ctx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/healthz", Handler: Health().ServeHTTP},
		{Method: http.MethodGet, Path: "/readyz", Handler: Ready(ctx.Needle, ctx.InfraControl).ServeHTTP},
	})
	auth := middleware.NewBearerAuth(ctx.Config.ControllerAPIToken)
	server.AddRoutes(rest.WithMiddlewares([]rest.Middleware{
		func(next http.HandlerFunc) http.HandlerFunc { return auth(next).ServeHTTP },
	}, rest.Route{Method: http.MethodPost, Path: "/api/v1/chat", Handler: Chat(ctx.Chat).ServeHTTP}))
}
