package main

import (
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"

	"needle-controller/internal/config"
	"needle-controller/internal/handler"
	"needle-controller/internal/svc"
)

var configFile = flag.String("f", "etc/needle-controller.yaml", "configuration file")

func main() {
	flag.Parse()
	cfg, err := config.Load(*configFile)
	if err != nil {
		logx.Must(err)
	}
	server := rest.MustNewServer(cfg.RestConf)
	defer server.Stop()
	handler.Register(server, svc.NewServiceContext(cfg))
	fmt.Printf("needle-controller listening on %s:%d\n", cfg.Host, cfg.Port)
	server.Start()
}
