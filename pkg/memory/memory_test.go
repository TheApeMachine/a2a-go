package memory

import (
	"context"
	"reflect"
	"testing"
)

func TestMockEmbeddingService(t *testing.T) {
	service := NewMockEmbeddingService()
	ctx := context.Background()
	
	t.Run("GenerateEmbedding", func(t *testing.T) {
		text := "Hello world"
		embedding, err := service.GenerateEmbedding(ctx, text)
		
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		
		if len(embedding) != 4 {
			t.Fatalf("Expected embedding dimension of 4, got: %d", len(embedding))
		}
		
		// Same text should generate same embedding (deterministic)
		embedding2, _ := service.GenerateEmbedding(ctx, text)
		if !reflect.DeepEqual(embedding, embedding2) {
			t.Fatalf("Expected consistent embeddings for same text")
		}
		
		// Different text should generate different embedding
		differentEmbedding, _ := service.GenerateEmbedding(ctx, "Different text")
		if reflect.DeepEqual(embedding, differentEmbedding) {
			t.Fatalf("Expected different embeddings for different text")
		}
	})
	
	t.Run("GenerateEmbeddings", func(t *testing.T) {
		texts := []string{"Hello", "World"}
		embeddings, err := service.GenerateEmbeddings(ctx, texts)
		
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		
		if len(embeddings) != len(texts) {
			t.Fatalf("Expected %d embeddings, got: %d", len(texts), len(embeddings))
		}
		
		// Check each embedding
		for i, emb := range embeddings {
			if len(emb) != 4 {
				t.Fatalf("Expected embedding dimension of 4, got: %d", len(emb))
			}
			
			// Verify single embedding matches batch embedding
			singleEmb, _ := service.GenerateEmbedding(ctx, texts[i])
			if !reflect.DeepEqual(emb, singleEmb) {
				t.Fatalf("Batch embedding doesn't match single embedding for text: %s", texts[i])
			}
		}
	})
}

