package registry

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolExecutorFunc defines the standard signature for a function that executes
// the logic of a specific tool. It receives context and parsed arguments,
// and returns a string result or an error.
type ToolExecutorFunc func(ctx context.Context, args map[string]any) (string, error)

// ToolDescriptor provides a description of a tool for use with LLM providers
type ToolDescriptor struct {
	ToolName    string              // The name of the tool
	Description string              // Description of what the tool does
	Schema      mcp.ToolInputSchema // JSON schema for the tool's parameters
}

// ToolDefinition encapsulates all necessary information about a tool,
// linking its configuration ID (SkillID) to its LLM representation (ToolName, Description, Schema)
// and its execution logic (Executor).
type ToolDefinition struct {
	SkillID     string              // The ID used in agent skill configurations (e.g., "development")
	ToolName    string              // The name presented to the LLM (e.g., "terminal")
	Description string              // The description presented to the LLM
	Schema      mcp.ToolInputSchema // The MCP input schema for the tool
	Executor    ToolExecutorFunc    // The function that executes the tool's logic
}

// toolRegistry holds the registered tool definitions, keyed by SkillID.
var (
	toolRegistry = make(map[string]ToolDefinition)
	registryMu   sync.RWMutex
)

// RegisterTool adds or updates a tool definition in the central registry.
// It should typically be called during initialization (e.g., in init() functions).
func RegisterTool(def ToolDefinition) {
	// Basic validation could be added here (e.g., ensure SkillID, ToolName, Executor are not empty)
	registryMu.Lock()
	toolRegistry[def.SkillID] = def
	registryMu.Unlock()
}

// GetToolDefinition retrieves a tool definition from the registry by its SkillID.
// Returns the definition and a boolean indicating if it was found.
func GetToolDefinition(skillID string) (ToolDefinition, bool) {
	registryMu.RLock()
	def, found := toolRegistry[skillID]
	registryMu.RUnlock()
	return def, found
}
