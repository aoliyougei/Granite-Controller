package handler

import (
	"log"
	"net/http"

	"github.com/aoliyougei/granite-controller/internal/middleware"
	"github.com/aoliyougei/granite-controller/internal/safelog"
	"github.com/aoliyougei/granite-controller/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func Register(server *rest.Server, ctx *svc.ServiceContext) {
	access := func(next http.HandlerFunc) http.HandlerFunc { return safelog.Access(log.Default())(next).ServeHTTP }
	server.AddRoutes(rest.WithMiddlewares([]rest.Middleware{access}, []rest.Route{{Method: http.MethodGet, Path: "/healthz", Handler: Health().ServeHTTP}, {Method: http.MethodGet, Path: "/readyz", Handler: Ready(ctx.Managed).ServeHTTP}}...))
	auth := middleware.NewBearerAuth(ctx.Config.APIKey)
	protected := []rest.Route{{Method: http.MethodGet, Path: "/v1/models", Handler: Models(ctx.Managed).ServeHTTP}, {Method: http.MethodPost, Path: "/v1/chat/completions", Handler: ChatCompletions(ctx.Managed).ServeHTTP}}
	server.AddRoutes(rest.WithMiddlewares([]rest.Middleware{access, func(next http.HandlerFunc) http.HandlerFunc { return auth(next).ServeHTTP }}, protected...))
}
