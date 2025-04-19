package tools

// This file implements memory-related tools that allow agents to use the unified
// long-term memory system. It includes both simple in-memory tools and more
// advanced tools that use the full unified memory interface.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/theapemachine/a2a-go/memory"
	pkgmemory "github.com/theapemachine/a2a-go/pkg/memory"
)

// defaultMemoryStore backs the simple memory_* tools when the caller did not supply
// its own implementation.
var defaultMemoryStore = memory.New()

// unifiedMemoryStore is the default implementation of the unified memory system
// It uses in-memory implementations for testing/demo purposes
var (
	defaultEmbeddingService = pkgmemory.NewMockEmbeddingService()
	defaultVectorStore      = pkgmemory.NewInMemoryVectorStore()
	defaultGraphStore       = pkgmemory.NewInMemoryGraphStore()
	unifiedMemoryStore      = pkgmemory.NewUnifiedStore(defaultEmbeddingService, defaultVectorStore, defaultGraphStore)
)

// registerMemoryTools attaches all memory tools to the supplied MCP server instance.
func registerMemoryTools(srv *server.MCPServer) {
	// Simple in-memory tools
	srv.AddTool(buildMemoryStoreTool(), handleMemoryStore)
	srv.AddTool(buildMemoryQueryTool(), handleMemoryQuery)
	srv.AddTool(buildMemorySearchTool(), handleMemorySearch)

	// Advanced unified memory tools
	srv.AddTool(buildUnifiedMemoryStoreTool(), handleUnifiedMemoryStore)
	srv.AddTool(buildUnifiedMemoryRetrieveTool(), handleUnifiedMemoryRetrieve)
	srv.AddTool(buildUnifiedMemorySearchTool(), handleUnifiedMemorySearch)
	srv.AddTool(buildUnifiedMemoryRelationTool(), handleUnifiedMemoryRelation)
	srv.AddTool(buildUnifiedMemoryRelatedTool(), handleUnifiedMemoryRelated)
}

// ---------------------------------------------------------------------------
// Simple memory tool builders (schema only – no execution logic)
// ---------------------------------------------------------------------------

func buildMemoryStoreTool() mcp.Tool {
	return mcp.NewTool(
		"memory_store",
		mcp.WithDescription("Stores a piece of content in either the vector or graph backend and returns the generated document ID."),
		mcp.WithString("content",
			mcp.Description("Textual content to store"),
			mcp.Required(),
		),
		mcp.WithString("backend",
			mcp.Description("Target backend – either 'vector' or 'graph' (default 'vector')"),
			mcp.Enum("vector", "graph"),
		),
		mcp.WithObject("metadata",
			mcp.Description("Arbitrary JSON metadata to attach to the document"),
		),
	)
}

func buildMemoryQueryTool() mcp.Tool {
	return mcp.NewTool(
		"memory_query",
		mcp.WithDescription("Retrieves a previously stored document by ID."),
		mcp.WithString("id",
			mcp.Description("Document ID returned by memory_store"),
			mcp.Required(),
		),
	)
}

