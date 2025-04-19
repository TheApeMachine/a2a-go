package a2a

// This file bundles the built‑in MCP tools that ship with the a2a‑go SDK.  The
// first one is an "Agent Orchestrator" tool capable of creating ephemeral
// child agents and delegating sub‑tasks to them.  At the moment the
// implementation is intentionally lightweight: it keeps state in the
// InMemoryTaskStore and echoes back simple status messages.  The primary goal
// is to provide a working end‑to‑end example that developers can extend.

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// RegisterBuiltInTools installs all official tools onto the given MCP server.
// The caller may pass a nil *InMemoryTaskStore to use the package‑level default.
func RegisterBuiltInTools(srv *server.MCPServer, store *InMemoryTaskStore) {
    if store == nil {
        store = defaultTaskStore
    }

    // ------------------------------------------------------------------
    // Agent Orchestrator tool
    // ------------------------------------------------------------------
    orchestratorTool := mcp.NewTool(
        "agent_orchestrator",
        mcp.WithDescription("Creates ephemeral child agents, breaks down complex objectives into sub‑tasks, and tracks their execution.  Returns a task ID that can be polled for status."),
        mcp.WithString("objective",
            mcp.Description("High‑level objective the orchestrator should achieve"),
            mcp.Required(),
        ),
        mcp.WithNumber("max_depth",
            mcp.Description("Maximum recursion depth when spawning sub‑agents (default 3)"),
        ),
    )

    srv.AddTool(orchestratorTool, makeOrchestratorHandler(store))
}

// defaultTaskStore is lazily created if the user does not provide their own.
var defaultTaskStore = NewInMemoryTaskStore()

// makeOrchestratorHandler returns the MCP tool handler closure capturing the
// task store instance.
func makeOrchestratorHandler(store *InMemoryTaskStore) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        obj, _ := req.Params.Arguments["objective"].(string)
        if strings.TrimSpace(obj) == "" {
            return nil, fmt.Errorf("missing objective parameter")
        }
        // Optional parameter.
        maxDepth, _ := req.Params.Arguments["max_depth"].(float64) // JSON numbers are float64
        if maxDepth == 0 {
            maxDepth = 3
        }

        // Create the parent task entry so the caller can track progress.
        parentID := fmt.Sprintf("orch‑%d", time.Now().UnixNano())
        store.Create(parentID, obj)
        store.UpdateState(parentID, TaskStateWorking)

        // Very naive decomposition strategy – split the objective on the word
        // "and" or on sentence terminators.  In real life this would be an
        // LLM‑powered planning step but a heuristic is sufficient for a
        // functional demo.
        subObjs := decomposeObjective(obj)
        if len(subObjs) == 0 {
            subObjs = []string{obj}
        }

        // Cap recursion depth (maxDepth refers to *levels*, not number of
        // sub‑tasks).
        if maxDepth < 1 {
            maxDepth = 1
        }

        // Spawn a goroutine that works through the sub‑tasks and updates their
        // state over time.  The caller immediately receives the parent task
        // ID and can poll/subscribe for updates.
        go func() {
            for i, sub := range subObjs {
                // Child task ID embeds parent for easier correlation.
                childID := fmt.Sprintf("%s‑%d", parentID, i+1)
                store.CreateChild(childID, sub, parentID)
                store.UpdateState(childID, TaskStateWorking)

                // Simulate some processing work.
                time.Sleep(300 * time.Millisecond)
                store.UpdateState(childID, TaskStateCompleted)
            }

            // All children done → mark parent completed.
            store.UpdateState(parentID, TaskStateCompleted)
        }()

        msg := fmt.Sprintf("Created orchestrator task %s with %d sub‑tasks – objective accepted", parentID, len(subObjs))
        return mcp.NewToolResultText(msg), nil
    }
}

// decomposeObjective applies a few simple heuristics to split a high‑level
// objective string into smaller pieces.  It purposefully stays deterministic
// and free of external dependencies so unit tests and demos remain stable.
func decomposeObjective(obj string) []string {
    // First split on the word " and ".
    if parts := strings.Split(obj, " and "); len(parts) > 1 {
        return trimParts(parts)
    }
    // Otherwise split on '.' and ';'.
    var tmp []string
    for _, s := range strings.FieldsFunc(obj, func(r rune) bool {
        return r == '.' || r == ';'
    }) {
        tmp = append(tmp, s)
    }
    return trimParts(tmp)
}

func trimParts(parts []string) []string {
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        if s := strings.TrimSpace(p); s != "" {
            out = append(out, s)
        }
    }
    return out
}