func TestInMemoryVectorStore(t *testing.T) {
	store := NewInMemoryVectorStore()
	ctx := context.Background()
	
	t.Run("StoreAndGetMemory", func(t *testing.T) {
		memory := Memory{
			Content:   "Test content",
			Metadata:  map[string]any{"key": "value"},
			Type:      "knowledge",
			Embedding: []float32{0.1, 0.2, 0.3, 0.4},
		}
		
		id, err := store.StoreMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		if id == "" {
			t.Fatalf("Expected non-empty ID")
		}
		
		// Retrieve the memory
		retrieved, err := store.GetMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to retrieve memory: %v", err)
		}
		
		if retrieved.Content != memory.Content {
			t.Fatalf("Content mismatch, got: %s, want: %s", retrieved.Content, memory.Content)
		}
		
		if retrieved.Type != memory.Type {
			t.Fatalf("Type mismatch, got: %s, want: %s", retrieved.Type, memory.Type)
		}
		
		if !reflect.DeepEqual(retrieved.Embedding, memory.Embedding) {
			t.Fatalf("Embedding mismatch")
		}
		
		// Check timestamps
		if retrieved.CreatedAt.IsZero() {
			t.Fatalf("CreatedAt should be set")
		}
		
		if retrieved.UpdatedAt.IsZero() {
			t.Fatalf("UpdatedAt should be set")
		}
	})
	
	t.Run("StoreMemories", func(t *testing.T) {
		memories := []Memory{
			{
				Content:   "Content 1",
				Type:      "knowledge",
				Embedding: []float32{0.1, 0.2, 0.3, 0.4},
			},
			{
				Content:   "Content 2",
				Type:      "experience",
				Embedding: []float32{0.5, 0.6, 0.7, 0.8},
			},
		}
		
		err := store.StoreMemories(ctx, memories)
		if err != nil {
			t.Fatalf("Failed to store memories: %v", err)
		}
	})
	
	t.Run("SearchSimilar", func(t *testing.T) {
		// Store test memories
		testMemories := []Memory{
			{
				Content:   "Apple is a fruit",
				Type:      "concept",
				Metadata:  map[string]any{"topic": "fruit"},
				Embedding: []float32{0.1, 0.2, 0.3, 0.4},
			},
			{
				Content:   "Banana is yellow",
				Type:      "concept",
				Metadata:  map[string]any{"topic": "fruit"},
				Embedding: []float32{0.2, 0.3, 0.4, 0.5},
			},
			{
				Content:   "Cars are vehicles",
				Type:      "concept",
				Metadata:  map[string]any{"topic": "transport"},
				Embedding: []float32{0.5, 0.6, 0.7, 0.8},
			},
		}
		
		for _, mem := range testMemories {
			_, err := store.StoreMemory(ctx, mem)
			if err != nil {
				t.Fatalf("Failed to store test memory: %v", err)
			}
		}
		
		// Search by content (naive implementation just does substring match)
		results, err := store.SearchSimilar(ctx, []float32{0, 0, 0, 0}, SearchParams{
			Query: "fruit",
			Limit: 5,
		})
		
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		if len(results) < 1 {
			t.Fatalf("Expected at least 1 result, got: %d", len(results))
		}
		
		// Search with type filter
		results, err = store.SearchSimilar(ctx, []float32{0, 0, 0, 0}, SearchParams{
			Query: "fruit",
			Types: []string{"concept"},
			Limit: 5,
		})
		
		if err != nil {
			t.Fatalf("Search with type filter failed: %v", err)
		}
		
		// Search with metadata filter
		results, err = store.SearchSimilar(ctx, []float32{0, 0, 0, 0}, SearchParams{
			Query: "fruit",
			Filters: []Filter{
				{Field: "topic", Operator: "eq", Value: "fruit"},
			},
			Limit: 5,
		})
		
		if err != nil {
			t.Fatalf("Search with metadata filter failed: %v", err)
		}
		
		if len(results) < 1 {
			t.Fatalf("Expected at least 1 result with topic=fruit, got: %d", len(results))
		}
	})
	
	t.Run("DeleteMemory", func(t *testing.T) {
		memory := Memory{
			Content:   "Delete me",
			Embedding: []float32{0.1, 0.2, 0.3, 0.4},
		}
		
		id, err := store.StoreMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		// Delete the memory
		err = store.DeleteMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to delete memory: %v", err)
		}
		
		// Try to retrieve the deleted memory
		_, err = store.GetMemory(ctx, id)
		if err == nil {
			t.Fatalf("Expected error when retrieving deleted memory")
		}
	})
	
	t.Run("Ping", func(t *testing.T) {
		err := store.Ping(ctx)
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}

func TestInMemoryGraphStore(t *testing.T) {
	store := NewInMemoryGraphStore()
	ctx := context.Background()
	
	t.Run("StoreAndGetMemory", func(t *testing.T) {
		memory := Memory{
			Content:  "Graph node content",
			Metadata: map[string]any{"key": "value"},
			Type:     "concept",
		}
		
		id, err := store.StoreMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		if id == "" {
			t.Fatalf("Expected non-empty ID")
		}
		
		// Retrieve the memory
		retrieved, err := store.GetMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to retrieve memory: %v", err)
		}
		
		if retrieved.Content != memory.Content {
			t.Fatalf("Content mismatch, got: %s, want: %s", retrieved.Content, memory.Content)
		}
		
		if retrieved.Type != memory.Type {
			t.Fatalf("Type mismatch, got: %s, want: %s", retrieved.Type, memory.Type)
		}
		
		// Check timestamps
		if retrieved.CreatedAt.IsZero() {
			t.Fatalf("CreatedAt should be set")
		}
		
		if retrieved.UpdatedAt.IsZero() {
			t.Fatalf("UpdatedAt should be set")
		}
	})
	
	t.Run("CreateAndQueryRelations", func(t *testing.T) {
		// Create two nodes
		memory1 := Memory{
			Content: "Parent concept",
			Type:    "concept",
		}
		
		memory2 := Memory{
			Content: "Child concept",
			Type:    "concept",
		}
		
		id1, err := store.StoreMemory(ctx, memory1)
		if err != nil {
			t.Fatalf("Failed to store memory1: %v", err)
		}
		
		id2, err := store.StoreMemory(ctx, memory2)
		if err != nil {
			t.Fatalf("Failed to store memory2: %v", err)
		}
		
		// Create relation
		relation := Relation{
			Source:     id1,
			Target:     id2,
			Type:       "includes",
			Properties: map[string]interface{}{"strength": 0.8},
		}
		
		err = store.CreateRelation(ctx, relation)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}
		
		// Find related memories
		related, err := store.FindRelated(ctx, id1, []string{"includes"}, 10)
		if err != nil {
			t.Fatalf("Failed to find related memories: %v", err)
		}
		
		if len(related) != 1 {
			t.Fatalf("Expected 1 related memory, got: %d", len(related))
		}
		
		if related[0].ID != id2 {
			t.Fatalf("Related memory ID mismatch, got: %s, want: %s", related[0].ID, id2)
		}
	})
	
	t.Run("QueryGraph", func(t *testing.T) {
		// Store test memories with content to search
		memory := Memory{
			Content: "Searchable graph content",
			Type:    "concept",
		}
		
		_, err := store.StoreMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		// Search using the query (naive implementation just does substring match)
		results, err := store.QueryGraph(ctx, "Searchable", nil)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		
		if len(results) < 1 {
			t.Fatalf("Expected at least 1 result, got: %d", len(results))
		}
	})
	
	t.Run("DeleteMemory", func(t *testing.T) {
		memory := Memory{
			Content: "Delete this node",
			Type:    "concept",
		}
		
		id, err := store.StoreMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		// Create another node and relation
		memory2 := Memory{
			Content: "Related node",
			Type:    "concept",
		}
		
		id2, err := store.StoreMemory(ctx, memory2)
		if err != nil {
			t.Fatalf("Failed to store memory2: %v", err)
		}
		
		// Create relation
		relation := Relation{
			Source: id,
			Target: id2,
			Type:   "related_to",
		}
		
		err = store.CreateRelation(ctx, relation)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}
		
		// Delete the memory
		err = store.DeleteMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to delete memory: %v", err)
		}
		
		// Try to retrieve the deleted memory
		_, err = store.GetMemory(ctx, id)
		if err == nil {
			t.Fatalf("Expected error when retrieving deleted memory")
		}
		
		// Relation should be deleted too - try to find related
		related, err := store.FindRelated(ctx, id, nil, 10)
		if err != nil {
			// Error is expected here in some implementations
			t.Logf("FindRelated returned error as expected: %v", err)
		}
		
		if len(related) > 0 {
			t.Fatalf("Expected 0 related memories after deletion, got: %d", len(related))
		}
	})
	
	t.Run("DeleteRelation", func(t *testing.T) {
		// Create two nodes
		memory1 := Memory{
			Content: "Source for relation delete",
			Type:    "concept",
		}
		
		memory2 := Memory{
			Content: "Target for relation delete",
			Type:    "concept",
		}
		
		id1, err := store.StoreMemory(ctx, memory1)
		if err != nil {
			t.Fatalf("Failed to store memory1: %v", err)
		}
		
		id2, err := store.StoreMemory(ctx, memory2)
		if err != nil {
			t.Fatalf("Failed to store memory2: %v", err)
		}
		
		// Create relation
		relation := Relation{
			Source: id1,
			Target: id2,
			Type:   "test_relation",
		}
		
		err = store.CreateRelation(ctx, relation)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}
		
		// Delete the relation
		err = store.DeleteRelation(ctx, id1, id2, "test_relation")
		if err != nil {
			t.Fatalf("Failed to delete relation: %v", err)
		}
		
		// Verify relation is deleted
		related, err := store.FindRelated(ctx, id1, []string{"test_relation"}, 10)
		if err != nil {
			t.Logf("FindRelated returned error: %v", err)
		}
		
		if len(related) > 0 {
			t.Fatalf("Expected 0 related memories after relation deletion, got: %d", len(related))
		}
	})
	
	t.Run("Ping", func(t *testing.T) {
		err := store.Ping(ctx)
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})
}