func buildMemorySearchTool() mcp.Tool {
	return mcp.NewTool(
		"memory_search",
		mcp.WithDescription("Performs a substring search across the vector, graph, or both backends and returns a list of document IDs that match."),
		mcp.WithString("query",
			mcp.Description("Search term (case‑insensitive substring match)"),
			mcp.Required(),
		),
		mcp.WithString("backend",
			mcp.Description("Backend filter – 'vector', 'graph', or omit for both"),
			mcp.Enum("vector", "graph"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of matches to return (0 = no limit)"),
		),
	)
}

// ---------------------------------------------------------------------------
// Unified memory tool builders
// ---------------------------------------------------------------------------

func buildUnifiedMemoryStoreTool() mcp.Tool {
	return mcp.NewTool(
		"memory_unified_store",
		mcp.WithDescription("Stores a memory in the unified memory system with vector embedding and optional type."),
		mcp.WithString("content",
			mcp.Description("Textual content to store"),
			mcp.Required(),
		),
		mcp.WithString("type",
			mcp.Description("Type of memory (e.g., 'knowledge', 'concept', 'experience')"),
			mcp.Enum("knowledge", "concept", "experience"),
		),
		mcp.WithObject("metadata",
			mcp.Description("Arbitrary JSON metadata to attach to the memory"),
		),
	)
}

func buildUnifiedMemoryRetrieveTool() mcp.Tool {
	return mcp.NewTool(
		"memory_unified_retrieve",
		mcp.WithDescription("Retrieves a memory by ID from the unified memory system."),
		mcp.WithString("id",
			mcp.Description("Memory ID to retrieve"),
			mcp.Required(),
		),
	)
}

func buildUnifiedMemorySearchTool() mcp.Tool {
	return mcp.NewTool(
		"memory_unified_search",
		mcp.WithDescription("Searches the unified memory system for semantically similar memories."),
		mcp.WithString("query",
			mcp.Description("Natural language query to search for similar memories"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return"),
		),
		mcp.WithArray("types",
			mcp.Description("Filter by memory types (e.g., 'knowledge', 'concept')"),
		),
		mcp.WithArray("filters",
			mcp.Description("Metadata filters in the format [{field, operator, value}, ...]"),
		),
	)
}

func buildUnifiedMemoryRelationTool() mcp.Tool {
	return mcp.NewTool(
		"memory_unified_relate",
		mcp.WithDescription("Creates a relationship between two memories in the unified memory system."),
		mcp.WithString("source_id",
			mcp.Description("Source memory ID"),
			mcp.Required(),
		),
		mcp.WithString("target_id",
			mcp.Description("Target memory ID"),
			mcp.Required(),
		),
		mcp.WithString("relation_type",
			mcp.Description("Type of relationship (e.g., 'related_to', 'causes', 'supports')"),
			mcp.Required(),
		),
		mcp.WithObject("properties",
			mcp.Description("Properties of the relationship (e.g., {\"strength\": 0.8})"),
		),
	)
}

func buildUnifiedMemoryRelatedTool() mcp.Tool {
	return mcp.NewTool(
		"memory_unified_get_related",
		mcp.WithDescription("Finds memories related to a given memory through specified relationship types."),
		mcp.WithString("id",
			mcp.Description("Memory ID to find related memories for"),
			mcp.Required(),
		),
		mcp.WithArray("relation_types",
			mcp.Description("Types of relationships to traverse (e.g., ['related_to', 'includes'])"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of related memories to return"),
		),
	)
}

// ---------------------------------------------------------------------------
// Simple memory tool handlers
// ---------------------------------------------------------------------------

func handleMemoryStore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	content, _ := args["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content parameter is required")
	}

	backend, _ := args["backend"].(string)
	if backend == "" {
		backend = "vector"
	}

	// Metadata may be passed as a map OR as a JSON‑encoded string (depending
	// on how the caller constructed the argument object).  Do a quick type
	// switch so we accept both.
	var meta map[string]any
	if raw, ok := args["metadata"]; ok {
		switch v := raw.(type) {
		case map[string]any:
			meta = v
		case string:
			_ = json.Unmarshal([]byte(v), &meta) // ignore err – meta stays nil on failure
		}
	}

	id := defaultMemoryStore.Put(backend, content, meta)
	return mcp.NewToolResultText(id), nil
}

func handleMemoryQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.Params.Arguments["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id parameter is required")
	}

	doc, ok := defaultMemoryStore.Get(id)
	if !ok {
		return nil, fmt.Errorf("document not found")
	}

	// Compact JSON result.
	b, _ := json.Marshal(doc)
	return mcp.NewToolResultText(string(b)), nil
}

func handleMemorySearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	backend, _ := args["backend"].(string)

	// `limit` might come through as float64 (JSON spec) or string – handle both.
	var limit int
	switch v := args["limit"].(type) {
	case float64:
		limit = int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			limit = i
		}
	}

	ids := defaultMemoryStore.Search(query, backend, limit)
	b, _ := json.Marshal(ids)
	return mcp.NewToolResultText(string(b)), nil
}

// ---------------------------------------------------------------------------
// Unified memory tool handlers
// ---------------------------------------------------------------------------

