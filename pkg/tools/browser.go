package tools

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/tools/browser"
)

type BrowserTool struct {
	tool *mcp.Tool
}

func NewBrowserTool() *mcp.Tool {
	tool := mcp.NewTool(
		"browser",
		mcp.WithDescription("A fully featured browser, useful for when you need to interact with a website."),
		mcp.WithString("url",
			mcp.Description("The URL to open in the browser"),
			mcp.Required(),
		),
	)

	return &tool
}

func (bt *BrowserTool) RegisterBrowserTools(srv *server.MCPServer) {
	srv.AddTool(*bt.tool, bt.Handle)
}

func (bt *BrowserTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("browser executing")

	browser := browser.NewBrowser()

	res, err := browser.Fetch(ctx, req.GetArguments()["url"].(string), "", false, "")

	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(res.Text), nil
}
