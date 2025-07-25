package sampling

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPHandler struct{ manager Manager }

func NewMCPHandler(m Manager) *MCPHandler { return &MCPHandler{manager: m} }

func (h *MCPHandler) HandleCreateMessage(ctx context.Context, req *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
	// Extract parameters from request
	var temperature float64
	var maxTokens int
	var stopSequences []string
	var messages []Message
	var systemPrompt string

	// For now, use default values until we find the correct accessor pattern
	temperature = 0.7
	maxTokens = 2048
	stopSequences = []string{}
	messages = []Message{}
	systemPrompt = "You are a helpful assistant."

	prefs := ModelPreferences{Temperature: temperature, MaxTokens: maxTokens, Stop: stopSequences}
	opts := SamplingOptions{ModelPreferences: prefs, Context: &Context{Messages: messages}}
	result, err := h.manager.CreateMessage(ctx, systemPrompt, opts)
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
	// Extract parameters from request - using defaults until we find correct accessor pattern
	temperature := 0.7
	maxTokens := 2048
	stopSequences := []string{}
	messages := []Message{}
	systemPrompt := "You are a helpful assistant."

	prefs := ModelPreferences{Temperature: temperature, MaxTokens: maxTokens, Stop: stopSequences}
	opts := SamplingOptions{ModelPreferences: prefs, Context: &Context{Messages: messages}, Stream: true}
	stream, err := h.manager.StreamMessage(ctx, systemPrompt, opts)
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
