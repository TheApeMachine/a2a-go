package provider

import (
	"context"

	"github.com/theapemachine/a2a-go/pkg/registry" // Import registry
	"github.com/theapemachine/a2a-go/pkg/types"    // Import types
)

/*
ToolExecutor abstracts the execution of an MCP tool.  Users should implement
this to wire in their own business logic / data sources.
*/
// Use registry.ToolDescriptor for the tool description
type ToolExecutor func(ctx context.Context, tool *registry.ToolDescriptor, args map[string]any) (string, error)

type Interface interface {
	// Pass registry.ToolDescriptor map
	Complete(context.Context, *types.Task, *map[string]*registry.ToolDescriptor) error
	Stream(context.Context, *types.Task, *map[string]*registry.ToolDescriptor, func(*types.Task)) error
	Embed(context.Context, string) ([]float32, error)
	EmbedBatch(context.Context, []string) ([][]float32, error)
}
