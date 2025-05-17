package components

import (
	"context"
	"io"
)

// Component is the base interface that all UI components must implement
type Component interface {
	io.ReadWriteCloser
	WithContext(ctx context.Context)
}