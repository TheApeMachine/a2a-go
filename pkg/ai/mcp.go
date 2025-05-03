package ai

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
ToMCPResource proxies to the existing helper on AgentCard.
*/
func (a *Agent) ToMCPResource() mcp.Resource {
	return types.ToMCPResource(&a.card)
}
