package a2a

// This file wires three simple memory_* MCP tools into the a2a‑go SDK.  They
// rely on the lightweight in‑memory backend implemented in the `memory`
// package so that no external services are required to run the examples or
// unit tests.

import (
    "context"
    "encoding/json"
    "fmt"
    "strconv"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"

    "github.com/theapemachine/a2a-go/memory"
)

// defaultMemoryStore backs the memory_* tools when the caller did not supply
// its own implementation.
var defaultMemoryStore = memory.New()

// registerMemoryTools attaches the three memory tools to the supplied MCP
// server instance.
func registerMemoryTools(srv *server.MCPServer) {
    srv.AddTool(buildMemoryStoreTool(),   handleMemoryStore)
    srv.AddTool(buildMemoryQueryTool(),   handleMemoryQuery)
    srv.AddTool(buildMemorySearchTool(),  handleMemorySearch)
}

// ---------------------------------------------------------------------------
// Tool builders (schema only – no execution logic)
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
// Tool handlers
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
    var meta map[string]interface{}
    if raw, ok := args["metadata"]; ok {
        switch v := raw.(type) {
        case map[string]interface{}:
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
