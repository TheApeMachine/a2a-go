package provider

import (
	"context"

	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
ToolExecutor abstracts the execution of an MCP tool.  Users should implement
this to wire in their own business logic / data sources.
*/
type ToolExecutor func(
	ctx context.Context, tool *types.MCPClient, args map[string]any,
) (string, error)

type Interface interface {
	Complete(context.Context, *types.Task, *map[string]*types.MCPClient) error
	Stream(context.Context, *types.Task, *map[string]*types.MCPClient, func(string)) error
	Embed(context.Context, string) ([]float32, error)
	EmbedBatch(context.Context, []string) ([][]float32, error)
}
