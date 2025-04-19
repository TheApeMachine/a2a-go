package a2a

import (
    "testing"

    "github.com/mark3labs/mcp-go/mcp"
)

func TestAgentCardToMCPResource(t *testing.T) {
    url := "https://example.com/.well-known/agent.json"
    desc := "Example agent"
    card := AgentCard{
        Name:        "ExampleAgent",
        URL:         url,
        Description: &desc,
        Capabilities: AgentCapabilities{
            Streaming: true,
        },
        Skills: []AgentSkill{},
    }

    res := card.ToMCPResource()

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
    skill := AgentSkill{
        ID:   "echo",
        Name: "Echo",
    }

    tool := skill.ToMCPTool()

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
