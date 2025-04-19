// Package memory provides interfaces and implementations for a unified
// long-term memory system that combines vector and graph stores.
package memory

import (
	"context"
	"time"
)

// Memory represents a single item stored in memory across vector and graph stores
type Memory struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	Metadata    map[string]any         `json:"metadata,omitempty"`
	Embedding   []float32              `json:"embedding,omitempty"`
	Relations   []Relation             `json:"relations,omitempty"`
	Type        string                 `json:"type,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Collections []string               `json:"collections,omitempty"`
}

// Relation represents a relationship between two memories
type Relation struct {
	Source      string                 `json:"source"`
	Target      string                 `json:"target"`
	Type        string                 `json:"type"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// SearchParams contains options for memory search operations
type SearchParams struct {
	Query       string   `json:"query"`
	Collections []string `json:"collections,omitempty"`
	Limit       int      `json:"limit"`
	Types       []string `json:"types,omitempty"`
	Filters     []Filter `json:"filters,omitempty"`
}

// Filter represents a metadata filter for searching memories
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, ne, gt, lt, gte, lte, contains
	Value    any    `json:"value"`
}

// VectorStore defines operations for a vector database backend
type VectorStore interface {
	// Store a memory in the vector database
	StoreMemory(ctx context.Context, memory Memory) (string, error)
	
	// Store multiple memories in a batch
	StoreMemories(ctx context.Context, memories []Memory) error
	
	// Get a memory by ID
	GetMemory(ctx context.Context, id string) (Memory, error)
	
	// Search for semantically similar memories
	SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error)
	
	// Delete a memory
	DeleteMemory(ctx context.Context, id string) error
	
	// Check connection to the store
	Ping(ctx context.Context) error
}

// GraphStore defines operations for a graph database backend
type GraphStore interface {
	// Store a memory as a node in the graph
	StoreMemory(ctx context.Context, memory Memory) (string, error)
	
	// Create a relationship between two memories
	CreateRelation(ctx context.Context, relation Relation) error
	
	// Get a memory by ID
	GetMemory(ctx context.Context, id string) (Memory, error)
	
	// Find related memories
	FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error)
	
	// Perform a graph query to find connected memories
	QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error)
	
	// Delete a memory node
	DeleteMemory(ctx context.Context, id string) error
	
	// Delete a relation
	DeleteRelation(ctx context.Context, source, target, relationType string) error
	
	// Check connection to the store
	Ping(ctx context.Context) error
}

// EmbeddingService generates vector embeddings from text
type EmbeddingService interface {
	// Generate an embedding for a text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	
	// Generate embeddings for multiple texts in a batch
	GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

// UnifiedStore combines vector and graph stores into a single interface
type UnifiedStore interface {
	// Store a memory in both vector and graph stores as appropriate
	StoreMemory(ctx context.Context, content string, metadata map[string]any, storeType string) (string, error)
	
	// Retrieve a memory by ID from either store
	GetMemory(ctx context.Context, id string) (Memory, error)
	
	// Create a relationship between two memories
	CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error
	
	// Search for memories based on semantic similarity
	SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error)
	
	// Find memories related to a given memory
	FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error)
	
	// Delete a memory from all stores
	DeleteMemory(ctx context.Context, id string) error
}