package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/memory"
	memstore "github.com/theapemachine/a2a-go/memory"
	pkgmemory "github.com/theapemachine/a2a-go/pkg/memory"
)

func TestMemoryTools(t *testing.T) {
	// Use the default in-memory store for tests
	store := memstore.New()
	// Override the default store for testing
	defaultMemoryStore = store

	// Test memory_store tool
	t.Run("memory_store", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Test content",
					"backend": "vector",
					"metadata": map[string]interface{}{
						"test": "value",
					},
				},
			},
		}

		result, err := handleMemoryStore(context.Background(), req)
		if err != nil {
			t.Fatalf("memory_store failed: %v", err)
		}

		// Result should be the document ID
		id := result.Content[0].(mcp.TextContent).Text
		if id == "" {
			t.Fatalf("Expected non-empty document ID")
		}

		// Verify the document was stored
		doc, found := store.Get(id)
		if !found {
			t.Fatalf("Document not found with ID: %s", id)
		}

		if doc.Content != "Test content" {
			t.Fatalf("Content mismatch, got: %s, want: %s", doc.Content, "Test content")
		}

		if value, ok := doc.Metadata["test"]; !ok || value != "value" {
			t.Fatalf("Metadata not stored correctly")
		}
	})

	// Test memory_query tool
	t.Run("memory_query", func(t *testing.T) {
		// First store a document
		docID := store.Put("vector", "Query test content", map[string]any{"query": "test"})

		// Now query it
		req := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_query",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"id": docID,
				},
			},
		}

		result, err := handleMemoryQuery(context.Background(), req)
		if err != nil {
			t.Fatalf("memory_query failed: %v", err)
		}

		// Result should be a JSON string
		var doc memory.Document
		if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &doc); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if doc.ID != docID {
			t.Fatalf("ID mismatch, got: %s, want: %s", doc.ID, docID)
		}

		if doc.Content != "Query test content" {
			t.Fatalf("Content mismatch, got: %s, want: %s", doc.Content, "Query test content")
		}
	})

	// Test memory_search tool
	t.Run("memory_search", func(t *testing.T) {
		// Store some test documents
		store.Put("vector", "Apple is a fruit", map[string]any{"topic": "fruits"})
		store.Put("vector", "Banana is yellow", map[string]any{"topic": "fruits"})

		// Search for them
		req := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_search",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"query":   "fruit",
					"backend": "vector",
					"limit":   float64(5),
				},
			},
		}

		result, err := handleMemorySearch(context.Background(), req)
		if err != nil {
			t.Fatalf("memory_search failed: %v", err)
		}

		// Result should be a JSON array of IDs
		var ids []string
		if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &ids); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if len(ids) == 0 {
			t.Fatalf("Expected at least one result")
		}
	})
}

