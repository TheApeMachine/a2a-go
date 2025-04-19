package service

import (
	"context"
	"testing"

	"github.com/theapemachine/a2a-go/pkg/stores"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestEchoTaskManagerPartValidation(t *testing.T) {
	store := stores.NewInMemoryTaskStore()
	manager := NewEchoTaskManager(store)
	ctx := context.Background()

	tests := []struct {
		name        string
		message     types.Message
		expectError bool
	}{
		{
			name: "Valid text part",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{Type: types.PartTypeText, Text: "Hello"},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid text part - empty text",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{Type: types.PartTypeText, Text: ""},
				},
			},
			expectError: true,
		},
		{
			name: "Invalid part - wrong type/field combo",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{Type: types.PartTypeText, File: &types.FilePart{URI: "https://example.com"}},
				},
			},
			expectError: true,
		},
		{
			name: "Invalid file part - both bytes and URI",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeFile,
						File: &types.FilePart{
							Bytes: "base64data",
							URI:   "https://example.com",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "Valid file part with bytes",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeFile,
						File: &types.FilePart{
							Bytes: "base64data",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid file part with URI",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeFile,
						File: &types.FilePart{
							URI: "https://example.com",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid data part",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeData,
						Data: map[string]any{"key": "value"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid data part - empty map",
			message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeData,
						Data: map[string]any{},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test SendTask
			params := types.TaskSendParams{
				ID:      "test-id",
				Message: tt.message,
			}
			_, err := manager.SendTask(ctx, params)
			
			if tt.expectError && err == nil {
				t.Errorf("SendTask expected an error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("SendTask unexpected error: %v", err)
			}
			
			// Test StreamTask
			stream, err := manager.StreamTask(ctx, params)
			
			if tt.expectError && err == nil {
				t.Errorf("StreamTask expected an error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("StreamTask unexpected error: %v", err)
			}
			
			// Clean up if a stream was created
			if stream != nil {
				for range stream {
					// Drain the channel
				}
			}
		})
	}
}