package tools

// This file wires browser tools into the a2a‑go SDK using the
// rod headless browser backend. The tools let an agent retrieve a web page
// (or DOM subset), take screenshots, and returns cleaned text plus metadata as JSON.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/theapemachine/a2a-go/pkg/tools/browser"
)

func registerBrowserTools(srv *server.MCPServer) {
	// Register browser_fetch tool
	fetchTool := mcp.NewTool(
		"browser_fetch",
		mcp.WithDescription("Fetches a web page in a headless browser and returns title, final URL and visible text (truncated to 4 KB)."),
		mcp.WithString("url",
			mcp.Description("Absolute http/https URL to navigate to"),
			mcp.Required(),
		),
		mcp.WithString("selector",
			mcp.Description("Optional CSS selector to scope text extraction"),
		),
		mcp.WithBoolean("take_screenshot",
			mcp.Description("Optional flag to take a screenshot of the page"),
		),
		mcp.WithString("wait_for_selector",
			mcp.Description("Optional CSS selector to wait for before extracting content"),
		),
	)
	srv.AddTool(fetchTool, handleBrowserFetch)
	
	// Register browser_screenshot tool
	screenshotTool := mcp.NewTool(
		"browser_screenshot",
		mcp.WithDescription("Takes a screenshot of a web page in a headless browser and returns it as a base64-encoded image."),
		mcp.WithString("url",
			mcp.Description("Absolute http/https URL to navigate to"),
			mcp.Required(),
		),
		mcp.WithString("wait_for_selector",
			mcp.Description("Optional CSS selector to wait for before taking the screenshot"),
		),
	)
	srv.AddTool(screenshotTool, handleBrowserScreenshot)
}

func handleBrowserFetch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	url, _ := args["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url parameter is required")
	}
	
	selector, _ := args["selector"].(string)
	takeScreenshot, _ := args["take_screenshot"].(bool)
	waitForSelector, _ := args["wait_for_selector"].(string)

	res, err := browser.Fetch(ctx, url, selector, takeScreenshot, waitForSelector)
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(res)
	return mcp.NewToolResultText(string(b)), nil
}

func handleBrowserScreenshot(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments
	url, _ := args["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url parameter is required")
	}
	
	waitForSelector, _ := args["wait_for_selector"].(string)

	// Use the browser.Fetch function with empty selector and takeScreenshot set to true
	res, err := browser.Fetch(ctx, url, "", true, waitForSelector)
	if err != nil {
		return nil, err
	}
	
	if !res.HasScreenshot {
		return nil, fmt.Errorf("failed to take screenshot")
	}

	b, _ := json.Marshal(map[string]interface{}{
		"title":      res.Title,
		"url":        res.URL,
		"screenshot": res.Screenshot,
		"duration_ms": res.Duration,
	})
	return mcp.NewToolResultText(string(b)), nil
}