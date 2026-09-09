package handler

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/aoliyougei/granite-controller/internal/middleware"
	"github.com/aoliyougei/granite-controller/internal/svc"
	"net/http"
)

func Register(server *rest.Server, ctx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{{Method: http.MethodGet, Path: "/healthz", Handler: Health().ServeHTTP}, {Method: http.MethodGet, Path: "/readyz", Handler: Ready(ctx.Managed).ServeHTTP}})
	auth := middleware.NewBearerAuth(ctx.Config.APIKey)
	protected := []rest.Route{{Method: http.MethodGet, Path: "/v1/models", Handler: Models(ctx.Managed).ServeHTTP}, {Method: http.MethodPost, Path: "/v1/chat/completions", Handler: ChatCompletions(ctx.Managed).ServeHTTP}}
	server.AddRoutes(rest.WithMiddlewares([]rest.Middleware{func(next http.HandlerFunc) http.HandlerFunc { return auth(next).ServeHTTP }}, protected...))
}
