package memory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/pkg/stores/qdrant"
)

// QdrantVectorStore implements VectorStore using Qdrant.
type QdrantVectorStore struct {
	client   *qdrant.Client
	embedder Embedder
}

func NewQdrantVectorStore(endpoint, collection string, embedder Embedder) *QdrantVectorStore {
	return &QdrantVectorStore{client: qdrant.New(endpoint, collection), embedder: embedder}
}

func (s *QdrantVectorStore) StoreMemory(ctx context.Context, mem Memory) (string, error) {
	if mem.ID == "" {
		mem.ID = uuid.NewString()
	}
	if mem.Embedding == nil && s.embedder != nil {
		emb, err := s.embedder.Embed(ctx, mem.Content)
		if err != nil {
			return "", err
		}
		mem.Embedding = emb
	}
	md := map[string]any{"embedding": mem.Embedding, "type": mem.Type}
	for k, v := range mem.Metadata {
		if k == "embedding" || k == "type" {
			continue
		}
		md[k] = v
	}
	doc := qdrant.NewDocument(mem.ID, mem.Content, md)
	if err := s.client.Put(ctx, []qdrant.Document{*doc}); err != nil {
		return "", err
	}
	return mem.ID, nil
}

func (s *QdrantVectorStore) StoreMemories(ctx context.Context, mems []Memory) error {
	for _, m := range mems {
		if _, err := s.StoreMemory(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (s *QdrantVectorStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	doc, err := s.client.Get(ctx, id)
	if err != nil {
		return Memory{}, err
	}
	return Memory{ID: doc.ID, Content: doc.Content, Metadata: doc.Metadata}, nil
}

func (s *QdrantVectorStore) SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error) {
	docs, err := s.client.Search(ctx, embedding, params.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(docs))
	for _, d := range docs {
		out = append(out, Memory{ID: d.ID, Content: d.Content, Metadata: d.Metadata})
	}
	return out, nil
}

func (s *QdrantVectorStore) DeleteMemory(ctx context.Context, id string) error {
	return s.client.Delete(ctx, id)
}

func (s *QdrantVectorStore) Ping(ctx context.Context) error {
	_, err := s.client.Search(ctx, []float32{0}, 1)
	if err != nil {
		return fmt.Errorf("qdrant ping failed: %w", err)
	}
	return nil
}
