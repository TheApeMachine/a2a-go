package types

import (
	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/tools"
)

func SkillsToTools(skills []a2a.AgentSkill) []*mcp.Tool {
	// Initialize an empty slice with a capacity if desired, or just empty.
	mcpTools := make([]*mcp.Tool, 0, len(skills))
	seenTools := make(map[string]bool)

	for _, skill := range skills {
		tool, err := ToMCPTool(skill)

		if err != nil {
			log.Error("failed to acquire tool", "error", err, "skill_id", skill.ID)
			// Decide if a nil tool should be added or if the loop should just skip this tool
			// For now, skipping seems more appropriate than adding a nil.
			continue
		}

		if tool != nil { // Ensure the acquired tool is not nil before appending
			if _, seen := seenTools[tool.Name]; !seen {
				mcpTools = append(mcpTools, tool)
				seenTools[tool.Name] = true
			}
		}
	}

	return mcpTools
}

func ToMCPTool(skill a2a.AgentSkill) (*mcp.Tool, error) {
	return tools.Acquire(skill.ID)
}
