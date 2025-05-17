package sse

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/server"
)

type MCPBroker struct {
	stdio *server.MCPServer
	sse   *server.SSEServer
}

func NewMCPBroker(hostname string) *MCPBroker {
	hooks := &server.Hooks{}
	hooks.AddOnRequestInitialization(func(ctx context.Context, id any, messageRaw any) error {
		log.Info("MCP Server: AddOnRequestInitialization hook called", "sessionID_from_context", id)
		return nil
	})

	mcpSrv := server.NewMCPServer(
		"mcp-server",
		"1.0.0",
		server.WithLogging(),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
		server.WithHooks(hooks),
	)

	baseURL := fmt.Sprintf("http://%s:3210", hostname)
	sseSrv := server.NewSSEServer(
		mcpSrv,
		server.WithBaseURL(baseURL),
	)

	return &MCPBroker{
		stdio: mcpSrv,
		sse:   sseSrv,
	}
}

// MCPServer returns the underlying *server.MCPServer instance.
func (broker *MCPBroker) MCPServer() (*server.MCPServer, *server.SSEServer) {
	return broker.stdio, broker.sse
}

func (broker *MCPBroker) Start() error {
	return broker.sse.Start(":3210")
}
