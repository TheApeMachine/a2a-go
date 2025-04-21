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
	Complete(ctx context.Context, task *types.Task, tools *map[string]*types.MCPClient) (err error)
	Stream(ctx context.Context, messages []types.Message, tools *map[string]*types.MCPClient, onDelta func(string)) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}
