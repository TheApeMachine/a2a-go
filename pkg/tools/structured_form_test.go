package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestNewFormPart(t *testing.T) {
	// Test with all parameters
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	data := map[string]any{
		"name": "John",
	}
	instructions := "Please fill out the form"

	part := NewFormPart(schema, data, instructions)
	assert.Equal(t, types.PartTypeData, part.Type)
	assert.NotNil(t, part.Data)

	// Verify the form payload structure
	assert.Equal(t, "form", part.Data["type"])
	assert.Equal(t, schema, part.Data["form"])
	assert.Equal(t, data, part.Data["form_data"])
	assert.Equal(t, instructions, part.Data["instructions"])

	// Test with nil parameters
	part = NewFormPart(nil, nil, "")
	assert.Equal(t, types.PartTypeData, part.Type)
	assert.NotNil(t, part.Data)

	assert.Equal(t, "form", part.Data["type"])
	assert.NotNil(t, part.Data["form"])
	assert.NotNil(t, part.Data["form_data"])
	assert.Empty(t, part.Data["instructions"])
}

func TestIsFormPart(t *testing.T) {
	// Test valid form part
	validPart := types.Part{
		Type: types.PartTypeData,
		Data: map[string]any{
			"type":         "form",
			"form":         map[string]any{"type": "object"},
			"form_data":    map[string]any{"field": "value"},
			"instructions": "Fill this out",
		},
	}

	payload, isForm := IsFormPart(validPart)
	assert.True(t, isForm)
	assert.Equal(t, "form", payload.Type)
	assert.Equal(t, map[string]any{"type": "object"}, payload.Form)
	assert.Equal(t, map[string]any{"field": "value"}, payload.FormData)
	assert.Equal(t, "Fill this out", payload.Instructions)

	// Test non-form part
	nonFormPart := types.Part{
		Type: types.PartTypeData,
		Data: map[string]any{
			"type": "not-form",
		},
	}

	payload, isForm = IsFormPart(nonFormPart)
	assert.False(t, isForm)
	assert.Empty(t, payload)

	// Test non-data part
	textPart := types.Part{
		Type: types.PartTypeText,
		Text: "Some text",
	}

	payload, isForm = IsFormPart(textPart)
	assert.False(t, isForm)
	assert.Empty(t, payload)

	// Test nil data
	nilDataPart := types.Part{
		Type: types.PartTypeData,
		Data: nil,
	}

	payload, isForm = IsFormPart(nilDataPart)
	assert.False(t, isForm)
	assert.Empty(t, payload)
}

func TestNewInputRequiredStatus(t *testing.T) {
	// Test with form part
	formPart := types.Part{
		Type: types.PartTypeData,
		Data: map[string]any{
			"type": "form",
			"form": map[string]any{"type": "object"},
		},
	}

	status := NewInputRequiredStatus("", formPart)
	assert.Equal(t, types.TaskStateInputReq, status.State)
	assert.NotNil(t, status.Message)
	assert.Equal(t, "agent", status.Message.Role)
	assert.Len(t, status.Message.Parts, 1)
	assert.Equal(t, formPart, status.Message.Parts[0])

	// Test with prompt only
	status = NewInputRequiredStatus("Please provide input", types.Part{})
	assert.Equal(t, types.TaskStateInputReq, status.State)
	assert.NotNil(t, status.Message)
	assert.Equal(t, "agent", status.Message.Role)
	assert.Len(t, status.Message.Parts, 1)
	assert.Equal(t, types.PartTypeText, status.Message.Parts[0].Type)
	assert.Equal(t, "Please provide input", status.Message.Parts[0].Text)
}
