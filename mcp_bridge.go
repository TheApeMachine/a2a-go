package a2a

// This file contains helper utilities for inter‑operability between the A2A
// protocol and Model Context Protocol (MCP).  The guiding principle (see
// MCP.txt) is to expose *agents* as MCP *resources* so that existing MCP
// tooling and ecosystems can discover and reason about an A2A‑speaking agent
// through a familiar interface.

import (
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
)

// ToMCPResource converts an AgentCard into an MCP Resource descriptor.  The
// resulting resource can be advertised by an MCP server so that LLM frameworks
// using MCP can discover the agent, fetch its agent‑card (via the URI) and
// subsequently communicate with it via the A2A protocol.
//
// Mapping rules:
//   • Resource.URI        → card.URL (expected to resolve to the .well-known/agent.json)
//   • Resource.Name       → card.Name
//   • Resource.Description→ card.Description (if any)
//   • Resource.MIMEType   → "application/json" (agent‑card mime‑type)
//   • Optionally audience annotations can indicate the card is meant for
//     assistants as well as users.
func (card AgentCard) ToMCPResource() mcp.Resource {
    res := mcp.NewResource(
        card.URL,
        card.Name,
        mcp.WithResourceDescription(deref(card.Description)),
        mcp.WithMIMEType("application/json"),
    )

    // Provide simple annotation hint – both user and assistant.
    res.Annotations = &struct {
        Audience []mcp.Role  `json:"audience,omitempty"`
        Priority float64     `json:"priority,omitempty"`
    }{
        Audience: []mcp.Role{mcp.RoleUser, mcp.RoleAssistant},
        Priority: 0.5,
    }

    return res
}

// ToMCPTags converts an AgentSkill to an MCP Tool.  The mapping is necessarily
// lossy because the semantics differ, but providing a skeleton definition
// allows MCP‑based LLM frameworks to catalogue the agent's skills and invoke
// them via the higher‑level A2A /tasks interface.
//
// The generated tool takes a single required string argument "task" which
// represents the textual instruction to forward to the agent.  The tool call
// simply proxies to tasks/send under the hood (the caller must implement the
// proxy logic – this helper only defines the schema).
func (skill AgentSkill) ToMCPTool() mcp.Tool {
    desc := deref(skill.Description)
    if desc == "" {
        desc = fmt.Sprintf("Invoke skill %s of the target A2A agent", skill.Name)
    }

    tool := mcp.NewTool(skill.ID,
        mcp.WithDescription(desc),
        mcp.WithString("task",
            mcp.Description("Free‑form user instruction to perform with this skill"),
            mcp.Required(),
        ),
    )
    return tool
}

func deref(ptr *string) string {
    if ptr == nil {
        return ""
    }
    return *ptr
}
