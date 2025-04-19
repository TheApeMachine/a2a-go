// memory-store is a comprehensive demonstration of the unified long-term memory system
// that combines vector and graph stores for AI agents. This example shows how to use
// the in-memory implementation for testing and development purposes. In production,
// you would replace this with real Qdrant and Neo4j implementations.
//
//   go run ./examples/memory-store

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/theapemachine/a2a-go/pkg/memory"
)

func main() {
	// Create context with timeout for our operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize the memory system components
	embeddingService := memory.NewMockEmbeddingService()
	vectorStore := memory.NewInMemoryVectorStore()
	graphStore := memory.NewInMemoryGraphStore()
	unifiedStore := memory.NewUnifiedStore(embeddingService, vectorStore, graphStore)

	fmt.Println("=== Unified Long-Term Memory System Demo ===")
	fmt.Println()

	// Demo 1: Storing different types of memories
	fmt.Println("=== Demo 1: Storing Memories ===")
	id1, err := unifiedStore.StoreMemory(ctx, "AI agents can communicate using the A2A protocol", 
		map[string]any{"topic": "agents", "importance": 8}, "knowledge")
	if err != nil {
		log.Fatalf("Failed to store first memory: %v", err)
	}
	fmt.Printf("✅ Stored knowledge memory with ID: %s\n", id1)

	id2, err := unifiedStore.StoreMemory(ctx, "The unified memory system combines vector and graph databases", 
		map[string]any{"topic": "memory", "importance": 9}, "concept")
	if err != nil {
		log.Fatalf("Failed to store second memory: %v", err)
	}
	fmt.Printf("✅ Stored concept memory with ID: %s\n", id2)

	id3, err := unifiedStore.StoreMemory(ctx, "Vector stores are great for semantic similarity search", 
		map[string]any{"topic": "memory", "subtopic": "vector"}, "knowledge")
	if err != nil {
		log.Fatalf("Failed to store third memory: %v", err)
	}
	fmt.Printf("✅ Stored knowledge memory with ID: %s\n", id3)

	id4, err := unifiedStore.StoreMemory(ctx, "Graph databases excel at relationship queries", 
		map[string]any{"topic": "memory", "subtopic": "graph"}, "knowledge")
	if err != nil {
		log.Fatalf("Failed to store fourth memory: %v", err)
	}
	fmt.Printf("✅ Stored knowledge memory with ID: %s\n", id4)
	fmt.Println()

	// Demo 2: Creating relationships between memories
	fmt.Println("=== Demo 2: Creating Relationships ===")
	err = unifiedStore.CreateRelation(ctx, id2, id3, "includes", map[string]any{"strength": 0.8})
	if err != nil {
		log.Fatalf("Failed to create relation: %v", err)
	}
	fmt.Printf("✅ Created 'includes' relationship from %s to %s\n", id2, id3)

	err = unifiedStore.CreateRelation(ctx, id2, id4, "includes", map[string]any{"strength": 0.9})
	if err != nil {
		log.Fatalf("Failed to create relation: %v", err)
	}
	fmt.Printf("✅ Created 'includes' relationship from %s to %s\n", id2, id4)

	err = unifiedStore.CreateRelation(ctx, id1, id2, "related_to", map[string]any{"strength": 0.5})
	if err != nil {
		log.Fatalf("Failed to create relation: %v", err)
	}
	fmt.Printf("✅ Created 'related_to' relationship from %s to %s\n", id1, id2)
	fmt.Println()

	// Demo 3: Retrieving memories by ID
	fmt.Println("=== Demo 3: Retrieving Memories ===")
	memory2, err := unifiedStore.GetMemory(ctx, id2)
	if err != nil {
		log.Fatalf("Failed to retrieve memory: %v", err)
	}
	memJSON, _ := json.MarshalIndent(memory2, "", "  ")
	fmt.Printf("✅ Retrieved memory by ID:\n%s\n\n", string(memJSON))

	// Demo 4: Semantic search
	fmt.Println("=== Demo 4: Semantic Search ===")
	searchParams := memory.SearchParams{
		Query:       "vector databases for AI memory",
		Limit:       10,
		Types:       []string{"knowledge", "concept"},
		Collections: []string{},
	}
	
	results, err := unifiedStore.SearchSimilar(ctx, searchParams.Query, searchParams)
	if err != nil {
		log.Fatalf("Failed to search memories: %v", err)
	}
	
	fmt.Printf("✅ Found %d semantically similar memories:\n", len(results))
	for i, mem := range results {
		fmt.Printf("  %d. %s (ID: %s, Type: %s)\n", i+1, mem.Content, mem.ID, mem.Type)
	}
	fmt.Println()

	// Demo 5: Finding related memories through graph relationships
	fmt.Println("=== Demo 5: Finding Related Memories ===")
	related, err := unifiedStore.FindRelated(ctx, id2, []string{"includes", "related_to"}, 10)
	if err != nil {
		log.Fatalf("Failed to find related memories: %v", err)
	}
	
	fmt.Printf("✅ Found %d memories related to ID %s:\n", len(related), id2)
	for i, mem := range related {
		fmt.Printf("  %d. %s (ID: %s)\n", i+1, mem.Content, mem.ID)
	}
	fmt.Println()

	// Demo 6: Filtered search 
	fmt.Println("=== Demo 6: Filtered Search ===")
	filteredParams := memory.SearchParams{
		Query: "memory system",
		Limit: 10,
		Filters: []memory.Filter{
			{Field: "topic", Operator: "eq", Value: "memory"},
		},
	}
	
	filteredResults, err := unifiedStore.SearchSimilar(ctx, filteredParams.Query, filteredParams)
	if err != nil {
		log.Fatalf("Failed to perform filtered search: %v", err)
	}
	
	fmt.Printf("✅ Found %d memories with topic='memory':\n", len(filteredResults))
	for i, mem := range filteredResults {
		fmt.Printf("  %d. %s (ID: %s)\n", i+1, mem.Content, mem.ID)
		if meta, ok := mem.Metadata["topic"]; ok {
			fmt.Printf("     Topic: %v\n", meta)
		}
	}
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println("The unified memory system successfully demonstrated:")
	fmt.Println("1. Storing memories in both vector and graph stores")
	fmt.Println("2. Creating relationships between memories")
	fmt.Println("3. Retrieving memories by ID")
	fmt.Println("4. Semantic similarity search")
	fmt.Println("5. Graph-based relationship queries")
	fmt.Println("6. Filtered metadata search")
}