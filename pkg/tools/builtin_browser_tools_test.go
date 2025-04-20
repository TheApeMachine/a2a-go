package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

type MockTransport struct{}

func (t *MockTransport) Start(context.Context) error {
	return nil
}

func (t *MockTransport) SendRequest(context.Context, transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	return nil, nil
}

func (t *MockTransport) SendNotification(context.Context, mcp.JSONRPCNotification) error {
	return nil
}

func (t *MockTransport) SetNotificationHandler(func(mcp.JSONRPCNotification)) {
	return
}

func (t *MockTransport) Close() error {
	return nil
}

func TestRegisterBrowserTools(t *testing.T) {
	srv := server.NewMCPServer("test", "1.0")
	registerBrowserTools(srv)

	c := client.NewClient(&MockTransport{})

	// Verify browser_fetch tool is registered
	if fetchTools, err := c.ListTools(t.Context(), mcp.ListToolsRequest{}); err != nil {
		t.Fatalf("failed to list tools: %v", err)
	} else {
		assert.NotNil(t, fetchTools)
		assert.Equal(t, "browser_fetch", fetchTools.Tools[0].Name)
		assert.Contains(t, fetchTools.Tools[0].Description, "Fetches a web page")
	}

	// Verify browser_screenshot tool is registered
	if screenshotTools, err := c.ListTools(t.Context(), mcp.ListToolsRequest{}); err != nil {
		t.Fatalf("failed to list tools: %v", err)
	} else {
		assert.NotNil(t, screenshotTools)
		assert.Equal(t, "browser_screenshot", screenshotTools.Tools[0].Name)
		assert.Contains(t, screenshotTools.Tools[0].Description, "Takes a screenshot")
	}
}

func TestHandleBrowserFetch(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		args          map[string]any
		expectError   bool
		errorContains string
	}{
		{
			name:          "Missing URL",
			args:          map[string]any{},
			expectError:   true,
			errorContains: "url parameter is required",
		},
		{
			name: "Empty URL",
			args: map[string]any{
				"url": "",
			},
			expectError:   true,
			errorContains: "url parameter is required",
		},
		{
			name: "Valid URL",
			args: map[string]any{
				"url": "https://example.com",
			},
			expectError: false,
		},
		{
			name: "Valid URL with selector",
			args: map[string]any{
				"url":      "https://example.com",
				"selector": "#main-content",
			},
			expectError: false,
		},
		{
			name: "Valid URL with screenshot",
			args: map[string]any{
				"url":             "https://example.com",
				"take_screenshot": true,
			},
			expectError: false,
		},
		{
			name: "Valid URL with wait selector",
			args: map[string]any{
				"url":               "https://example.com",
				"wait_for_selector": ".loaded",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Request: mcp.Request{
					Method: "browser_fetch",
				},
				Params: struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Arguments: tt.args,
				},
			}

			result, err := handleBrowserFetch(ctx, req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestHandleBrowserScreenshot(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		args          map[string]any
		expectError   bool
		errorContains string
	}{
		{
			name:          "Missing URL",
			args:          map[string]any{},
			expectError:   true,
			errorContains: "url parameter is required",
		},
		{
			name: "Empty URL",
			args: map[string]any{
				"url": "",
			},
			expectError:   true,
			errorContains: "url parameter is required",
		},
		{
			name: "Valid URL",
			args: map[string]any{
				"url": "https://example.com",
			},
			expectError: false,
		},
		{
			name: "Valid URL with wait selector",
			args: map[string]any{
				"url":               "https://example.com",
				"wait_for_selector": ".loaded",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Request: mcp.Request{
					Method: "browser_screenshot",
				},
				Params: struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Arguments: tt.args,
				},
			}

			result, err := handleBrowserScreenshot(ctx, req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
