package resources

import (
    "context"

    "github.com/mark3labs/mcp-go/mcp"
)

// MCPHandler turns a ResourceManager into MCP JSON‑RPC methods.
type MCPHandler struct{ manager ResourceManager }

func NewMCPHandler(m ResourceManager) *MCPHandler { return &MCPHandler{manager: m} }

func (h *MCPHandler) HandleListResources(ctx context.Context, _ *mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
    rs, tmpl, err := h.manager.List(ctx)
    if err != nil {
        return nil, err
    }
    out := make([]mcp.Resource, len(rs))
    for i, r := range rs {
        out[i] = mcp.NewResource(r.URI, r.Name, mcp.WithResourceDescription(r.Description), mcp.WithMIMEType(r.MimeType))
    }
    tpl := make([]mcp.ResourceTemplate, len(tmpl))
    for i, t := range tmpl {
        tpl[i] = mcp.NewResourceTemplate(t.URITemplate, t.Name, mcp.WithTemplateDescription(t.Description), mcp.WithTemplateMIMEType(t.MimeType))
    }
    res := &mcp.ListResourcesResult{Resources: out}
    if len(tpl) > 0 {
        res.Meta = map[string]any{"templates": tpl}
    }
    return res, nil
}

func (h *MCPHandler) HandleReadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
    cs, err := h.manager.Read(ctx, req.Params.URI)
    if err != nil {
        return nil, err
    }
    mcpContents := make([]mcp.ResourceContents, len(cs))
    for i, c := range cs {
        if c.Text != "" {
            mcpContents[i] = &mcp.TextResourceContents{URI: c.URI, MIMEType: c.MimeType, Text: c.Text}
        } else {
            mcpContents[i] = &mcp.BlobResourceContents{URI: c.URI, MIMEType: c.MimeType, Blob: c.Blob}
        }
    }
    return &mcp.ReadResourceResult{Contents: mcpContents}, nil
}