func TestUnifiedStore(t *testing.T) {
	// Create mock components
	embeddingService := NewMockEmbeddingService()
	vectorStore := NewInMemoryVectorStore()
	graphStore := NewInMemoryGraphStore()
	
	// Create unified store
	store := NewUnifiedStore(embeddingService, vectorStore, graphStore)
	
	ctx := context.Background()
	
	t.Run("StoreMemory", func(t *testing.T) {
		content := "Test unified memory"
		metadata := map[string]any{"key": "value"}
		memoryType := "knowledge"
		
		id, err := store.StoreMemory(ctx, content, metadata, memoryType)
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		if id == "" {
			t.Fatalf("Expected non-empty ID")
		}
		
		// Retrieve the memory
		memory, err := store.GetMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to retrieve memory: %v", err)
		}
		
		if memory.Content != content {
			t.Fatalf("Content mismatch, got: %s, want: %s", memory.Content, content)
		}
		
		if memory.Type != memoryType {
			t.Fatalf("Type mismatch, got: %s, want: %s", memory.Type, memoryType)
		}
		
		// Check that embedding was generated
		if memory.Embedding == nil || len(memory.Embedding) == 0 {
			t.Fatalf("Embedding was not generated")
		}
	})
	
	t.Run("CreateAndQueryRelation", func(t *testing.T) {
		// Create two memories
		content1 := "Parent memory for relation"
		content2 := "Child memory for relation"
		
		id1, err := store.StoreMemory(ctx, content1, nil, "concept")
		if err != nil {
			t.Fatalf("Failed to store memory1: %v", err)
		}
		
		id2, err := store.StoreMemory(ctx, content2, nil, "concept")
		if err != nil {
			t.Fatalf("Failed to store memory2: %v", err)
		}
		
		// Create relation
		err = store.CreateRelation(ctx, id1, id2, "includes", map[string]any{"strength": 0.9})
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}
		
		// Find related memories
		related, err := store.FindRelated(ctx, id1, []string{"includes"}, 10)
		if err != nil {
			t.Fatalf("Failed to find related memories: %v", err)
		}
		
		if len(related) != 1 {
			t.Fatalf("Expected 1 related memory, got: %d", len(related))
		}
		
		if related[0].ID != id2 {
			t.Fatalf("Related memory ID mismatch, got: %s, want: %s", related[0].ID, id2)
		}
	})
	
	t.Run("SearchSimilar", func(t *testing.T) {
		// Store test memories with distinct content
		_, err := store.StoreMemory(ctx, "Elephants are mammals", 
			map[string]any{"topic": "animals"}, "knowledge")
		if err != nil {
			t.Fatalf("Failed to store test memory: %v", err)
		}
		
		_, err = store.StoreMemory(ctx, "Tigers are big cats", 
			map[string]any{"topic": "animals"}, "knowledge")
		if err != nil {
			t.Fatalf("Failed to store test memory: %v", err)
		}
		
		// Search by text (in-memory implementation uses simple substring matching)
		_, err = store.SearchSimilar(ctx, "animals", SearchParams{
			Query: "animals",
			Limit: 5,
		})
		
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}
		
		// Check metadata filters
		_, err = store.SearchSimilar(ctx, "mammals", SearchParams{
			Query: "mammals",
			Filters: []Filter{
				{Field: "topic", Operator: "eq", Value: "animals"},
			},
			Limit: 5,
		})
		
		if err != nil {
			t.Fatalf("Search with filters failed: %v", err)
		}
	})
	
	t.Run("DeleteMemory", func(t *testing.T) {
		content := "Memory to delete"
		
		id, err := store.StoreMemory(ctx, content, nil, "knowledge")
		if err != nil {
			t.Fatalf("Failed to store memory: %v", err)
		}
		
		// Delete the memory
		err = store.DeleteMemory(ctx, id)
		if err != nil {
			t.Fatalf("Failed to delete memory: %v", err)
		}
		
		// Try to retrieve the deleted memory
		_, err = store.GetMemory(ctx, id)
		if err == nil {
			t.Fatalf("Expected error when retrieving deleted memory")
		}
	})
}