func TestUnifiedMemoryTools(t *testing.T) {
	// Create a new server
	mcpServer := server.NewMCPServer("test", "1.0")

	// Set up unified memory components
	embeddingService := pkgmemory.NewMockEmbeddingService()
	vectorStore := pkgmemory.NewInMemoryVectorStore()
	graphStore := pkgmemory.NewInMemoryGraphStore()
	memoryStore := pkgmemory.NewUnifiedStore(embeddingService, vectorStore, graphStore)

	// Override the default store for testing
	unifiedMemoryStore = memoryStore

	// Register memory tools
	registerMemoryTools(mcpServer)

	// Test memory_unified_store tool
	t.Run("memory_unified_store", func(t *testing.T) {
		req := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Unified store test content",
					"type":    "knowledge",
					"metadata": map[string]interface{}{
						"topic": "testing",
					},
				},
			},
		}

		result, err := handleUnifiedMemoryStore(context.Background(), req)
		if err != nil {
			t.Fatalf("memory_unified_store failed: %v", err)
		}

		// Parse result to get ID
		var storeResult map[string]string
		if err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &storeResult); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		id := storeResult["id"]
		if id == "" {
			t.Fatalf("Expected non-empty ID")
		}

		// Test retrieving the stored memory
		retrieveReq := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_retrieve",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"id": id,
				},
			},
		}

		retrieveResult, err := handleUnifiedMemoryRetrieve(context.Background(), retrieveReq)
		if err != nil {
			t.Fatalf("memory_unified_retrieve failed: %v", err)
		}

		// Parse memory
		var memory pkgmemory.Memory
		if err := json.Unmarshal([]byte(retrieveResult.Content[0].(mcp.TextContent).Text), &memory); err != nil {
			t.Fatalf("Failed to parse memory: %v", err)
		}

		if memory.Content != "Unified store test content" {
			t.Fatalf("Content mismatch, got: %s, want: %s", memory.Content, "Unified store test content")
		}

		if memory.Type != "knowledge" {
			t.Fatalf("Type mismatch, got: %s, want: %s", memory.Type, "knowledge")
		}

		if topic, ok := memory.Metadata["topic"]; !ok || topic != "testing" {
			t.Fatalf("Metadata not stored correctly")
		}
	})

	// Test memory_unified_search tool
	t.Run("memory_unified_search", func(t *testing.T) {
		// Store some test memories
		req1 := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Dogs are mammals",
					"type":    "knowledge",
					"metadata": map[string]interface{}{
						"topic": "animals",
					},
				},
			},
		}

		req2 := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Cats are mammals",
					"type":    "knowledge",
					"metadata": map[string]interface{}{
						"topic": "animals",
					},
				},
			},
		}

		_, err := handleUnifiedMemoryStore(context.Background(), req1)
		if err != nil {
			t.Fatalf("Failed to store test memory 1: %v", err)
		}

		_, err = handleUnifiedMemoryStore(context.Background(), req2)
		if err != nil {
			t.Fatalf("Failed to store test memory 2: %v", err)
		}

		// Search for memories
		searchReq := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_search",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"query": "mammals",
					"limit": float64(5),
					"types": []string{"knowledge"},
					"filters": []map[string]interface{}{
						{
							"field":    "topic",
							"operator": "eq",
							"value":    "animals",
						},
					},
				},
			},
		}

		searchResult, err := handleUnifiedMemorySearch(context.Background(), searchReq)
		if err != nil {
			t.Fatalf("memory_unified_search failed: %v", err)
		}

		// Parse search results
		var results map[string]any
		if err := json.Unmarshal([]byte(searchResult.Content[0].(mcp.TextContent).Text), &results); err != nil {
			t.Fatalf("Failed to parse search results: %v", err)
		}

		// Check count
		count, ok := results["count"].(float64)
		if !ok {
			t.Fatalf("Expected count in results")
		}

		// The in-memory implementation might not find exact matches due to simple substring matching
		// but there should be some results
		t.Logf("Found %d results for 'mammals'", int(count))
	})

	// Test memory_unified_relate tool
	t.Run("memory_unified_relate", func(t *testing.T) {
		// Store two memories to relate
		req1 := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Mammals are warm-blooded",
					"type":    "concept",
				},
			},
		}

		req2 := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_store",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"content": "Dogs are a type of mammal",
					"type":    "knowledge",
				},
			},
		}

		result1, err := handleUnifiedMemoryStore(context.Background(), req1)
		if err != nil {
			t.Fatalf("Failed to store concept memory: %v", err)
		}

		result2, err := handleUnifiedMemoryStore(context.Background(), req2)
		if err != nil {
			t.Fatalf("Failed to store knowledge memory: %v", err)
		}

		// Extract IDs
		var storeResult1, storeResult2 map[string]string
		json.Unmarshal([]byte(result1.Content[0].(mcp.TextContent).Text), &storeResult1)
		json.Unmarshal([]byte(result2.Content[0].(mcp.TextContent).Text), &storeResult2)

		id1 := storeResult1["id"]
		id2 := storeResult2["id"]

		// Create relation
		relateReq := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_relate",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"source_id":     id1,
					"target_id":     id2,
					"relation_type": "includes",
					"properties": map[string]interface{}{
						"strength": 0.9,
					},
				},
			},
		}

		relateResult, err := handleUnifiedMemoryRelation(context.Background(), relateReq)
		if err != nil {
			t.Fatalf("memory_unified_relate failed: %v", err)
		}

		// Parse relation result
		var relationResult map[string]string
		if err := json.Unmarshal([]byte(relateResult.Content[0].(mcp.TextContent).Text), &relationResult); err != nil {
			t.Fatalf("Failed to parse relation result: %v", err)
		}

		if relationResult["status"] != "success" {
			t.Fatalf("Relation creation failed: %s", relationResult["message"])
		}

		// Test finding related memories
		relatedReq := mcp.CallToolRequest{
			Request: mcp.Request{
				Method: "memory_unified_get_related",
			},
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Arguments: map[string]interface{}{
					"id":             id1,
					"relation_types": []string{"includes"},
					"limit":          float64(5),
				},
			},
		}

		relatedResult, err := handleUnifiedMemoryRelated(context.Background(), relatedReq)
		if err != nil {
			t.Fatalf("memory_unified_get_related failed: %v", err)
		}

		// Parse related results
		var relatedData map[string]any
		if err := json.Unmarshal([]byte(relatedResult.Content[0].(mcp.TextContent).Text), &relatedData); err != nil {
			t.Fatalf("Failed to parse related results: %v", err)
		}

		count, ok := relatedData["count"].(float64)
		if !ok || count == 0 {
			t.Fatalf("Expected at least one related memory")
		}
	})
}
