package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aoliyougei/granite-controller/internal/config"
	"github.com/aoliyougei/granite-controller/internal/granite"
	"github.com/aoliyougei/granite-controller/internal/handler"
	"github.com/aoliyougei/granite-controller/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/granite-controller.yaml", "configuration file")

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := checkHealth("http://127.0.0.1:8080/healthz", 3*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		logx.Error(err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}
	var manager *granite.Manager
	probeClient := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	manager = granite.NewManager(granite.ProcessConfig{Binary: "/opt/granite/llama-server", Model: "/opt/granite/granite-4.0-350m-Q4_K_M.gguf", Threads: cfg.Granite.Threads, ContextSize: cfg.Granite.ContextSize, StartupTimeout: cfg.Granite.StartupTimeout}, func(ctx context.Context) bool {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:18080/health", nil)
		req.Header.Set("Authorization", "Bearer "+manager.APIKey())
		resp, e := probeClient.Do(req)
		if e != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
	server := rest.MustNewServer(cfg.RestConf)
	serviceContext := svc.NewServiceContext(cfg, manager)
	handler.Register(server, serviceContext)
	go func() {
		if err := manager.Start(context.Background()); err != nil {
			exitOnGraniteFailure(err, os.Exit)
			return
		}
		<-manager.Wait()
		if manager.Stopping() {
			return
		}
		if err := manager.ExitError(); err != nil {
			exitOnGraniteFailure(fmt.Errorf("llama-server exited: %w", err), os.Exit)
		} else {
			exitOnGraniteFailure(fmt.Errorf("llama-server exited"), os.Exit)
		}
	}()
	fmt.Printf("granite-controller listening on %s:%d\n", cfg.Host, cfg.Port)
	server.Start()
	return manager.Stop()
}

func exitOnGraniteFailure(err error, exit func(int)) {
	logx.Error(err)
	exit(1)
}
func checkHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned status %d", response.StatusCode)
	}
	return nil
}
