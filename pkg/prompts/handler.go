package prompts

import (
    "context"

    "github.com/mark3labs/mcp-go/mcp"
)

// MCPHandler adapts PromptManager to the MCP JSON‑RPC methods.
// Only the subset we need initially is implemented: prompts/list and
// prompts/get.
type MCPHandler struct {
    manager PromptManager
}

func NewMCPHandler(m PromptManager) *MCPHandler { return &MCPHandler{manager: m} }

// HandleListPrompts implements prompts/list.
func (h *MCPHandler) HandleListPrompts(ctx context.Context, _ *mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
    ps, err := h.manager.List(ctx)
    if err != nil {
        return nil, err
    }
    out := make([]mcp.Prompt, len(ps))
    for i, p := range ps {
        out[i] = mcp.NewPrompt(p.Name, mcp.WithPromptDescription(p.Description))
    }
    return mcp.NewListPromptsResult(out, ""), nil
}

// HandleGetPrompt implements prompts/get (returns full content).
func (h *MCPHandler) HandleGetPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
    // The spec uses name, we store by ID but Name is unique in demo. Iterate.
    all, err := h.manager.List(ctx)
    if err != nil {
        return nil, err
    }
    var pr *Prompt
    for _, p := range all {
        if p.Name == req.Params.Name {
            pr = &p
            break
        }
    }
    if pr == nil {
        return nil, ErrorPromptNotFound{ID: req.Params.Name}
    }

    msgs := []mcp.PromptMessage{
        mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(pr.Content)),
    }
    if pr.Type == MultiStepPrompt {
        steps, err := h.manager.GetSteps(ctx, pr.ID)
        if err != nil {
            return nil, err
        }
        for _, s := range steps {
            msgs = append(msgs, mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(s.Content)))
        }
    }
    return mcp.NewGetPromptResult(pr.Description, msgs), nil
}
