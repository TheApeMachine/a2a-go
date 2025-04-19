# Memory System Architecture

The A2A-Go memory system provides a unified interface for storing and retrieving memories using both vector and graph databases.

## Architecture Overview

```
┌─────────────────────────────────────┐
│       UnifiedStore Interface        │
│                                     │
│ StoreMemory    CreateRelation       │
│ GetMemory      FindRelated          │
│ SearchSimilar  DeleteMemory         │
└───────────────┬─────────────────────┘
                │
        ┌───────┴───────┐
        │               │
┌───────▼───────┐ ┌─────▼─────────┐
│  VectorStore  │ │  GraphStore   │
│  Interface    │ │  Interface    │
│               │ │               │
└───────┬───────┘ └─────┬─────────┘
        │               │
    ┌───┴───┐       ┌───┴───┐
┌───▼───┐ ┌─▼─────┐ ┌▼─────┐ ┌─▼────┐
│Qdrant │ │In-Mem │ │Neo4j │ │In-Mem│
│Store  │ │Vector │ │Store │ │Graph │
│       │ │Store  │ │      │ │Store │
└───────┘ └───────┘ └──────┘ └──────┘
```

## Key Components

### UnifiedStore

The `UnifiedStore` interface provides a unified API for interacting with both vector and graph stores:

```go
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
```

### VectorStore

The `VectorStore` interface handles semantic similarity search:

```go
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
```

### GraphStore

The `GraphStore` interface handles relationships between memories:

```go
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
```

### EmbeddingService

The `EmbeddingService` interface generates vector embeddings from text:

```go
type EmbeddingService interface {
    // Generate an embedding for a text
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    
    // Generate embeddings for multiple texts in a batch
    GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}
```

## Memory Implementations

The system provides both in-memory implementations for testing/development and real database implementations for production:

1. **Vector Stores**:
   - `QdrantVectorStore`: Uses Qdrant for production
   - `MockVectorStore`: In-memory implementation for testing

2. **Graph Stores**:
   - `Neo4jGraphStore`: Uses Neo4j for production
   - `MockGraphStore`: In-memory implementation for testing

3. **Embedding Services**:
   - `OpenAIEmbeddingService`: Uses OpenAI's API for production
   - `MockEmbeddingService`: Generates simple embeddings for testing

## Usage Example

```go
// Initialize components
embeddingService := memory.NewOpenAIEmbeddingService(openaiKey)
vectorStore := memory.NewQdrantVectorStore("http://localhost:6333", "memories", embeddingService)
graphStore := memory.NewNeo4jGraphStore("http://localhost:7474", "neo4j", "password")
unifiedStore := memory.NewUnifiedStore(embeddingService, vectorStore, graphStore)

// Store a memory
id, err := unifiedStore.StoreMemory(ctx, "Important information to remember", 
    map[string]any{"topic": "knowledge", "importance": 8}, "knowledge")

// Create a relationship
err = unifiedStore.CreateRelation(ctx, sourceID, targetID, "related_to", 
    map[string]any{"strength": 0.7})

// Semantic search
results, err := unifiedStore.SearchSimilar(ctx, "search query", memory.SearchParams{
    Limit: 10,
    Types: []string{"knowledge"},
})

// Find related memories
related, err := unifiedStore.FindRelated(ctx, id, []string{"related_to"}, 10)
```

## Built-in Memory Tools

A2A-Go provides built-in MCP tools for agents to interact with the memory system:

- `memory_unified_store`: Stores a memory in the unified memory system
- `memory_unified_retrieve`: Retrieves a memory by ID
- `memory_unified_search`: Searches for semantically similar memories
- `memory_unified_relate`: Creates a relationship between two memories
- `memory_unified_get_related`: Finds memories related to a given memory

These tools allow AI agents to maintain long-term memory across conversations.