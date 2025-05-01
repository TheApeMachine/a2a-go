package memory

import (
	"bytes"
	"io"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/types"
)

type Store interface {
	io.ReadWriteCloser
}

type UnifiedLongTerm struct {
	stores []Store
}

func NewUnifiedLongTerm(stores ...Store) *UnifiedLongTerm {
	return &UnifiedLongTerm{
		stores: stores,
	}
}

func (unified *UnifiedLongTerm) Remember(task *types.Task) error {
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if part.Type == types.PartTypeText {
				for _, store := range unified.stores {
					if _, err := store.Write([]byte(part.Text)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (unified *UnifiedLongTerm) Recall(task *types.Task) []mcp.Resource {
	resources := make([]mcp.Resource, 0)

	for _, store := range unified.stores {
		// Create a buffer to read the store's content
		buf := new(bytes.Buffer)

		// Read all content from the store
		if _, err := io.Copy(buf, store); err != nil {
			continue // Skip this store if there's an error
		}

		// Create a resource with the store's content
		resource := mcp.NewResource(
			"unified",
			"long-term",
			mcp.WithResourceDescription("Long-term memory content"),
			mcp.WithMIMEType("text/plain"),
		)

		// Set the content data using the appropriate method
		// We need to use the appropriate method to set content in the resource
		// This is a placeholder - we need to find the correct way to set content
		// For now, we'll just add the resource without content
		resources = append(resources, resource)
	}

	return resources
}

func (unified *UnifiedLongTerm) Forget(task *types.Task) error {
	for _, store := range unified.stores {
		if _, err := store.Write([]byte{}); err != nil {
			return err
		}
	}
	return nil
}
