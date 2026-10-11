package main

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/health"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/material"
	"campusclaw/internal/recovery"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"context"
	"database/sql"
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
	var tokens *auth.Tokens
	limit, err := config.UploadLimit(os.Getenv)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	if os.Getenv("STARTUP_MODE") != "infrastructure" {
		secret, secretErr := config.SigningSecret(os.Getenv)
		if secretErr != nil {
			log.Print(secretErr)
			os.Exit(1)
		}
		tokens, err = auth.NewTokens(secret, time.Now)
		if err != nil {
			log.Print("authentication configuration failed")
			os.Exit(1)
		}
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
		err = recovery.Startup(ctx, db, c.UploadRoot)
		cancel()
		if err != nil {
			log.Print("startup initialization or material recovery failed; preserve storage and restore consistency")
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
	var login *auth.Login
	var identity *auth.Identity
	var upload *httpapi.UploadHandler
	var reads *httpapi.MaterialReads
	if tokens != nil {
		login = &auth.Login{Users: repository.LoginUsers{DB: db}, Tokens: tokens}
		identity = &auth.Identity{Store: repository.LoginUsers{DB: db}, Tokens: tokens}
		files, err := storage.OpenPrivate(c.UploadRoot)
		if err != nil {
			log.Print("private storage initialization failed")
			os.Exit(1)
		}
		defer files.Close()
		uploader := material.Uploader{Repository: repository.Materials{DB: db, Reconnect: func() (*sql.DB, error) { return repository.Open(c) }}}
		upload = &httpapi.UploadHandler{Store: files, Uploader: uploader, Limit: limit}
		reads = &httpapi.MaterialReads{Store: files, Reader: material.Reader{Repository: repository.Materials{DB: db}}}
	}
	server := http.Server{Addr: c.Listen, Handler: httpapi.WithMaterials(service, login, identity, upload, reads), ReadHeaderTimeout: 5 * time.Second}
	log.Print("server ready; material APIs enabled")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print("http server failed")
		os.Exit(1)
	}
}
