package ai

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func (agent *Agent) execute(
	ctx context.Context, tool *types.MCPClient, args map[string]any,
) (string, error) {
	log.Info("executing tool", "tool", tool.Schema.Properties["name"], "args", args)
	return "", nil
}
