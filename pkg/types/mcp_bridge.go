package types

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/registry"
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
func ToMCPResource(card *AgentCard) mcp.Resource {
	var res mcp.Resource

	// Check if card is nil
	if card == nil {
		return res
	}

	// Basic resource with required fields
	res = mcp.NewResource(
		card.URL,
		card.Name,
		mcp.WithMIMEType("application/json"),
	)

	// Add description if available
	if card.Description != nil {
		res = mcp.NewResource(
			card.URL,
			card.Name,
			mcp.WithResourceDescription(*card.Description),
			mcp.WithMIMEType("application/json"),
		)
	}

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

// GetToolDefinition is a wrapper for registry.GetToolDefinition
// which is used to avoid circular dependencies between packages
func GetToolDefinition(skillID string) (registry.ToolDefinition, bool) {
	return registry.GetToolDefinition(skillID)
}

/*
ToMCPTool converts an AgentSkill to an MCP Tool.
*/
func ToMCPTool(skill AgentSkill) *MCPClient {
	name, description, tool := getTool(skill.ID)

	client := &MCPClient{
		Name:        name,
		Description: description,
		Schema:      &tool.InputSchema,
	}

	return client
}

type MCPClient struct {
	Name        string
	Description string
	Schema      *mcp.ToolInputSchema
	Toolcall    *mcp.CallToolRequest
}

// ToToolDescriptor converts an MCPClient to a registry.ToolDescriptor
func (m *MCPClient) ToToolDescriptor() *registry.ToolDescriptor {
	return &registry.ToolDescriptor{
		ToolName:    m.Name,
		Description: m.Description,
		Schema:      *m.Schema,
	}
}

func getTool(id string) (string, string, mcp.Tool) {
	switch id {
	case "development":
		return "terminal", "A fully featured Debian Linux terminal, useful for development tasks or anything that may require access to a computer.", getDockerTool()
	}

	return "", "", mcp.Tool{}
}

// getDockerTool returns a docker tool
func getDockerTool() mcp.Tool {
	// Create a placeholder implementation since the tools package reference is broken
	// We'll need to implement this properly after fixing the circular dependency
	var tool mcp.Tool
	tool.InputSchema.Type = "object"

	// Add the command property
	tool.InputSchema.Properties = map[string]interface{}{
		"command": map[string]interface{}{
			"type":        "string",
			"description": "The command to execute in the terminal",
		},
	}
	tool.InputSchema.Required = []string{"command"}

	return tool
}
