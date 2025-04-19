package prompts

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tj/assert"
)

func TestDefaultManagerList(t *testing.T) {
	manager := NewDefaultManager()
	ctx := context.Background()

	// List should return seeded prompts
	prompts, err := manager.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, prompts, 2) // Two seeded prompts

	// Verify seeded single-step prompt
	var singlePrompt *Prompt
	for i := range prompts {
		if prompts[i].Type == SingleStepPrompt {
			singlePrompt = &prompts[i]
			break
		}
	}
	assert.NotNil(t, singlePrompt)
	assert.Equal(t, "Greeting", singlePrompt.Name)
	assert.Equal(t, "A friendly greeting", singlePrompt.Description)
	assert.Equal(t, "Hello – how can I help you today?", singlePrompt.Content)

	// Verify seeded multi-step prompt
	var multiPrompt *Prompt
	for i := range prompts {
		if prompts[i].Type == MultiStepPrompt {
			multiPrompt = &prompts[i]
			break
		}
	}
	assert.NotNil(t, multiPrompt)
	assert.Equal(t, "Customer‑Support", multiPrompt.Name)
	assert.Equal(t, "4‑step customer support flow", multiPrompt.Description)
}

func TestDefaultManagerCRUD(t *testing.T) {
	manager := NewDefaultManager()
	ctx := context.Background()

	// Create a new prompt
	newPrompt := Prompt{
		Name:        "Test Prompt",
		Description: "Test Description",
		Type:        SingleStepPrompt,
		Content:     "Test Content",
		Version:     "1.0.0",
	}

	created, err := manager.Create(ctx, newPrompt)
	assert.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, newPrompt.Name, created.Name)
	assert.Equal(t, newPrompt.Content, created.Content)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), created.UpdatedAt, time.Second)

	// Get the prompt
	retrieved, err := manager.Get(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)

	// Update the prompt
	retrieved.Content = "Updated Content"
	updated, err := manager.Update(ctx, *retrieved)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Content", updated.Content)
	assert.Equal(t, created.CreatedAt, updated.CreatedAt)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt))

	// Delete the prompt
	err = manager.Delete(ctx, created.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = manager.Get(ctx, created.ID)
	assert.Error(t, err)
	assert.IsType(t, ErrorPromptNotFound{}, err)
}

func TestDefaultManagerSteps(t *testing.T) {
	manager := NewDefaultManager()
	ctx := context.Background()

	// Create a multi-step prompt
	prompt := Prompt{
		Name:        "Multi-Step Test",
		Description: "Test Multi-Step Prompt",
		Type:        MultiStepPrompt,
		Content:     "Parent content",
		Version:     "1.0.0",
	}

	created, err := manager.Create(ctx, prompt)
	assert.NoError(t, err)

	// Create steps
	steps := []PromptStep{
		{
			PromptID:    created.ID,
			Name:        "Step 1",
			Description: "First step",
			Content:     "Step 1 content",
			Order:       1,
		},
		{
			PromptID:    created.ID,
			Name:        "Step 2",
			Description: "Second step",
			Content:     "Step 2 content",
			Order:       2,
		},
	}

	// Add steps
	for i := range steps {
		step, err := manager.CreateStep(ctx, steps[i])
		assert.NoError(t, err)
		assert.NotEmpty(t, step.ID)
		steps[i] = *step
	}

	// Get steps
	retrievedSteps, err := manager.GetSteps(ctx, created.ID)
	assert.NoError(t, err)
	assert.Len(t, retrievedSteps, 2)

	// Update a step
	steps[0].Content = "Updated step 1 content"
	updated, err := manager.UpdateStep(ctx, steps[0])
	assert.NoError(t, err)
	assert.Equal(t, "Updated step 1 content", updated.Content)

	// Delete a step
	err = manager.DeleteStep(ctx, steps[0].ID)
	assert.NoError(t, err)

	// Verify step deletion
	remainingSteps, err := manager.GetSteps(ctx, created.ID)
	assert.NoError(t, err)
	assert.Len(t, remainingSteps, 1)
	assert.Equal(t, steps[1].ID, remainingSteps[0].ID)
}

func TestDefaultManagerErrors(t *testing.T) {
	manager := NewDefaultManager()
	ctx := context.Background()

	// Test getting non-existent prompt
	_, err := manager.Get(ctx, "non-existent")
	assert.Error(t, err)
	assert.IsType(t, ErrorPromptNotFound{}, err)

	// Test updating non-existent prompt
	_, err = manager.Update(ctx, Prompt{ID: "non-existent"})
	assert.Error(t, err)
	assert.IsType(t, ErrorPromptNotFound{}, err)

	// Test getting steps for single-step prompt
	singlePrompt := Prompt{
		ID:      uuid.NewString(),
		Name:    "Single",
		Type:    SingleStepPrompt,
		Content: "Content",
	}
	created, err := manager.Create(ctx, singlePrompt)
	assert.NoError(t, err)

	_, err = manager.GetSteps(ctx, created.ID)
	assert.Error(t, err)
	assert.IsType(t, ErrorInvalidPromptType{}, err)

	// Test creating step for non-existent prompt
	_, err = manager.CreateStep(ctx, PromptStep{PromptID: "non-existent"})
	assert.Error(t, err)
	assert.IsType(t, ErrorPromptNotFound{}, err)

	// Test updating non-existent step
	_, err = manager.UpdateStep(ctx, PromptStep{ID: "non-existent", PromptID: created.ID})
	assert.Error(t, err)
	assert.IsType(t, ErrorStepNotFound{}, err)

	// Test deleting non-existent step
	err = manager.DeleteStep(ctx, "non-existent")
	assert.Error(t, err)
	assert.IsType(t, ErrorStepNotFound{}, err)
}
