package layouts

import (
	"github.com/theapemachine/a2a-go/pkg/kubrick/components"
)

// StackLayout arranges components vertically or horizontally
type StackLayout struct {
	Vertical   bool
	Spacing    int
	Components []components.Component
}

func NewVerticalStackLayout(spacing int) *StackLayout {
	return &StackLayout{
		Vertical:   true,
		Spacing:    spacing,
		Components: make([]components.Component, 0),
	}
}

func NewHorizontalStackLayout(spacing int) *StackLayout {
	return &StackLayout{
		Vertical:   false,
		Spacing:    spacing,
		Components: make([]components.Component, 0),
	}
}

// Write implements io.Writer by writing to all components
func (layout *StackLayout) Write(p []byte) (n int, err error) {
	// Write to each component
	for _, comp := range layout.Components {
		if _, err := comp.Write(p); err != nil {
			return n, err
		}
	}

	return len(p), nil
}

// Close implements io.Closer by closing all components
func (layout *StackLayout) Close() error {
	// Close each component
	for _, comp := range layout.Components {
		if err := comp.Close(); err != nil {
			return err
		}
	}

	return nil
}

func WithStackComponents(components ...components.Component) func(*StackLayout) {
	return func(layout *StackLayout) {
		layout.Components = append(layout.Components, components...)
	}
}
