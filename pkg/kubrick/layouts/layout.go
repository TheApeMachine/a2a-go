package layouts

import (
	"context"
	"io"
)

// Layout is an interface for objects that can arrange components
type Layout interface {
	io.ReadWriteCloser
	SetRect(rect Rect)
	WithContext(ctx context.Context)
}