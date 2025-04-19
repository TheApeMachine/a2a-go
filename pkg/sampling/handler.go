package sampling

import (
    "context"

    "github.com/mark3labs/mcp-go/mcp"
)

type MCPHandler struct{ manager Manager }

func NewMCPHandler(m Manager) *MCPHandler { return &MCPHandler{manager: m} }

func (h *MCPHandler) HandleCreateMessage(ctx context.Context, req *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
    prefs := ModelPreferences{Temperature: req.Params.Temperature, MaxTokens: req.Params.MaxTokens, Stop: req.Params.StopSequences}
    // collect context messages
    messages := make([]Message, len(req.Params.Messages))
    for i, mmsg := range req.Params.Messages {
        txt := ""
        if c, ok := mmsg.Content.(*mcp.TextContent); ok {
            txt = c.Text
        }
        messages[i] = Message{Role: string(mmsg.Role), Content: txt}
    }
    opts := SamplingOptions{ModelPreferences: prefs, Context: &Context{Messages: messages}}
    result, err := h.manager.CreateMessage(ctx, req.Params.SystemPrompt, opts)
    if err != nil {
        return nil, err
    }
    return &mcp.CreateMessageResult{
        SamplingMessage: mcp.SamplingMessage{Role: mcp.Role(result.Message.Role), Content: mcp.NewTextContent(result.Message.Content)},
        // model field left blank for now
    }, nil
}

// HandleStreamMessage returns a channel of incremental results suitable for
// server push.
func (h *MCPHandler) HandleStreamMessage(ctx context.Context, req *mcp.CreateMessageRequest) (<-chan *mcp.CreateMessageResult, error) {
    prefs := ModelPreferences{Temperature: req.Params.Temperature, MaxTokens: req.Params.MaxTokens, Stop: req.Params.StopSequences}

    // convert context messages same as CreateMessage
    messages := make([]Message, len(req.Params.Messages))
    for i, msg := range req.Params.Messages {
        txt := ""
        if tc, ok := msg.Content.(*mcp.TextContent); ok {
            txt = tc.Text
        }
        messages[i] = Message{Role: string(msg.Role), Content: txt}
    }

    opts := SamplingOptions{ModelPreferences: prefs, Context: &Context{Messages: messages}, Stream: true}
    stream, err := h.manager.StreamMessage(ctx, req.Params.SystemPrompt, opts)
    if err != nil {
        return nil, err
    }

    out := make(chan *mcp.CreateMessageResult)
    go func() {
        defer close(out)
        for res := range stream {
            out <- &mcp.CreateMessageResult{
                SamplingMessage: mcp.SamplingMessage{Role: mcp.Role(res.Message.Role), Content: mcp.NewTextContent(res.Message.Content)},
            }
        }
    }()
    return out, nil
}
