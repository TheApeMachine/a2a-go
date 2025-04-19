package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestAgentCardToMCPResource(t *testing.T) {
	url := "https://example.com/.well-known/agent.json"
	desc := "Example agent"
	card := types.AgentCard{
		Name:        "ExampleAgent",
		URL:         url,
		Description: &desc,
		Capabilities: types.AgentCapabilities{
			Streaming: true,
		},
		Skills: []types.AgentSkill{},
	}

	res := ToMCPResource(card)

	if res.URI != url {
		t.Fatalf("uri mismatch: got %s", res.URI)
	}
	if res.Name != card.Name {
		t.Fatalf("name mismatch")
	}
	if res.Description != desc {
		t.Fatalf("description mismatch")
	}
	if res.MIMEType != "application/json" {
		t.Fatalf("mime mismatch")
	}
	if res.Annotations == nil || len(res.Annotations.Audience) == 0 {
		t.Fatalf("annotations missing")
	}
	if res.Annotations.Audience[0] != mcp.RoleUser {
		t.Fatalf("audience mismatch")
	}
}

func TestAgentSkillToMCPTool(t *testing.T) {
	skill := types.AgentSkill{
		ID:   "echo",
		Name: "Echo",
	}

	tool := ToMCPTool(skill)

	if tool.Name != skill.ID {
		t.Fatalf("tool name mismatch")
	}
	// Should have one property called "task" in schema
	if len(tool.InputSchema.Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(tool.InputSchema.Properties))
	}
	if _, ok := tool.InputSchema.Properties["task"]; !ok {
		t.Fatalf("property 'task' missing")
	}
}
