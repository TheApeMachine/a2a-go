package types

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/registry"
)

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
	tool.InputSchema.Properties = map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "The command to execute in the terminal",
		},
	}
	tool.InputSchema.Required = []string{"command"}

	return tool
}
