package main

import (
	"campusclaw/mcp-server/internal/server"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func run() int {
	addr := os.Getenv("MCP_LISTEN_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://127.0.0.1:8090/health")
		if err != nil {
			return 1
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return 1
		}
		return 0
	}
	var origins []string
	if raw := os.Getenv("MCP_ALLOWED_ORIGINS"); raw != "" {
		origins = strings.Split(raw, ",")
	}
	h, err := server.New(server.Config{Token: os.Getenv("MCP_AUTH_TOKEN"), AllowedOrigins: origins}, server.Builtins())
	if err != nil {
		fmt.Fprintln(os.Stderr, "MCP configuration invalid: service credential or allowed origin missing/invalid")
		return 1
	}
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	// No request, credential, payload or configuration logging.
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "MCP listener failed")
		return 1
	}
	return 0
}

func main() { os.Exit(run()) }
