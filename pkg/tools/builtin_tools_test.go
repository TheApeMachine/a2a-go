package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/stores"
)

func TestOrchestratorHandlerCreatesTask(t *testing.T) {
	store := stores.NewInMemoryTaskStore()
	handler := makeOrchestratorHandler(store)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"objective": "write docs"}

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("empty content")
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok || !strings.HasPrefix(tc.Text, "Created orchestrator task") {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(store.List()) == 0 {
		t.Fatalf("no task stored")
	}
}
