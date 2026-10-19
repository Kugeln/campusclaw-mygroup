package mysql

import (
	"campusclaw/internal/rag"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type RAG struct{ DB *sql.DB }

const evidenceSQL = `SELECT c.id,c.material_id,c.knowledge_entry_id,m.original_name,c.chunk_index,c.chunk_text FROM knowledge_chunks c JOIN knowledge_entries k ON k.id=c.knowledge_entry_id AND k.material_id=c.material_id AND k.class_id=c.class_id JOIN materials m ON m.id=c.material_id AND m.class_id=c.class_id WHERE c.class_id=? AND c.index_status='ready' AND k.index_status='ready'`

func restriction(column string, values []string) (string, []any) {
	if len(values) == 0 {
		return "", nil
	}
	args := []any{}
	for _, v := range values {
		args = append(args, v)
	}
	return " AND " + column + " IN (" + strings.TrimRight(strings.Repeat("?,", len(values)), ",") + ")", args
}
func (r RAG) Keyword(ctx context.Context, class int64, q string, materials []string, limit int) ([]rag.Evidence, error) {
	match := "MATCH(c.chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE)"
	query := strings.Replace(evidenceSQL, " FROM knowledge_chunks", ", "+match+" FROM knowledge_chunks", 1)
	filter, args := restriction("c.material_id", materials)
	a := []any{q, class}
	a = append(a, args...)
	a = append(a, q, limit)
	rows, e := r.DB.QueryContext(ctx, query+filter+" AND "+match+">0 ORDER BY 7 DESC,c.id ASC LIMIT ?", a...)
	if e != nil {
		return nil, rag.Unavailable
	}
	defer rows.Close()
	out := []rag.Evidence{}
	for rows.Next() {
		var v rag.Evidence
		if e = rows.Scan(&v.ChunkID, &v.MaterialID, &v.EntryID, &v.Title, &v.Index, &v.Text, &v.Score); e != nil {
			return nil, rag.Unavailable
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		return nil, rag.Unavailable
	}
	return out, nil
}
func (r RAG) Hydrate(ctx context.Context, class int64, ids, materials []string) (map[string]rag.Evidence, error) {
	out := map[string]rag.Evidence{}
	if len(ids) == 0 {
		return out, nil
	}
	filter, args := restriction("c.id", ids)
	mf, ma := restriction("c.material_id", materials)
	a := []any{class}
	a = append(a, args...)
	a = append(a, ma...)
	rows, e := r.DB.QueryContext(ctx, evidenceSQL+filter+mf, a...)
	if e != nil {
		return nil, rag.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var v rag.Evidence
		if rows.Scan(&v.ChunkID, &v.MaterialID, &v.EntryID, &v.Title, &v.Index, &v.Text) != nil {
			return nil, rag.Unavailable
		}
		out[v.ChunkID] = v
	}
	if rows.Err() != nil {
		return nil, rag.Unavailable
	}
	return out, nil
}
func (r RAG) Find(ctx context.Context, class int64, id string) (rag.Material, error) {
	var m rag.Material
	e := r.DB.QueryRowContext(ctx, `SELECT m.id,m.class_id,k.id,k.plain_text FROM materials m JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id WHERE m.id=? AND m.class_id=?`, id, class).Scan(&m.ID, &m.ClassID, &m.EntryID, &m.Text)
	if e == sql.ErrNoRows {
		return m, rag.NotFound
	}
	if e != nil {
		return m, rag.Unavailable
	}
	return m, nil
}
func (r RAG) State(ctx context.Context, class int64, id, state string, cleanup bool) error {
	if state != "indexing" && state != "ready" && state != "failed" {
		return rag.Invalid
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return rag.Unavailable
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, "UPDATE knowledge_entries SET index_status=?,cleanup_required=? WHERE material_id=? AND class_id=?", state, cleanup, id, class)
	if e != nil {
		return rag.Unavailable
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var one int
		if tx.QueryRowContext(ctx, "SELECT 1 FROM knowledge_entries WHERE material_id=? AND class_id=?", id, class).Scan(&one) != nil {
			return rag.NotFound
		}
	}
	chunkState := state
	if state == "indexing" {
		chunkState = "pending"
	}
	if _, e = tx.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status=? WHERE material_id=? AND class_id=?", chunkState, id, class); e != nil {
		return rag.Unavailable
	}
	if tx.Commit() != nil {
		return rag.Unavailable
	}
	return nil
}
func (r RAG) Replace(ctx context.Context, m rag.Material, texts []string, strategy rag.Strategy) ([]rag.Evidence, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, rag.Unavailable
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "DELETE FROM knowledge_chunks WHERE material_id=? AND class_id=?", m.ID, m.ClassID); e != nil {
		return nil, rag.Unavailable
	}
	b, _ := json.Marshal(strategy)
	if _, e = tx.ExecContext(ctx, "UPDATE knowledge_entries SET chunk_strategy=? WHERE material_id=? AND class_id=?", b, m.ID, m.ClassID); e != nil {
		return nil, rag.Unavailable
	}
	pieces := []rag.Evidence{}
	for i, text := range texts {
		var random [16]byte
		if _, e = rand.Read(random[:]); e != nil {
			return nil, rag.Unavailable
		}
		random[6] = (random[6] & 15) | 64
		random[8] = (random[8] & 63) | 128
		id := fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:])
		v := rag.Evidence{ChunkID: id, MaterialID: m.ID, EntryID: m.EntryID, Index: i + 1, Text: text}
		if _, e = tx.ExecContext(ctx, "INSERT INTO knowledge_chunks(id,knowledge_entry_id,material_id,class_id,chunk_index,chunk_text) VALUES (?,?,?,?,?,?)", v.ChunkID, v.EntryID, v.MaterialID, m.ClassID, v.Index, v.Text); e != nil {
			return nil, rag.Unavailable
		}
		pieces = append(pieces, v)
	}
	if tx.Commit() != nil {
		return nil, rag.Unavailable
	}
	return pieces, nil
}
func (r RAG) Interrupted(ctx context.Context) ([]rag.Material, error) {
	rows, e := r.DB.QueryContext(ctx, "SELECT material_id,class_id FROM knowledge_entries WHERE index_status IN ('pending','indexing') OR cleanup_required=1")
	if e != nil {
		return nil, rag.Unavailable
	}
	defer rows.Close()
	out := []rag.Material{}
	for rows.Next() {
		var m rag.Material
		if rows.Scan(&m.ID, &m.ClassID) != nil {
			return nil, rag.Unavailable
		}
		out = append(out, m)
	}
	if rows.Err() != nil {
		return nil, rag.Unavailable
	}
	return out, nil
}