func handleUnifiedMemoryStore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	content, _ := args["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content parameter is required")
	}

	memoryType, _ := args["type"].(string)
	if memoryType == "" {
		memoryType = "knowledge"
	}

	// Handle metadata from different possible input formats
	var metadata map[string]any
	if raw, ok := args["metadata"]; ok {
		switch v := raw.(type) {
		case map[string]any:
			metadata = v
		case string:
			if err := json.Unmarshal([]byte(v), &metadata); err != nil {
				return nil, fmt.Errorf("invalid metadata JSON: %v", err)
			}
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	// Add a timestamp if not present
	if _, ok := metadata["timestamp"]; !ok {
		metadata["timestamp"] = time.Now().Format(time.RFC3339)
	}

	// Store the memory
	id, err := unifiedMemoryStore.StoreMemory(ctx, content, metadata, memoryType)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %v", err)
	}

	result := map[string]string{
		"id":      id,
		"status":  "success",
		"message": "Memory stored successfully",
	}
	resultJSON, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func handleUnifiedMemoryRetrieve(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.Params.Arguments["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id parameter is required")
	}

	memory, err := unifiedMemoryStore.GetMemory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memory: %v", err)
	}

	// Remove the embedding from the response as it's not needed by the agent
	memory.Embedding = nil

	resultJSON, _ := json.MarshalIndent(memory, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func handleUnifiedMemorySearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Parse limit
	limit := 5 // default
	if rawLimit, ok := args["limit"]; ok {
		switch v := rawLimit.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				limit = i
			}
		}
	}

	// Parse types filter
	var types []string
	if rawTypes, ok := args["types"]; ok {
		switch v := rawTypes.(type) {
		case []interface{}:
			for _, t := range v {
				if typeStr, ok := t.(string); ok {
					types = append(types, typeStr)
				}
			}
		case string:
			// Handle comma-separated string
			if v != "" {
				types = strings.Split(v, ",")
				for i, t := range types {
					types[i] = strings.TrimSpace(t)
				}
			}
		}
	}

	// Parse metadata filters
	var filters []pkgmemory.Filter
	if rawFilters, ok := args["filters"]; ok {
		switch v := rawFilters.(type) {
		case []interface{}:
			for _, f := range v {
				if filterMap, ok := f.(map[string]interface{}); ok {
					filter := pkgmemory.Filter{}
					if field, ok := filterMap["field"].(string); ok {
						filter.Field = field
					}
					if op, ok := filterMap["operator"].(string); ok {
						filter.Operator = op
					}
					if val, ok := filterMap["value"]; ok {
						filter.Value = val
					}
					filters = append(filters, filter)
				}
			}
		case string:
			// Try to parse as JSON array
			var filtersArray []map[string]interface{}
			if err := json.Unmarshal([]byte(v), &filtersArray); err == nil {
				for _, filterMap := range filtersArray {
					filter := pkgmemory.Filter{}
					if field, ok := filterMap["field"].(string); ok {
						filter.Field = field
					}
					if op, ok := filterMap["operator"].(string); ok {
						filter.Operator = op
					}
					if val, ok := filterMap["value"]; ok {
						filter.Value = val
					}
					filters = append(filters, filter)
				}
			}
		}
	}

	// Create search parameters
	searchParams := pkgmemory.SearchParams{
		Query:   query,
		Limit:   limit,
		Types:   types,
		Filters: filters,
	}

	// Perform the search
	results, err := unifiedMemoryStore.SearchSimilar(ctx, query, searchParams)
	if err != nil {
		return nil, fmt.Errorf("search failed: %v", err)
	}

	// Remove embeddings from responses
	for i := range results {
		results[i].Embedding = nil
	}

	// Format results
	formattedResults := map[string]interface{}{
		"query":           query,
		"count":           len(results),
		"memories":        results,
		"search_params":   searchParams,
		"search_time_utc": time.Now().UTC().Format(time.RFC3339),
	}

	resultJSON, _ := json.MarshalIndent(formattedResults, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func handleUnifiedMemoryRelation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	sourceID, _ := args["source_id"].(string)
	if sourceID == "" {
		return nil, fmt.Errorf("source_id parameter is required")
	}

	targetID, _ := args["target_id"].(string)
	if targetID == "" {
		return nil, fmt.Errorf("target_id parameter is required")
	}

	relationType, _ := args["relation_type"].(string)
	if relationType == "" {
		return nil, fmt.Errorf("relation_type parameter is required")
	}

	// Handle properties
	var properties map[string]any
	if raw, ok := args["properties"]; ok {
		switch v := raw.(type) {
		case map[string]any:
			properties = v
		case string:
			if err := json.Unmarshal([]byte(v), &properties); err != nil {
				return nil, fmt.Errorf("invalid properties JSON: %v", err)
			}
		}
	}
	if properties == nil {
		properties = map[string]any{
			"created_at": time.Now().UTC().Format(time.RFC3339),
		}
	}

	// Create the relation
	err := unifiedMemoryStore.CreateRelation(ctx, sourceID, targetID, relationType, properties)
	if err != nil {
		return nil, fmt.Errorf("failed to create relation: %v", err)
	}

	result := map[string]string{
		"status":        "success",
		"source_id":     sourceID,
		"target_id":     targetID,
		"relation_type": relationType,
		"relation_id":   uuid.NewString(), // Generate a unique ID for the relation
		"message":       "Relation created successfully",
	}
	resultJSON, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func handleUnifiedMemoryRelated(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	id, _ := args["id"].(string)
	if id == "" {
		return nil, fmt.Errorf("id parameter is required")
	}

	// Parse relation types
	var relationTypes []string
	if rawTypes, ok := args["relation_types"]; ok {
		switch v := rawTypes.(type) {
		case []interface{}:
			for _, t := range v {
				if typeStr, ok := t.(string); ok {
					relationTypes = append(relationTypes, typeStr)
				}
			}
		case string:
			// Handle comma-separated string
			if v != "" {
				relationTypes = strings.Split(v, ",")
				for i, t := range relationTypes {
					relationTypes[i] = strings.TrimSpace(t)
				}
			}
		}
	}
	// If no relation types specified, use all types
	if len(relationTypes) == 0 {
		relationTypes = []string{"related_to", "includes", "causes", "supports"}
	}

	// Parse limit
	limit := 5 // default
	if rawLimit, ok := args["limit"]; ok {
		switch v := rawLimit.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				limit = i
			}
		}
	}

	// Find related memories
	relatedMemories, err := unifiedMemoryStore.FindRelated(ctx, id, relationTypes, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find related memories: %v", err)
	}

	// Remove embeddings from responses
	for i := range relatedMemories {
		relatedMemories[i].Embedding = nil
	}

	// Format results
	formattedResults := map[string]interface{}{
		"memory_id":      id,
		"relation_types": relationTypes,
		"count":          len(relatedMemories),
		"memories":       relatedMemories,
	}

	resultJSON, _ := json.MarshalIndent(formattedResults, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
