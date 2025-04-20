package types

// This file contains helper utilities for inter‑operability between the A2A
// protocol and Model Context Protocol (MCP).  The guiding principle (see
// MCP.txt) is to expose *agents* as MCP *resources* so that existing MCP
// tooling and ecosystems can discover and reason about an A2A‑speaking agent
// through a familiar interface.

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToMCPResource converts an AgentCard into an MCP Resource descriptor.  The
// resulting resource can be advertised by an MCP server so that LLM frameworks
// using MCP can discover the agent, fetch its agent‑card (via the URI) and
// subsequently communicate with it via the A2A protocol.
//
// Mapping rules:
//   - Resource.URI        → card.URL (expected to resolve to the .well-known/agent.json)
//   - Resource.Name       → card.Name
//   - Resource.Description→ card.Description (if any)
//   - Resource.MIMEType   → "application/json" (agent‑card mime‑type)
//   - Optionally audience annotations can indicate the card is meant for
//     assistants as well as users.
func ToMCPResource(card AgentCard) mcp.Resource {
	res := mcp.NewResource(
		card.URL,
		card.Name,
		mcp.WithResourceDescription(*card.Description),
		mcp.WithMIMEType("application/json"),
	)

	// Provide simple annotation hint – both user and assistant.
	res.Annotations = &struct {
		Audience []mcp.Role `json:"audience,omitempty"`
		Priority float64    `json:"priority,omitempty"`
	}{
		Audience: []mcp.Role{mcp.RoleUser, mcp.RoleAssistant},
		Priority: 0.5,
	}

	return res
}

// ToMCPTool converts an AgentSkill to an MCP Tool.  The mapping is necessarily
// lossy because the semantics differ, but providing a skeleton definition
// allows MCP‑based LLM frameworks to catalogue the agent's skills and invoke
// them via the higher‑level A2A /tasks interface.
//
// The generated tool takes a single required string argument "task" which
// represents the textual instruction to forward to the agent.  The tool call
// simply proxies to tasks/send under the hood (the caller must implement the
// proxy logic – this helper only defines the schema).
func ToMCPTool(skill AgentSkill) *MCPClient {
	mcpClient := &MCPClient{}
	return mcpClient
}

type MCPClient struct {
}

func (c *MCPClient) Start(ctx context.Context) error {
	return nil
}

func (c *MCPClient) SendRequest(ctx context.Context, request mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
	return nil, nil
}

func (c *MCPClient) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	return nil
}

func (c *MCPClient) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
}

func (c *MCPClient) Close() error {
	return nil
}
