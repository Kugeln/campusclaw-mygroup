package main

import (
	"campusclaw/internal/config"
	"campusclaw/internal/health"
	httpapi "campusclaw/internal/http"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"campusclaw/internal/transfer"
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 5 * time.Second}
		r, e := client.Get("http://127.0.0.1:8080/health")
		if e != nil {
			os.Exit(1)
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	db, err := repository.Open(c)
	if err != nil {
		log.Print("database configuration failed")
		os.Exit(1)
	}
	defer db.Close()
	// The explicit infrastructure mode is only for the Phase 1 health scaffold.
	// Normal startup requires offline initialization evidence before any file probe.
	if os.Getenv("STARTUP_MODE") != "infrastructure" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = transfer.Ready(ctx, db, c.UploadRoot)
		cancel()
		if err != nil {
			log.Print("startup initialization gate failed")
			os.Exit(1)
		}
	}
	service := health.Service{DB: db, Probe: func() error { return storage.Probe(c.UploadRoot) }}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = db.PingContext(ctx)
	cancel()
	if err != nil {
		log.Printf("startup dependencies unavailable: database (%T, code=%d)", err, repository.ErrorCode(err))
		os.Exit(1)
	}
	if err = storage.Probe(c.UploadRoot); err != nil {
		log.Printf("startup dependencies unavailable: storage (%T)", err)
		os.Exit(1)
	}
	server := http.Server{Addr: c.Listen, Handler: httpapi.Handler(service), ReadHeaderTimeout: 5 * time.Second}
	log.Print("infrastructure server ready; business routes are not implemented")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print("http server failed")
		os.Exit(1)
	}
}
