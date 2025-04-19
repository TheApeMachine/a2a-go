package roots

import (
    "context"
    "encoding/json"

    "github.com/mark3labs/mcp-go/mcp"
)

type MCPHandler struct{ manager *Manager }

func NewMCPHandler(m *Manager) *MCPHandler { return &MCPHandler{manager: m} }

// ListRoots
func (h *MCPHandler) HandleListRoots(ctx context.Context) (*mcp.ListRootsResult, error) {
    rs, err := h.manager.List(ctx)
    if err != nil {
        return nil, err
    }
    out := make([]mcp.Root, len(rs))
    for i, r := range rs {
        out[i] = mcp.Root{URI: r.URI, Name: r.Name}
    }
    return &mcp.ListRootsResult{Roots: out}, nil
}

// CreateRoot – params: {"uri":"...","name":"..."}
func (h *MCPHandler) HandleCreateRoot(ctx context.Context, raw json.RawMessage) (*mcp.Root, error) {
    var p struct {
        URI  string `json:"uri"`
        Name string `json:"name,omitempty"`
    }
    if err := json.Unmarshal(raw, &p); err != nil {
        return nil, err
    }
    r, err := h.manager.Create(ctx, Root{URI: p.URI, Name: p.Name})
    if err != nil {
        return nil, err
    }
    return &mcp.Root{URI: r.URI, Name: r.Name}, nil
}
