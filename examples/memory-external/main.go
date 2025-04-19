// memory-external demonstrates using the unified memory system with external
// databases (Qdrant for vector storage and Neo4j for graph storage).
//
// Before running this example, start the databases with:
//   docker-compose -f docker-compose.memory.yml up -d
//
// You'll also need to set your OpenAI API key in the environment:
//   export OPENAI_API_KEY=sk-...
//
// Then run the example:
//   go run ./examples/memory-external
//
// You can also explore the databases in your browser:
// - Qdrant: http://localhost:6333/dashboard
// - Neo4j: http://localhost:7474/browser/

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/theapemachine/a2a-go/pkg/memory"
)

func main() {
	// Check for OpenAI API key
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== External Memory Stores Demo ===")
	fmt.Println("Connecting to Qdrant (vector store) and Neo4j (graph store)...")

	// Initialize the memory system components
	embeddingService := memory.NewOpenAIEmbeddingService(openaiKey)
	vectorStore := memory.NewQdrantVectorStore("http://localhost:6333", "memories", embeddingService)
	graphStore := memory.NewNeo4jGraphStore("http://localhost:7474", "neo4j", "password")
	
	// Test connections
	if err := vectorStore.Ping(ctx); err != nil {
		log.Printf("Warning: Qdrant connection failed: %v", err)
		log.Println("Is Qdrant running? Try: docker-compose -f docker-compose.memory.yml up -d")
		log.Println("Falling back to in-memory vector store")
		vectorStore = memory.NewInMemoryVectorStore()
	} else {
		fmt.Println("✅ Connected to Qdrant vector store")
	}

	if err := graphStore.Ping(ctx); err != nil {
		log.Printf("Warning: Neo4j connection failed: %v", err)
		log.Println("Is Neo4j running? Try: docker-compose -f docker-compose.memory.yml up -d")
		log.Println("Falling back to in-memory graph store")
		graphStore = memory.NewInMemoryGraphStore()
	} else {
		fmt.Println("✅ Connected to Neo4j graph store")
	}

	// Create the unified store
	unifiedStore := memory.NewUnifiedStore(embeddingService, vectorStore, graphStore)
	
	fmt.Println("\n=== Demo 1: Storing Memories ===")
	// Store a few memories
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

	id3, err := unifiedStore.StoreMemory(ctx, "Vector stores like Qdrant are great for semantic similarity search", 
		map[string]any{"topic": "memory", "subtopic": "vector"}, "knowledge")
	if err != nil {
		log.Fatalf("Failed to store third memory: %v", err)
	}
	fmt.Printf("✅ Stored knowledge memory with ID: %s\n", id3)

	id4, err := unifiedStore.StoreMemory(ctx, "Graph databases like Neo4j excel at relationship queries", 
		map[string]any{"topic": "memory", "subtopic": "graph"}, "knowledge")
	if err != nil {
		log.Fatalf("Failed to store fourth memory: %v", err)
	}
	fmt.Printf("✅ Stored knowledge memory with ID: %s\n", id4)
	
	fmt.Println("\n=== Demo 2: Creating Relationships ===")
	// Create relationships between memories
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
	
	fmt.Println("\n=== Demo 3: Semantic Search ===")
	// Perform a semantic search
	searchParams := memory.SearchParams{
		Query:       "databases for storing AI memory",
		Limit:       10,
		Types:       []string{"knowledge", "concept"},
	}
	
	results, err := unifiedStore.SearchSimilar(ctx, searchParams.Query, searchParams)
	if err != nil {
		log.Fatalf("Failed to search memories: %v", err)
	}
	
	fmt.Printf("✅ Found %d semantically similar memories:\n", len(results))
	for i, mem := range results {
		fmt.Printf("  %d. %s (ID: %s, Type: %s)\n", i+1, mem.Content, mem.ID, mem.Type)
	}
	
	fmt.Println("\n=== Demo 4: Finding Related Memories ===")
	// Find memories related to a concept
	related, err := unifiedStore.FindRelated(ctx, id2, []string{"includes"}, 10)
	if err != nil {
		log.Fatalf("Failed to find related memories: %v", err)
	}
	
	fmt.Printf("✅ Found %d memories related to memory system concept:\n", len(related))
	for i, mem := range related {
		fmt.Printf("  %d. %s (ID: %s)\n", i+1, mem.Content, mem.ID)
	}
	
	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Successfully demonstrated the unified memory system with external stores!")
	fmt.Println("Explore the data in your browser:")
	fmt.Println("- Qdrant: http://localhost:6333/dashboard")
	fmt.Println("- Neo4j: http://localhost:7474/browser/ (login with neo4j/password)")
}