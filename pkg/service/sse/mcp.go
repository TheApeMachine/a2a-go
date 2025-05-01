package sse

import (
	"net/http"

	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/tools"
)

type MCPBroker struct {
	srv *server.MCPServer
	sse *server.SSEServer
}

func NewMCPBroker() *MCPBroker {
	mcpSrv := server.NewMCPServer(
		"mcp-server",
		"1.0.0",
		server.WithLogging(),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithToolCapabilities(true),
	)

	tools.RegisterDockerTools(mcpSrv)

	sseSrv := server.NewSSEServer(
		mcpSrv,
	)

	return &MCPBroker{
		srv: mcpSrv,
		sse: sseSrv,
	}
}

func (b *MCPBroker) Start() error {
	return b.sse.Start("0.0.0.0:3210")
}

func (b *MCPBroker) Server() http.Handler {
	return b.sse
}
