package a2a

// This file wires a single browser_fetch tool into the a2a‑go SDK using the
// rod headless browser backend.  The tool lets an agent retrieve a web page
// (or DOM subset) and returns cleaned text plus metadata as JSON.

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"

    "github.com/theapemachine/a2a-go/pkg/tools/browser"
)

func registerBrowserTools(srv *server.MCPServer) {
    tool := mcp.NewTool(
        "browser_fetch",
        mcp.WithDescription("Fetches a web page in a headless browser and returns title, final URL and visible text (truncated to 4 KB)."),
        mcp.WithString("url",
            mcp.Description("Absolute http/https URL to navigate to"),
            mcp.Required(),
        ),
        mcp.WithString("selector",
            mcp.Description("Optional CSS selector to scope text extraction"),
        ),
    )

    srv.AddTool(tool, handleBrowserFetch)
}

func handleBrowserFetch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments
    url, _ := args["url"].(string)
    if url == "" {
        return nil, fmt.Errorf("url parameter is required")
    }
    selector, _ := args["selector"].(string)

    res, err := browser.Fetch(ctx, url, selector)
    if err != nil {
        return nil, err
    }

    b, _ := json.Marshal(res)
    return mcp.NewToolResultText(string(b)), nil
}
