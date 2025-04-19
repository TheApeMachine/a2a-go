package prompts

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tj/assert"
)

func TestHandleListPrompts(t *testing.T) {
	manager := NewDefaultManager()
	handler := NewMCPHandler(manager)
	ctx := context.Background()

	// Test listing prompts
	result, err := handler.HandleListPrompts(ctx, &mcp.ListPromptsRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Prompts, 2) // Two seeded prompts

	// Verify prompt fields are correctly mapped
	for _, p := range result.Prompts {
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Description)
	}
}

func TestHandleGetPrompt(t *testing.T) {
	manager := NewDefaultManager()
	handler := NewMCPHandler(manager)
	ctx := context.Background()

	// Get list of prompts first to find a valid name
	prompts, err := manager.List(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, prompts)

	// Test getting single-step prompt
	var singlePromptName string
	for _, p := range prompts {
		if p.Type == SingleStepPrompt {
			singlePromptName = p.Name
			break
		}
	}
	assert.NotEmpty(t, singlePromptName)

	result, err := handler.HandleGetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: singlePromptName,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 1) // Single step prompt has one message

	// Test getting multi-step prompt
	var multiPromptName string
	for _, p := range prompts {
		if p.Type == MultiStepPrompt {
			multiPromptName = p.Name
			break
		}
	}
	assert.NotEmpty(t, multiPromptName)

	result, err = handler.HandleGetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: multiPromptName,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, len(result.Messages) > 1) // Multi-step prompt has multiple messages

	// Test getting non-existent prompt
	result, err = handler.HandleGetPrompt(ctx, &mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "non-existent",
		},
	})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, ErrorPromptNotFound{}, err)
}
