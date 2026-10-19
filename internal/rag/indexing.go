package rag

import (
	"context"
	"sync"
	"time"
)

type Material struct {
	ID               string
	ClassID, EntryID int64
	Text             string
}
type IndexStore interface {
	Find(context.Context, int64, string) (Material, error)
	State(context.Context, int64, string, string, bool) error
	Replace(context.Context, Material, []string, Strategy) ([]Evidence, error)
	Interrupted(context.Context) ([]Material, error)
}
type IndexVectors interface {
	Check(context.Context) (bool, error)
	Ensure(context.Context) error
	Delete(context.Context, int64, string) error
	Upsert(context.Context, int64, []Evidence, [][]float64) error
}
type Indexer struct {
	Store     IndexStore
	Embedding Embedder
	Vectors   IndexVectors
	mu        sync.Mutex
	active    map[string]bool
}

func (i *Indexer) Run(ctx context.Context, s Scope, id string, options map[string]any, fresh bool) (string, error) {
	if s.Role != "teacher" || s.ClassID <= 0 {
		return "", Invalid
	}
	strategy, e := ParseStrategy(options)
	if e != nil {
		return "", e
	}
	i.mu.Lock()
	if i.active == nil {
		i.active = map[string]bool{}
	}
	if i.active[id] {
		i.mu.Unlock()
		return "", Busy
	}
	i.active[id] = true
	i.mu.Unlock()
	defer func() { i.mu.Lock(); delete(i.active, id); i.mu.Unlock() }()
	m, e := i.Store.Find(ctx, s.ClassID, id)
	if e != nil {
		return "", e
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if !fresh {
		if _, e = i.Vectors.Check(ctx); e != nil {
			return "", e
		}
	}
	cleanup, wrote := !fresh, false
	work := func() error {
		if e := i.Store.State(ctx, s.ClassID, id, "indexing", cleanup); e != nil {
			return e
		}
		if !fresh {
			if e := i.Vectors.Ensure(ctx); e != nil {
				return e
			}
			if e := i.Vectors.Delete(ctx, s.ClassID, id); e != nil {
				return e
			}
			cleanup = false
		}
		pieces, e := i.Store.Replace(ctx, m, Chunks(m.Text, strategy), strategy)
		if e != nil {
			return e
		}
		vectors := [][]float64{}
		for _, p := range pieces {
			v, e := i.Embedding.Embed(ctx, p.Text)
			if e != nil {
				return e
			}
			vectors = append(vectors, v)
		}
		if len(pieces) > 0 {
			if e = i.Vectors.Ensure(ctx); e != nil {
				return e
			}
			wrote = true
			if e = i.Vectors.Upsert(ctx, s.ClassID, pieces, vectors); e != nil {
				return e
			}
		}
		return i.Store.State(ctx, s.ClassID, id, "ready", false)
	}
	if e = work(); e != nil {
		recovery, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if wrote {
			cleanup = i.Vectors.Delete(recovery, s.ClassID, id) != nil
		}
		_ = i.Store.State(recovery, s.ClassID, id, "failed", cleanup)
		return "failed", Unavailable
	}
	return "ready", nil
}
func (i *Indexer) Recover(ctx context.Context) error {
	items, e := i.Store.Interrupted(ctx)
	if e != nil {
		return e
	}
	for _, m := range items {
		if _, e = i.Vectors.Check(ctx); e != nil {
			continue
		}
		if e = i.Store.State(ctx, m.ClassID, m.ID, "failed", true); e != nil {
			return e
		}
		if i.Vectors.Ensure(ctx) != nil || i.Vectors.Delete(ctx, m.ClassID, m.ID) != nil {
			continue
		}
		if e = i.Store.State(ctx, m.ClassID, m.ID, "failed", false); e != nil {
			return e
		}
	}
	return nil
}
