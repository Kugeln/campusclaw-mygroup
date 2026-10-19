package main

import (
	"campusclaw/internal/auth"
	"campusclaw/internal/config"
	"campusclaw/internal/conversation"
	"campusclaw/internal/health"
	httpapi "campusclaw/internal/http"
	"campusclaw/internal/material"
	"campusclaw/internal/mcp"
	"campusclaw/internal/rag"
	"campusclaw/internal/recovery"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"campusclaw/internal/transfer"
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"
)

// @title CampusClaw Go API
// @version week5-aligned
// @description 处理器注解生成的实际契约。Bearer每请求核验当前身份；Week4 Ask无据零模型调用，Week5无据仍生成。材料UUID、1-based切片；无公开会话DELETE。
// @BasePath /
// @securityDefinitions.apikey bearerAuth
// @in header
// @name Authorization
// @description HS256 JWT，8小时；退出撤销账号旧Token。转换产物使用HTTP bearer，输入JWT本身。
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
	rc, err := config.LoadRAG(os.Getenv)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	mc, err := mcp.LoadConfig(os.Getenv)
	if err != nil {
		log.Print("MCP configuration invalid")
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
	var ragHandler *httpapi.RAGHandler
	if rc.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = repository.ValidateRAGSchema(ctx, db)
		if err == nil {
			err = transfer.VerifyMCPSchemaReceipt(ctx, db, c.UploadRoot)
		}
		store := repository.RAG{DB: db}
		indexer := &rag.Indexer{Store: store, Embedding: rc.Embedding, Vectors: rc.Vectors}
		if err == nil {
			err = indexer.Recover(ctx)
		}
		cancel()
		if err != nil {
			log.Print("RAG schema or index recovery failed")
			os.Exit(1)
		}
		ragHandler = &httpapi.RAGHandler{Retrieval: rag.Retrieval{Store: store, Embedding: rc.Embedding, Vectors: rc.Vectors}, Dialogue: rc.Dialogue, Indexer: indexer}
	}
	var identity *auth.Identity
	var upload *httpapi.UploadHandler
	var reads *httpapi.MaterialReads
	var tutorHandler *httpapi.TutorHandler
	var conversationHandler *httpapi.ConversationHandler
	if rc.Enabled {
		tutorHandler = &httpapi.TutorHandler{Store: repository.Tutor{DB: db}}
		conversationStore := repository.Conversations{DB: db}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = conversationStore.Recover(ctx)
		cancel()
		if err != nil {
			log.Print("conversation recovery failed")
			os.Exit(1)
		}
		conversationHandler = &httpapi.ConversationHandler{Store: conversationStore, Tutor: repository.Tutor{DB: db}, Retrieval: ragHandler.Retrieval, Model: conversation.StreamingModel{Gateway: rc.Dialogue.Gateway}}
	}
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
		if ragHandler != nil {
			upload.Indexer = ragHandler.Indexer
		}
		reads = &httpapi.MaterialReads{Store: files, Reader: material.Reader{Repository: repository.Materials{DB: db}}}
	}
	var mcpHandler *httpapi.MCPHandler
	if mc.Enabled {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = repository.ValidateMCPSchema(ctx, db)
		if err == nil {
			err = transfer.VerifyMCPSchemaReceipt(ctx, db, c.UploadRoot)
		}
		cancel()
		if err != nil || identity == nil {
			log.Print("MCP schema, receipt or identity unavailable")
			os.Exit(1)
		}
		runtimeStore := repository.MCP{DB: db, Vault: mc.Vault, Policy: mc.Policy, Secrets: []string{os.Getenv("MCP_MASTER_KEY"), os.Getenv("JWT_SIGNING_SECRET"), os.Getenv("DIALOGUE_API_KEY"), os.Getenv("EMBEDDING_API_KEY"), rc.Dialogue.Gateway.Key}}
		mcpHandler = &httpapi.MCPHandler{Store: runtimeStore}
		if conversationHandler != nil {
			conversationHandler.Tools = mcp.Runner{Store: runtimeStore}
		}
	}
	server := http.Server{Addr: c.Listen, Handler: httpapi.WithMCP(httpapi.WithConversations(service, login, identity, upload, reads, ragHandler, tutorHandler, conversationHandler), identity, mcpHandler), ReadHeaderTimeout: 5 * time.Second}
	log.Print("server ready; material APIs enabled")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print("http server failed")
		os.Exit(1)
	}
}
