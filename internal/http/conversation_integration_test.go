//go:build conversationintegration

package httpapi

import (
	"bufio"
	"bytes"
	"campusclaw/internal/auth"
	"campusclaw/internal/conversation"
	"campusclaw/internal/material"
	"campusclaw/internal/provision"
	"campusclaw/internal/rag"
	repository "campusclaw/internal/repository/mysql"
	"campusclaw/internal/storage"
	"campusclaw/internal/tutor"
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	mysqldriver "github.com/go-sql-driver/mysql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type conversationHealth struct{}

func (conversationHealth) Check(context.Context) error { return nil }
func TestPrivateStreamingConversations(t *testing.T) {
	ctx := context.Background()
	db, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var database string
	if db.QueryRow("SELECT DATABASE()").Scan(&database) != nil || database != "week5_rag_test" {
		t.Fatal("existing isolated fixture required")
	}
	if e = repository.ValidateRAGSchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	config := repository.Tutor{DB: db}
	sTeacher := rag.Scope{UserID: 1, ClassID: 1, Role: "teacher"}
	skills, e := config.Skills(ctx, sTeacher)
	if e != nil || len(skills) != 1 {
		t.Fatal("existing skill fixture")
	}
	if _, e = config.Toggle(ctx, sTeacher, skills[0].ID, true); e != nil {
		t.Fatal(e)
	}
	// Create only this run's material; reindex must never change older fixtures.
	files, e := storage.OpenPrivate(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer files.Close()
	original := strings.Repeat("星河活动原始来源：星期三图书馆。", 95)
	staged, e := files.Prepare("stream-original.md", strings.NewReader(original), storage.DefaultUploadLimit)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := (repository.Materials{DB: db}).Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if e = tx.Insert(material.Record{UploadInfo: staged.Info(), ClassID: 1, UploaderID: 1, CreatedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")}); e != nil {
		t.Fatal(e)
	}
	if e = staged.Publish(); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	materialID := staged.Info().ID
	agent, e := config.SaveAgent(ctx, sTeacher, 0, tutor.Config{Name: "流式测试-" + time.Now().Format("150405.000000"), Prompt: "第一轮教师原则：耐心引导", Enabled: true, SkillIDs: []int64{skills[0].ID}, MaterialIDs: []string{materialID}})
	if e != nil {
		t.Fatal(e)
	}
	a2Name := "student_a2_stream_" + time.Now().Format("150405")
	_, e = provision.CreateUser(ctx, db, a2Name, os.Getenv("WEEK5_ACCOUNT_PASSWORD"), "student", 1)
	if e != nil {
		t.Fatal(e)
	}
	var mu sync.Mutex
	mode := "gated"
	release := make(chan struct{})
	modelCalls := 0
	seenMessages := [][]conversation.ModelMessage{}
	cancelled := make(chan struct{}, 10)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		if r.URL.Path == "/embeddings" {
			var text string
			json.Unmarshal(body["input"], &text)
			v := []float64{0, 0, 1}
			if strings.Contains(text, "星河") {
				v = []float64{1, 0, 0}
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": v}}})
			return
		}
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(404)
			return
		}
		var streaming bool
		json.Unmarshal(body["stream"], &streaming)
		if !streaming {
			t.Error("missing stream=true")
		}
		var messages []conversation.ModelMessage
		json.Unmarshal(body["messages"], &messages)
		mu.Lock()
		current := mode
		gate := release
		modelCalls++
		seenMessages = append(seenMessages, messages)
		mu.Unlock()
		if current == "preflight" {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		delta := func(text string) {
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]string{"content": text}}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
			w.(http.Flusher).Flush()
		}
		delta("先分析条件，")
		if current == "gated" {
			select {
			case <-gate:
			case <-r.Context().Done():
				cancelled <- struct{}{}
				return
			}
		}
		if current == "disconnect" || current == "timeout" {
			<-r.Context().Done()
			cancelled <- struct{}{}
			return
		}
		if current == "failure" {
			fmt.Fprint(w, "data: {\"error\":{\"message\":\"controlled failure\"}}\n\n")
			return
		}
		time.Sleep(20 * time.Millisecond)
		delta("尝试变换等式。")
		time.Sleep(20 * time.Millisecond)
		if current == "badcite" {
			delta("错误来源 [999]")
		} else {
			delta("请说明下一步 [1]")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer gateway.Close()
	embedding := rag.Embedding{Gateway: rag.Gateway{Base: gateway.URL, Key: "controlled", Model: "test"}, Dimension: 3, Format: "openai"}
	vector := rag.VectorClient{Gateway: rag.Gateway{Base: "http://qdrant:6333"}, Dimension: 3, Collection: "week5_stream_" + strings.ReplaceAll(materialID, "-", "")}
	indexer := &rag.Indexer{Store: repository.RAG{DB: db}, Embedding: embedding, Vectors: vector}
	if state, err := indexer.Run(ctx, sTeacher, materialID, nil, true); err != nil || state != "ready" {
		t.Fatal("new fixture index", state, err)
	}
	retrieval := rag.Retrieval{Store: repository.RAG{DB: db}, Embedding: embedding, Vectors: vector}
	secret := make([]byte, 32)
	rand.Read(secret)
	tokens, _ := auth.NewTokens(hex.EncodeToString(secret), time.Now)
	login := &auth.Login{Users: repository.LoginUsers{DB: db}, Tokens: tokens}
	identity := &auth.Identity{Store: repository.LoginUsers{DB: db}, Tokens: tokens}
	store := repository.Conversations{DB: db}
	handler := &ConversationHandler{Store: store, Tutor: config, Retrieval: retrieval, Model: conversation.StreamingModel{Gateway: rag.Gateway{Base: gateway.URL, Key: "controlled", Model: "test"}}}
	server := httptest.NewServer(WithConversations(conversationHealth{}, login, identity, nil, nil, nil, &TutorHandler{Store: config}, handler))
	defer server.Close()
	request := func(method, path, token string, value any) *http.Response {
		b, _ := json.Marshal(value)
		r, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		return res
	}
	object := func(res *http.Response) map[string]any {
		defer res.Body.Close()
		v := map[string]any{}
		json.NewDecoder(res.Body).Decode(&v)
		return v
	}
	creds := map[string]string{}
	for _, user := range []string{"teacher_a", "student_a1", "student_b1", a2Name} {
		res := request("POST", "/api/auth/login", "", map[string]any{"username": user, "password": os.Getenv("WEEK5_ACCOUNT_PASSWORD")})
		if res.StatusCode != 200 {
			t.Fatal("fixture login", user)
		}
		creds[user] = object(res)["access_token"].(string)
	}
	if res := request("GET", "/api/conversations", "", nil); res.StatusCode != 401 {
		res.Body.Close()
		t.Fatal("anonymous")
	} else {
		res.Body.Close()
	}
	if res := request("POST", "/api/conversations", creds["teacher_a"], map[string]any{"agent_id": agent.ID}); res.StatusCode != 403 {
		res.Body.Close()
		t.Fatal("teacher private conversation")
	} else {
		res.Body.Close()
	}
	create := func() int64 {
		res := request("POST", "/api/conversations", creds["student_a1"], map[string]any{"agent_id": agent.ID})
		if res.StatusCode != 201 {
			t.Fatal("create", res.StatusCode, object(res))
		}
		return int64(object(res)["id"].(float64))
	}
	id := create()
	path := "/api/conversations/" + strconv.FormatInt(id, 10)
	for _, token := range []string{creds[a2Name], creds["student_b1"]} {
		for _, method := range []string{"GET", "POST"} {
			p := path
			if method == "POST" {
				p += "/messages"
			}
			res := request(method, p, token, map[string]any{"content": "星河问题"})
			if res.StatusCode != 404 {
				res.Body.Close()
				t.Fatal("private scope", method, res.StatusCode)
			}
			res.Body.Close()
		}
	}
	res := request("POST", path+"/messages", creds["student_a1"], map[string]any{"content": "星河问题"})
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatal("stream preflight", res.StatusCode, object(res))
	}
	scanner := bufio.NewScanner(res.Body)
	events := []string{}
	firstRead := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
		if strings.HasPrefix(line, "data: ") && len(events) > 0 && events[len(events)-1] == "delta" {
			firstRead = true
			break
		}
	}
	if !firstRead {
		t.Fatal("first delta unavailable while upstream held open")
	}
	scopeA := rag.Scope{UserID: 2, ClassID: 1, Role: "student"}
	_, messages, e := store.Get(ctx, scopeA, id)
	if e != nil || len(messages) != 2 || messages[1].State != "generating" || !strings.Contains(messages[1].Content, "先分析条件") || messages[1].SkillName != tutor.SkillName {
		t.Fatal("delta not persisted before delivery", messages, e)
	}
	initialRevision := messages[1].PromptRevision
	_, e = config.SaveAgent(ctx, sTeacher, agent.ID, tutor.Config{Name: agent.Name, Prompt: "第二轮教师原则：检查理解", Enabled: true, SkillIDs: agent.SkillIDs, MaterialIDs: agent.MaterialIDs})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = config.Toggle(ctx, sTeacher, skills[0].ID, false); e != nil {
		t.Fatal(e)
	}
	if concurrent := request("POST", path+"/messages", creds["student_a1"], map[string]any{"content": "星河并发"}); concurrent.StatusCode != 409 {
		concurrent.Body.Close()
		t.Fatal("concurrent turn allowed")
	} else {
		concurrent.Body.Close()
	}
	close(release)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "event: ") {
			events = append(events, strings.TrimPrefix(scanner.Text(), "event: "))
		}
	}
	res.Body.Close()
	if scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	if strings.Join(events, ",") != "meta,delta,delta,delta,done" {
		t.Fatal("SSE order", events)
	}
	_, messages, e = store.Get(ctx, scopeA, id)
	if e != nil || messages[1].State != "completed" || len(messages[1].Citations) != 1 || messages[1].Snapshot.Agent.Prompt != "第一轮教师原则：耐心引导" {
		t.Fatal("initial audit changed", e)
	}
	source := messages[1].Citations[0]
	var chunk string
	if db.QueryRow("SELECT chunk_text FROM knowledge_chunks WHERE id=?", source.ChunkID).Scan(&chunk) != nil || chunk != source.Text {
		t.Fatal("untrue citation")
	}
	mu.Lock()
	mode = "normal"
	mu.Unlock()
	res = request("POST", path+"/messages", creds["student_a1"], map[string]any{"content": "星河下一步呢"})
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(body, []byte("event: done")) {
		t.Fatal("followup", res.StatusCode, string(body))
	}
	_, messages, e = store.Get(ctx, scopeA, id)
	if e != nil || len(messages) != 4 || messages[3].SkillName != "" || messages[3].PromptRevision != initialRevision+1 || messages[1].SkillName != tutor.SkillName {
		t.Fatal("next-turn snapshots", e)
	}
	if messages[1].SkillHash == "" || messages[1].SkillVersion != tutor.SkillVersion || messages[3].SkillHash != "" {
		t.Fatal("actual skill audit")
	}
	listed := object(request("GET", "/api/conversations", creds["student_a1"], nil))
	found := false
	for _, row := range listed["conversations"].([]any) {
		item := row.(map[string]any)
		if int64(item["id"].(float64)) == id {
			found = true
		}
		if item["user_id"] != float64(2) || item["class_id"] != float64(1) {
			t.Fatal("list owner/class")
		}
	}
	if !found {
		t.Fatal("new conversation missing from list")
	}
	mu.Lock()
	if len(seenMessages) < 2 || len(seenMessages[1]) != 4 || !strings.Contains(seenMessages[0][0].Content, "不得直接给出最终答案") || strings.Contains(seenMessages[1][0].Content, "解题引导约束优先") || !strings.Contains(seenMessages[1][0].Content, "第二轮教师原则") {
		t.Error("prompt/skill/history combined")
	}
	mu.Unlock()
	noID := create()
	mu.Lock()
	before := modelCalls
	mu.Unlock()
	res = request("POST", "/api/conversations/"+strconv.FormatInt(noID, 10)+"/messages", creds["student_a1"], map[string]any{"content": "无此资料"})
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	mu.Lock()
	after := modelCalls
	mu.Unlock()
	if res.StatusCode != 200 || !bytes.Contains(body, []byte(rag.NoEvidence)) || before != after {
		t.Fatal("no evidence/zero model", string(body))
	}
	_, emptyMessages, e := store.Get(ctx, scopeA, noID)
	if e != nil || emptyMessages[1].SkillName != "" || emptyMessages[1].ResultStatus != "no_evidence" || len(emptyMessages[1].Citations) != 0 {
		t.Fatal("no evidence persistence", e)
	}
	for _, failure := range []string{"failure", "badcite"} {
		mu.Lock()
		mode = failure
		mu.Unlock()
		fid := create()
		res = request("POST", "/api/conversations/"+strconv.FormatInt(fid, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河故障"})
		body, _ = io.ReadAll(res.Body)
		res.Body.Close()
		_, failed, e := store.Get(ctx, scopeA, fid)
		if e != nil || !bytes.Contains(body, []byte("event: error")) || bytes.Contains(body, []byte("event: done")) || failed[1].State != "failed" || failed[1].Content == "" {
			t.Fatal("failed stream falsely completed", failure, e, string(body))
		}
	}
	mu.Lock()
	mode = "disconnect"
	mu.Unlock()
	did := create()
	res = request("POST", "/api/conversations/"+strconv.FormatInt(did, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河断连"})
	s := bufio.NewScanner(res.Body)
	for s.Scan() {
		if strings.Contains(s.Text(), "先分析条件") {
			break
		}
	}
	res.Body.Close()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream not cancelled")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, m, e := store.Get(ctx, scopeA, did)
		if e == nil && len(m) == 2 && m[1].State == "interrupted" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnect state", e)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	mode = "preflight"
	mu.Unlock()
	pid := create()
	res = request("POST", "/api/conversations/"+strconv.FormatInt(pid, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河前置故障"})
	if res.StatusCode != 503 {
		t.Fatal("preflight status", res.StatusCode)
	}
	res.Body.Close()
	_, m, e := store.Get(ctx, scopeA, pid)
	if e != nil || len(m) != 0 {
		t.Fatal("preflight left orphan messages", e)
	}
	// Timeout after a durably sent prefix is failed, rather than completed or a
	// successful no-evidence response. Timeout is short only in this test handler.
	mu.Lock()
	mode = "timeout"
	mu.Unlock()
	handler.Timeout = 500 * time.Millisecond
	tid := create()
	res = request("POST", "/api/conversations/"+strconv.FormatInt(tid, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河超时"})
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	handler.Timeout = 0
	_, timeoutMessages, e := store.Get(ctx, scopeA, tid)
	if e != nil || len(timeoutMessages) != 2 || timeoutMessages[1].State != "failed" || bytes.Contains(body, []byte("event: done")) {
		t.Fatal("timeout state", e)
	}
	// Simulate a crash after the initial pair/prefix commit. Startup recovery
	// must not mutate completed history; a fresh DB connection verifies storage.
	crashID := create()
	crashConversation, _, e := store.Get(ctx, scopeA, crashID)
	if e != nil {
		t.Fatal(e)
	}
	p, e := tutor.PrepareTurn(ctx, config, retrieval, scopeA, agent.ID, "星河", "hybrid")
	if e != nil {
		t.Fatal(e)
	}
	turn, e := store.Start(ctx, scopeA, crashConversation, "星河中断", p)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Append(ctx, scopeA, crashID, turn, "已提交前缀"); e != nil {
		t.Fatal(e)
	}
	if e = store.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	reopened, e := sql.Open("mysql", os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	_, recovered, e := (repository.Conversations{DB: reopened}).Get(ctx, scopeA, crashID)
	if e != nil || recovered[1].State != "interrupted" || recovered[1].Content != "已提交前缀" {
		t.Fatal("restart persistence", e)
	}
	_, completed, e := store.Get(ctx, scopeA, id)
	if e != nil || completed[1].State != "completed" || completed[1].PromptRevision != initialRevision {
		t.Fatal("recovery changed history", e)
	}
	// Reindex only the newly created material with deliberately different content.
	oldSourceJSON, _ := json.Marshal(messages[1].Citations)
	oldEvidenceJSON, _ := json.Marshal(messages[1].Evidence)
	if _, e = db.Exec("UPDATE knowledge_entries SET plain_text=? WHERE material_id=? AND class_id=1", strings.Repeat("星河重建新版内容：星期五实验楼。", 95), materialID); e != nil {
		t.Fatal(e)
	}
	if state, err := indexer.Run(ctx, sTeacher, materialID, nil, false); err != nil || state != "ready" {
		t.Fatal("real Qdrant reindex", state, err)
	}
	var oldCount int
	if db.QueryRow("SELECT COUNT(*) FROM knowledge_chunks WHERE id=?", source.ChunkID).Scan(&oldCount) != nil || oldCount != 0 {
		t.Fatal("old chunk was not replaced")
	}
	_, afterReindex, e := (repository.Conversations{DB: reopened}).Get(ctx, scopeA, id)
	newSourceJSON, _ := json.Marshal(afterReindex[1].Citations)
	newEvidenceJSON, _ := json.Marshal(afterReindex[1].Evidence)
	if e != nil || !bytes.Equal(oldSourceJSON, newSourceJSON) || !bytes.Equal(oldEvidenceJSON, newEvidenceJSON) || !strings.Contains(afterReindex[1].Citations[0].Text, "原始来源") {
		t.Fatal("historical citation was repointed", e)
	}
	currentEvidence, e := retrieval.Search(ctx, scopeA, "星河", "hybrid", []string{materialID}, 4)
	if e != nil || len(currentEvidence) == 0 || currentEvidence[0].ChunkID == source.ChunkID || !strings.Contains(currentEvidence[0].Text, "新版内容") {
		t.Fatal("reindex not visible", e)
	}
	// Ordinary repeated sends create exactly two distinct sequential pairs.
	mu.Lock()
	mode = "normal"
	beforeRepeat := modelCalls
	mu.Unlock()
	repeatID := create()
	for attempt := 0; attempt < 2; attempt++ {
		repeatResponse := request("POST", "/api/conversations/"+strconv.FormatInt(repeatID, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河重复问题"})
		repeatBody, _ := io.ReadAll(repeatResponse.Body)
		repeatResponse.Body.Close()
		if repeatResponse.StatusCode != 200 || bytes.Count(repeatBody, []byte("event: done")) != 1 {
			t.Fatal("repeat stream", attempt)
		}
	}
	_, repeated, e := store.Get(ctx, scopeA, repeatID)
	if e != nil || len(repeated) != 4 || repeated[0].Content != repeated[2].Content || repeated[1].State != "completed" || repeated[3].State != "completed" || repeated[1].ID == repeated[3].ID {
		t.Fatal("repeat overwrote or duplicated pair", e)
	}
	var turnCount, rowCount int
	if db.QueryRow("SELECT COUNT(DISTINCT turn_id),COUNT(*) FROM conversation_messages WHERE conversation_id=?", repeatID).Scan(&turnCount, &rowCount) != nil || turnCount != 2 || rowCount != 4 {
		t.Fatal("distinct repeated turns")
	}
	mu.Lock()
	repeatCalls := modelCalls - beforeRepeat
	mu.Unlock()
	if repeatCalls != 2 {
		t.Fatal("unexpected replay", repeatCalls)
	}
	t.Log("PASS Phase3 closure: real MySQL/Qdrant reindex preserves historical source snapshots; identical sends create exactly two new turns")
	// Reject invalid extra client configuration without creating messages.
	res = request("POST", path+"/messages", creds["student_a1"], map[string]any{"content": "星河", "prompt": "替换教师"})
	if res.StatusCode != 400 {
		t.Fatal("client prompt accepted")
	}
	res.Body.Close()
	res = request("GET", path, creds["teacher_a"], nil)
	if res.StatusCode != 403 {
		t.Fatal("teacher reads private messages")
	}
	res.Body.Close()
	// A real MySQL transaction is rolled back instead of committed, exercising
	// atomic pair insertion without damaging the shared isolated server.
	mu.Lock()
	mode = "normal"
	mu.Unlock()
	failID := create()
	connectorConfig, e := mysqldriver.ParseDSN(os.Getenv("WEEK5_TEST_DSN"))
	if e != nil {
		t.Fatal(e)
	}
	connector, e := mysqldriver.NewConnector(connectorConfig)
	if e != nil {
		t.Fatal(e)
	}
	rejectedDB := sql.OpenDB(conversationRejectConnector{connector})
	defer rejectedDB.Close()
	handler.Store = repository.Conversations{DB: rejectedDB}
	res = request("POST", "/api/conversations/"+strconv.FormatInt(failID, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河事务失败"})
	if res.StatusCode != 503 {
		t.Fatal("initial transaction failure", res.StatusCode)
	}
	res.Body.Close()
	handler.Store = store
	_, atomicMessages, e := store.Get(ctx, scopeA, failID)
	if e != nil || len(atomicMessages) != 0 {
		t.Fatal("orphan messages after rejected commit", e)
	}
	appendID := create()
	handler.Store = conversationWriteFault{Store: store, appendFails: true}
	res = request("POST", "/api/conversations/"+strconv.FormatInt(appendID, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河片段保存失败"})
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	handler.Store = store
	_, appendMessages, e := store.Get(ctx, scopeA, appendID)
	if e != nil || appendMessages[1].State != "failed" || appendMessages[1].Content != "" || bytes.Contains(body, []byte("event: delta")) || bytes.Contains(body, []byte("event: done")) {
		t.Fatal("uncommitted prefix emitted", e)
	}
	finishID := create()
	handler.Store = conversationWriteFault{Store: store, completeFails: true}
	res = request("POST", "/api/conversations/"+strconv.FormatInt(finishID, 10)+"/messages", creds["student_a1"], map[string]any{"content": "星河终态保存失败"})
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	handler.Store = store
	_, finishMessages, e := store.Get(ctx, scopeA, finishID)
	if e != nil || finishMessages[1].State != "failed" || bytes.Contains(body, []byte("event: done")) {
		t.Fatal("failed completion published", e)
	}
	// Completion already committed but transport confirmation lost: failure or
	// startup recovery must not rewrite a completed answer as interrupted.
	validID := create()
	validConversation, _, e := store.Get(ctx, scopeA, validID)
	if e != nil {
		t.Fatal(e)
	}
	validTurn, e := store.Start(ctx, scopeA, validConversation, "星河完成后断连", p)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Append(ctx, scopeA, validID, validTurn, "完成回答 [1]"); e != nil {
		t.Fatal(e)
	}
	validSources, e := rag.ValidateSources("完成回答 [1]", p.Evidence)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Finish(ctx, scopeA, validID, validTurn, "completed", validSources, ""); e != nil {
		t.Fatal(e)
	}
	if e = store.Finish(ctx, scopeA, validID, validTurn, "interrupted", nil, "lost_done"); e == nil {
		t.Fatal("completed result regressed")
	}
	if e = store.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	_, confirmed, e := store.Get(ctx, scopeA, validID)
	if e != nil || confirmed[1].State != "completed" || len(confirmed[1].Citations) != 1 {
		t.Fatal("done-loss history confirmation", e)
	}
	// Current authenticated class controls every request, even with an old JWT.
	res = request("POST", "/api/conversations", creds[a2Name], map[string]any{"agent_id": agent.ID})
	if res.StatusCode != 201 {
		t.Fatal("A2 own conversation")
	}
	a2ID := int64(object(res)["id"].(float64))
	if _, e = db.Exec("UPDATE users SET class_id=2 WHERE username=?", a2Name); e != nil {
		t.Fatal(e)
	}
	res = request("GET", "/api/conversations/"+strconv.FormatInt(a2ID, 10), creds[a2Name], nil)
	if res.StatusCode != 404 {
		t.Fatal("old class conversation leaked")
	}
	res.Body.Close()
	t.Log("PASS real isolated MySQL: ownership/A-B, gated first delta, persisted prefixes, snapshots and Phase2.3, history, sources, no-evidence zero-call, failure, cancellation and concurrency")
}

type emptyConversationVectors struct{}

func (emptyConversationVectors) Search(context.Context, int64, []float64, []string, int) ([]rag.Hit, error) {
	return []rag.Hit{}, nil
}

type conversationWriteFault struct {
	conversation.Store
	appendFails, completeFails bool
}

func (f conversationWriteFault) Append(ctx context.Context, s rag.Scope, id int64, turn conversation.Turn, text string) error {
	if f.appendFails {
		return conversation.Unavailable
	}
	return f.Store.Append(ctx, s, id, turn, text)
}
func (f conversationWriteFault) Finish(ctx context.Context, s rag.Scope, id int64, turn conversation.Turn, state string, sources []rag.Source, reason string) error {
	if f.completeFails && state == "completed" {
		return conversation.Unavailable
	}
	return f.Store.Finish(ctx, s, id, turn, state, sources, reason)
}

type conversationRejectConnector struct{ driver.Connector }

func (c conversationRejectConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, e := c.Connector.Connect(ctx)
	if e != nil {
		return nil, e
	}
	return conversationRejectConnection{conn}, nil
}

type conversationRejectConnection struct{ driver.Conn }

func (c conversationRejectConnection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, e := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if e != nil {
		return nil, e
	}
	return conversationRejectTransaction{tx}, nil
}

type conversationRejectTransaction struct{ driver.Tx }

func (t conversationRejectTransaction) Commit() error {
	_ = t.Tx.Rollback()
	return fmt.Errorf("controlled transaction rejection")
}
