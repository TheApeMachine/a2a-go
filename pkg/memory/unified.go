package memory

import (
	"context"
	"github.com/charmbracelet/log"
)

// UnifiedMemory implements the UnifiedStore interface.
type UnifiedMemory struct {
	embedder Embedder
	vector   VectorStore
	graph    GraphStore
}

func NewUnifiedStore(embedder Embedder, vector VectorStore, graph GraphStore) *UnifiedMemory {
	return &UnifiedMemory{embedder: embedder, vector: vector, graph: graph}
}

func (u *UnifiedMemory) StoreMemory(ctx context.Context, content string, metadata map[string]any, memType string) (string, error) {
	mem := Memory{Content: content, Metadata: metadata, Type: memType}
	id, err := u.vector.StoreMemory(ctx, mem)
	if err != nil {
		return "", err
	}
	mem.ID = id
	if u.graph != nil {
		if _, err := u.graph.StoreMemory(ctx, mem); err != nil {
			log.Error("graph store", "err", err)
		}
	}
	return id, nil
}

func (u *UnifiedMemory) CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error {
	if u.graph == nil {
		return nil
	}
	return u.graph.CreateRelation(ctx, Relation{SourceID: source, TargetID: target, Type: relationType, Properties: properties})
}

func (u *UnifiedMemory) SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error) {
	if u.vector == nil || u.embedder == nil {
		return nil, nil
	}
	emb, err := u.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return u.vector.SearchSimilar(ctx, emb, params)
}

func (u *UnifiedMemory) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	if u.graph == nil {
		return nil, nil
	}
	return u.graph.FindRelated(ctx, id, relationTypes, limit)
}

func (u *UnifiedMemory) InjectMemories(ctx context.Context, task TaskLike) error {
	last := task.LastMessage()
	if last == nil || u.vector == nil || u.embedder == nil {
		return nil
	}
	emb, err := u.embedder.Embed(ctx, last.String())
	if err != nil {
		return err
	}
	mems, err := u.vector.SearchSimilar(ctx, emb, SearchParams{Limit: 5})
	if err != nil {
		return err
	}
	for _, m := range mems {
		task.AddMessage("system", "memory", m.Content)
		rels, err := u.FindRelated(ctx, m.ID, nil, 5)
		if err == nil {
			for _, r := range rels {
				task.AddMessage("system", "relation", r.Content)
			}
		}
	}
	return nil
}

func (u *UnifiedMemory) ExtractMemories(ctx context.Context, task TaskLike) error {
	msg := task.LastMessage()
	if msg == nil {
		return nil
	}
	_, err := u.StoreMemory(ctx, msg.String(), map[string]any{"role": msg.Role}, "message")
	return err
}
