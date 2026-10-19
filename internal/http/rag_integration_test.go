//go:build ragintegration

package httpapi

import (
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/material"
	"campusclaw/internal/provision"
	"campusclaw/internal/rag"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type healthyRAG struct{}

func (healthyRAG) Check(context.Context) error { return nil }
func TestGoRAGRealServices(t *testing.T) {
	ctx := context.Background()
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if db.QueryRow("SELECT DATABASE()").Scan(&name) != nil || name != "week5_rag_test" {
		t.Fatal("isolated database required")
	}
	if e = repository.PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = repository.PrepareRAGSchema(ctx, db); e != nil {
		t.Fatal("idempotent upgrade", e)
	}
	root := os.Getenv("WEEK5_UPLOAD_ROOT")
	if root != "/uploads" {
		t.Fatal("isolated volume required")
	}
	password := os.Getenv("WEEK5_ACCOUNT_PASSWORD")
	if password == "" {
		t.Fatal("test account password required")
	}
	if _, e = provision.Fresh(ctx, db, root, func(string) string { return password }); e != nil {
		t.Fatal(e)
	}
	files, e := storage.OpenPrivate(root)
	if e != nil {
		t.Fatal(e)
	}
	defer files.Close()
	var mu sync.Mutex
	embeds, dialogues := 0, 0
	failEmbedding := false
	failAt := 0
	badAnswer := false
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("gateway body")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/embeddings", "/embeddings/multimodal":
			embeds++
			if failEmbedding || (failAt > 0 && embeds == failAt) {
				w.WriteHeader(503)
				return
			}
			text, _ := body["input"].(string)
			if text == "" {
				inputs, _ := body["input"].([]any)
				if len(inputs) == 1 {
					item, _ := inputs[0].(map[string]any)
					text, _ = item["text"].(string)
				}
			}
			v := []float64{0, 0, 1}
			if strings.Contains(text, "星河") {
				v = []float64{1, 0, 0}
			}
			if strings.Contains(text, "海风") {
				v = []float64{0, 1, 0}
			}
			if r.URL.Path == "/embeddings" {
				json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": v}}})
			} else {
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"embedding": v}})
			}
		case "/chat/completions":
			dialogues++
			messages, _ := body["messages"].([]any)
			if len(messages) != 2 {
				t.Error("dialogue shape")
			}
			if len(messages) == 2 {
				message, _ := messages[1].(map[string]any)
				content, _ := message["content"].(string)
				var input struct {
					Evidence []map[string]any `json:"evidence"`
				}
				if json.Unmarshal([]byte(content), &input) != nil || len(input.Evidence) == 0 || len(input.Evidence) > 4 {
					t.Error("top4 evidence input")
				}
				for _, item := range input.Evidence {
					if strings.Contains(item["chunk_text"].(string), "海风") {
						t.Error("foreign evidence in model input")
					}
				}
			}
			b, _ := json.Marshal(body)
			if strings.Contains(string(b), "vector") {
				t.Error("vector leaked into dialogue")
			}
			text := "本班资料说明活动安排 [1]"
			if badAnswer {
				text = "越界 [999]"
			}
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": text}}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer gateway.Close()
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: gateway.URL, Key: "controlled-test", Model: "test"}, Dimension: 3, Format: "openai"}
	vector := rag.VectorClient{Gateway: rag.Gateway{Base: "http://qdrant:6333"}, Dimension: 3, Collection: "week5_go_" + time.Now().Format("150405")}
	store := repository.RAG{DB: db}
	indexer := &rag.Indexer{Store: store, Embedding: embedding, Vectors: vector}
	retrieval := rag.Retrieval{Store: store, Embedding: embedding, Vectors: vector}
	secret := make([]byte, 32)
	rand.Read(secret)
	tokens, e := auth.NewTokens(hex.EncodeToString(secret), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	login := &auth.Login{Users: repository.LoginUsers{DB: db}, Tokens: tokens}
	identity := &auth.Identity{Store: repository.LoginUsers{DB: db}, Tokens: tokens}
	upload := &UploadHandler{Store: files, Uploader: material.Uploader{Repository: repository.Materials{DB: db}}, Indexer: indexer}
	reads := &MaterialReads{Store: files, Reader: material.Reader{Repository: repository.Materials{DB: db}}}
	server := httptest.NewServer(WithRAG(healthyRAG{}, login, identity, upload, reads, &RAGHandler{Retrieval: retrieval, Dialogue: rag.DialogueClient{Gateway: rag.Gateway{Base: gateway.URL, Key: "controlled-test", Model: "test"}}, Indexer: indexer}))
	defer server.Close()
	baseURL := server.URL
	request := func(method, path, token string, body io.Reader, contentType string) (int, map[string]any) {
		req, e := http.NewRequest(method, baseURL+path, body)
		if e != nil {
			t.Fatal(e)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		data := map[string]any{}
		b, _ := io.ReadAll(res.Body)
		_ = json.Unmarshal(b, &data)
		return res.StatusCode, data
	}
	jsonRequest := func(method, path, token string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		return request(method, path, token, bytes.NewReader(b), "application/json")
	}
	creds := map[string]string{}
	for _, name := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		code, v := jsonRequest("POST", "/api/auth/login", "", map[string]any{"username": name, "password": password})
		if code != 200 {
			t.Fatal("login", name, code)
		}
		creds[name], _ = v["access_token"].(string)
	}
	if code, _ := jsonRequest("POST", "/api/auth/login", "", map[string]any{"username": "teacher_a", "password": "wrong"}); code != 401 {
		t.Fatal("wrong login")
	}
	if code, _ := request("GET", "/api/search?q=x", "", nil, ""); code != 401 {
		t.Fatal("anonymous")
	}
	put := func(token, text string) (int, map[string]any) {
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		p, _ := writer.CreateFormFile("file", "lesson.md")
		p.Write([]byte(text))
		writer.Close()
		return request("POST", "/api/materials", token, &b, writer.FormDataContentType())
	}
	ids := map[string]string{}
	for _, group := range []struct{ name, text string }{{"teacher_a", strings.Repeat("星河活动周三在图书馆举行。", 95)}, {"teacher_b", strings.Repeat("海风活动周五在实验楼举行。", 95)}} {
		code, v := put(creds[group.name], group.text)
		if code != 201 || v["index_status"] != "ready" {
			t.Fatal("upload/index", code, v)
		}
		ids[group.name], _ = v["material_id"].(string)
	}
	if code, _ := put(creds["student_a1"], "星河"); code != 403 {
		t.Fatal("student upload")
	}
	for _, role := range []string{"teacher_a", "student_a1", "teacher_b", "student_b1"} {
		q, foreign := "星河", "海风"
		if strings.HasSuffix(role, "_b") || role == "student_b1" {
			q, foreign = foreign, q
		}
		for _, mode := range []string{"keyword", "vector", "hybrid"} {
			code, v := request("GET", "/api/search?q="+q+"&mode="+mode, creds[role], nil, "")
			if code != 200 {
				t.Fatal(role, mode, code, v)
			}
			items, _ := v["items"].([]any)
			if len(items) == 0 {
				t.Fatal("no expected evidence", role, mode)
			}
			for _, item := range items {
				hit := item.(map[string]any)
				var text, title string
				var index int
				if db.QueryRow("SELECT c.chunk_text,m.original_name,c.chunk_index FROM knowledge_chunks c JOIN materials m ON m.id=c.material_id WHERE c.id=?", hit["chunk_id"]).Scan(&text, &title, &index) != nil || hit["chunk_text"] != text || hit["title"] != title || hit["chunk_index"] != float64(index) {
					t.Fatal("untrue evidence")
				}
			}
			code, v = request("GET", "/api/search?q="+foreign+"&mode="+mode+"&class_id=999", creds[role], nil, "")
			if code != 200 || len(v["items"].([]any)) != 0 {
				t.Fatal("class isolation", role, mode, v)
			}
		}
	}
	code, answer := jsonRequest("POST", "/api/ask", creds["student_a1"], map[string]any{"question": "星河活动", "mode": "hybrid"})
	if code != 200 || len(answer["sources"].([]any)) == 0 {
		t.Fatal("ask", code, answer)
	}
	mu.Lock()
	before := dialogues
	mu.Unlock()
	code, answer = jsonRequest("POST", "/api/ask", creds["student_a1"], map[string]any{"question": "无此数据", "mode": "hybrid"})
	mu.Lock()
	after := dialogues
	mu.Unlock()
	if code != 200 || answer["answer"] != rag.NoEvidence || len(answer["sources"].([]any)) != 0 || before != after {
		t.Fatal("no evidence", code, answer)
	}
	mu.Lock()
	badAnswer = true
	mu.Unlock()
	if code, _ := jsonRequest("POST", "/api/ask", creds["student_a1"], map[string]any{"question": "星河", "mode": "keyword"}); code != 502 {
		t.Fatal("citation validation")
	}
	mu.Lock()
	badAnswer = false
	mu.Unlock()
	if code, _ := jsonRequest("POST", "/api/materials/"+ids["teacher_a"]+"/reindex", creds["student_a1"], map[string]any{}); code != 403 {
		t.Fatal("student reindex")
	}
	if code, _ := jsonRequest("POST", "/api/materials/"+ids["teacher_b"]+"/reindex", creds["teacher_a"], map[string]any{}); code != 404 {
		t.Fatal("foreign reindex")
	}
	if code, v := jsonRequest("POST", "/api/materials/"+ids["teacher_a"]+"/reindex", creds["teacher_a"], map[string]any{"size": 400, "overlap": 40}); code != 200 || v["index_status"] != "ready" {
		t.Fatal("rebuild", code, v)
	}
	mu.Lock()
	failEmbedding = true
	mu.Unlock()
	code, v := put(creds["teacher_a"], "星河原文故障也须保存")
	if code != 201 || v["index_status"] != "failed" {
		t.Fatal("failed index preserves upload", code, v)
	}
	if code, _ := request("GET", "/api/materials/"+v["material_id"].(string), creds["student_a1"], nil, ""); code != 200 {
		t.Fatal("original retained")
	}
	mu.Lock()
	failEmbedding = false
	mu.Unlock()
	// Fail on the second chunk: none of the first chunk's vectors may publish.
	mu.Lock()
	failAt = embeds + 2
	mu.Unlock()
	code, v = put(creds["teacher_a"], strings.Repeat("星河第二片故障验证。", 180))
	if code != 201 || v["index_status"] != "failed" {
		t.Fatal("second chunk failure", code, v)
	}
	failedID := v["material_id"].(string)
	var state string
	var readyCount int
	if db.QueryRow("SELECT index_status FROM knowledge_entries WHERE material_id=?", failedID).Scan(&state) != nil || state != "failed" {
		t.Fatal("failure state")
	}
	if db.QueryRow("SELECT COUNT(*) FROM knowledge_chunks WHERE material_id=? AND index_status='ready'", failedID).Scan(&readyCount) != nil || readyCount != 0 {
		t.Fatal("partial publication")
	}
	var count struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	b, _ := json.Marshal(map[string]any{"filter": rag.Filter(1, []string{failedID}), "exact": true})
	res, e := http.Post("http://qdrant:6333/collections/"+vector.Collection+"/points/count", "application/json", bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	json.NewDecoder(res.Body).Decode(&count)
	res.Body.Close()
	if count.Result.Count != 0 {
		t.Fatal("vectors published before all embeddings")
	}
	mu.Lock()
	failAt = 0
	mu.Unlock()
	// Simulate an interrupted index in this fixture; original text stays intact.
	if e = store.State(ctx, 1, failedID, "indexing", true); e != nil {
		t.Fatal(e)
	}
	// Recovery only touches this fixture's incomplete indexes.
	if e = indexer.Recover(ctx); e != nil {
		t.Fatal("recovery", e)
	}
	var cleanup bool
	if db.QueryRow("SELECT index_status,cleanup_required FROM knowledge_entries WHERE material_id=?", failedID).Scan(&state, &cleanup) != nil || state != "failed" || cleanup {
		t.Fatal("interrupted recovery state")
	}
	// Reject a mismatching collection dimension before changing the old index.
	wrong := vector
	wrong.Dimension = 4
	wrongIndexer := &rag.Indexer{Store: store, Embedding: embedding, Vectors: wrong}
	if _, e = wrongIndexer.Run(ctx, rag.Scope{ClassID: 1, Role: "teacher"}, ids["teacher_a"], nil, false); e != rag.Unavailable {
		t.Fatal("dimension mismatch")
	}
	if db.QueryRow("SELECT index_status FROM knowledge_entries WHERE material_id=?", ids["teacher_a"]).Scan(&state) != nil || state != "ready" {
		t.Fatal("mismatch hid old index")
	}
	// A faulting proxy preserves the real Qdrant service: acknowledge a write
	// upstream but lose its response, then reject cleanup. Failed chunks must
	// stay invisible and recovery must confirm cleanup before republishing.
	faultProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/points/delete") {
			w.WriteHeader(503)
			return
		}
		req, e := http.NewRequestWithContext(r.Context(), r.Method, "http://qdrant:6333"+r.URL.RequestURI(), r.Body)
		if e != nil {
			w.WriteHeader(503)
			return
		}
		req.Header = r.Header.Clone()
		req.Header.Del("Accept-Encoding")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			w.WriteHeader(503)
			return
		}
		defer res.Body.Close()
		if r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/points") {
			io.Copy(io.Discard, res.Body)
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
	}))
	defer faultProxy.Close()
	faultVectors := vector
	faultVectors.Base = faultProxy.URL
	faultIndexer := &rag.Indexer{Store: store, Embedding: embedding, Vectors: faultVectors}
	// Persist the original through the ordinary upload handler, then publish a
	// fresh index with fault injection on an otherwise valid new UUID.
	code, v = put(creds["teacher_a"], "星河孤立向量故障")
	if code != 201 {
		t.Fatal("fault fixture upload")
	}
	orphanID := v["material_id"].(string)
	if _, e = faultIndexer.Run(ctx, rag.Scope{ClassID: 1, Role: "teacher"}, orphanID, nil, true); e != rag.Unavailable {
		t.Fatal("unconfirmed write accepted")
	}
	if db.QueryRow("SELECT index_status,cleanup_required FROM knowledge_entries WHERE material_id=?", orphanID).Scan(&state, &cleanup) != nil || state != "failed" || !cleanup {
		t.Fatal("unconfirmed cleanup state")
	}
	if evidence, e := retrieval.Search(ctx, rag.Scope{ClassID: 1}, "星河", "vector", []string{orphanID}, 4); e != nil || len(evidence) != 0 {
		t.Fatal("failed vectors visible", evidence, e)
	}
	if e = indexer.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	if db.QueryRow("SELECT cleanup_required FROM knowledge_entries WHERE material_id=?", orphanID).Scan(&cleanup) != nil || cleanup {
		t.Fatal("cleanup not confirmed")
	}
	if _, e = indexer.Run(ctx, rag.Scope{ClassID: 1, Role: "teacher"}, orphanID, nil, false); e != nil {
		t.Fatal("retry after recovery", e)
	}
	broken := retrieval
	broken.Vectors = rag.VectorClient{Gateway: rag.Gateway{Base: "http://127.0.0.1:1"}, Dimension: 3, Collection: vector.Collection}
	if _, e = broken.Search(ctx, rag.Scope{ClassID: 1}, "星河", "keyword", nil, 4); e != nil {
		t.Fatal("keyword independent")
	}
	if _, e = broken.Search(ctx, rag.Scope{ClassID: 1}, "星河", "hybrid", nil, 4); e != rag.Unavailable {
		t.Fatal("no fallback")
	}
	if code, _ := jsonRequest("POST", "/api/auth/logout", creds["student_a1"], nil); code != 204 {
		t.Fatal("logout")
	}
	if code, _ := request("GET", "/api/auth/me", creds["student_a1"], nil, ""); code != 401 {
		t.Fatal("revocation")
	}
	mu.Lock()
	counts := map[string]int{"embeddings": embeds, "dialogues": dialogues}
	mu.Unlock()
	t.Log("PASS isolated Go HTTP/auth/material/index/retrieval/ask/permission/citation/recovery", counts)
	// Verify the real default Go executable rather than only the handler factory.
	server.Close()
	build := exec.Command("go", "build", "-o", "/tmp/week5-go-server", "../../cmd/server")
	if output, e := build.CombinedOutput(); e != nil {
		t.Fatalf("server build: %s", output)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := listener.Addr().String()
	listener.Close()
	command := exec.Command("/tmp/week5-go-server")
	command.Env = append(os.Environ(), "DB_HOST=mysql", "DB_PORT=3306", "DB_NAME=week5_rag_test", "DB_USER=week5_test", "DB_PASSWORD="+os.Getenv("WEEK5_ACCOUNT_PASSWORD"), "UPLOAD_ROOT=/uploads", "LISTEN_ADDR="+addr, "JWT_SIGNING_SECRET="+hex.EncodeToString(secret), "STARTUP_MODE=application", "RAG_ENABLED=true", "QDRANT_URL=http://qdrant:6333", "QDRANT_COLLECTION="+vector.Collection, "EMBEDDING_BASE_URL="+gateway.URL, "EMBEDDING_API_KEY=controlled-test", "EMBEDDING_MODEL=test", "EMBEDDING_DIMENSION=3", "EMBEDDING_API_FORMAT=openai", "DIALOGUE_BASE_URL="+gateway.URL, "DIALOGUE_API_KEY=controlled-test", "DIALOGUE_MODEL=test")
	var logs bytes.Buffer
	command.Stdout = &logs
	command.Stderr = &logs
	if e = command.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { command.Process.Kill(); command.Wait() }()
	baseURL = "http://" + addr
	ready := false
	for start := time.Now(); time.Since(start) < 20*time.Second; {
		res, e := http.Get(baseURL + "/health")
		if e == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("real cmd/server did not become ready")
	}
	if code, v := request("GET", "/api/search?q=星河&mode=hybrid", creds["teacher_a"], nil, ""); code != 200 || len(v["items"].([]any)) == 0 {
		t.Fatal("real Go entry retrieval", code, v)
	}
	if code, v := jsonRequest("POST", "/api/ask", creds["teacher_a"], map[string]any{"question": "星河", "mode": "keyword"}); code != 200 || len(v["sources"].([]any)) == 0 {
		t.Fatal("real Go entry ask", code, v)
	}
	t.Log("PASS actual cmd/server health + authenticated hybrid search + evidence ask using persistent MySQL/files; no Flask/Python/SQLite runtime")
}
