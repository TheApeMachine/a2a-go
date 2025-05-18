package memory

import (
	"context"

	"github.com/theapemachine/a2a-go/pkg/a2a"
)

// Embedder represents a service capable of generating embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// VectorStore provides semantic search capabilities over memories.
type VectorStore interface {
	StoreMemory(ctx context.Context, memory Memory) (string, error)
	StoreMemories(ctx context.Context, memories []Memory) error
	GetMemory(ctx context.Context, id string) (Memory, error)
	SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error)
	DeleteMemory(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}

// GraphStore manages relationships between memories.
type GraphStore interface {
	StoreMemory(ctx context.Context, memory Memory) (string, error)
	CreateRelation(ctx context.Context, relation Relation) error
	GetMemory(ctx context.Context, id string) (Memory, error)
	FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error)
	QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error)
	DeleteMemory(ctx context.Context, id string) error
	DeleteRelation(ctx context.Context, source, target, relationType string) error
	Ping(ctx context.Context) error
}

// UnifiedStore exposes a combined interface for vector and graph stores.
type UnifiedStore interface {
	StoreMemory(ctx context.Context, content string, metadata map[string]any, memType string) (string, error)
	CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error
	SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error)
	FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error)
	InjectMemories(ctx context.Context, task TaskLike) error
	ExtractMemories(ctx context.Context, task TaskLike) error
}

// TaskLike captures the subset of task operations needed by the memory system.
type TaskLike interface {
	AddMessage(role, name, text string)
	LastMessage() *a2a.Message
}
