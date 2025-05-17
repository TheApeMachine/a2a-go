package layouts

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/kubrick/components"
	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
	"github.com/theapemachine/a2a-go/pkg/logging"
)

type GridLayout struct {
	*types.Contextualizer

	wg             *sync.WaitGroup
	Components     []components.Component
	Rows           int
	Columns        int
	Spacing        int
	rect           Rect
	status         types.State
	err            error
	internalBuffer *types.Buffer
}

type GridLayoutOption func(*GridLayout)

func NewGridLayout(options ...GridLayoutOption) *GridLayout {
	logging.Log("GridLayout.NewGridLayout: Called")
	layout := &GridLayout{
		Contextualizer: types.NewContextualizer(),
		wg:             &sync.WaitGroup{},
		Components:     make([]components.Component, 0),
		Rows:           1,
		Columns:        1,
		Spacing:        0,
		status:         types.StateInitialized,
		internalBuffer: types.NewBuffer(1, 1),
	}

	layout.Contextualizer.WithContext(context.Background())

	logging.Log("GridLayout.NewGridLayout: Applying options (%d)", len(options))
	for i, option := range options {
		logging.Log("GridLayout.NewGridLayout: Applying option %d", i)
		option(layout)
	}

	logging.Log("GridLayout.NewGridLayout: Starting render")
	if err := layout.render(); err != nil {
		layout.err = err
		logging.Log("GridLayout.NewGridLayout: Error from render: %v", err)
	}

	logging.Log("GridLayout.NewGridLayout: Initialization complete")
	return layout
}

// render handles continuous updates from components
func (layout *GridLayout) render() (err error) {
	layout.status = types.StateRunning
	logging.Log("GridLayout.render: Goroutine starting, status=Running")

	go func() {
		defer logging.Log("GridLayout.render: Goroutine finished")
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-layout.Done():
				logging.Log("GridLayout.render: Context done, exiting loop")
				layout.status = types.StateCanceled
				return
			case <-ticker.C:
				if layout.status != types.StateRunning || layout.internalBuffer == nil {
					continue
				}

				layout.internalBuffer.Clear()

				layout.updatePositions()

				if layout.rect.Size.Width == 0 || layout.rect.Size.Height == 0 || layout.Columns == 0 || layout.Rows == 0 {
					continue
				}

				cellWidth := (layout.rect.Size.Width - (layout.Spacing * (layout.Columns - 1))) / layout.Columns
				cellHeight := (layout.rect.Size.Height - (layout.Spacing * (layout.Rows - 1))) / layout.Rows

				if cellWidth <= 0 || cellHeight <= 0 {
					continue
				}

				for i, comp := range layout.Components {
					if i >= layout.Rows*layout.Columns {
						break
					}

					compCellRow := (i / layout.Columns)
					compCellCol := (i % layout.Columns)

					targetX := compCellCol * (cellWidth + layout.Spacing)
					targetY := compCellRow * (cellHeight + layout.Spacing)

					readBufSize := (cellWidth * 4 * cellHeight) + cellHeight
					childData := make([]byte, readBufSize)
					m, readErr := comp.Read(childData)

					if readErr != nil && readErr != io.EOF {
						continue
					}

					if m > 0 {
						lines := strings.Split(string(childData[:m]), "\n")

						for lineIdx, lineStr := range lines {
							if lineIdx >= cellHeight {
								break
							}

							currentScreenRow := targetY + lineIdx
							if currentScreenRow >= layout.internalBuffer.Height() {
								break
							}

							runesToWrite := []rune(lineStr)
							if len(runesToWrite) > cellWidth {
								runesToWrite = runesToWrite[:cellWidth]
							}

							layout.internalBuffer.WriteRunesAt(currentScreenRow, targetX, runesToWrite)
						}
					}
				}
			}
		}
	}()

	return nil
}

func (layout *GridLayout) updatePositions() {
	if layout.rect.Size.Width == 0 || layout.rect.Size.Height == 0 || layout.Columns == 0 || layout.Rows == 0 {
		return
	}

	cellWidth := (layout.rect.Size.Width - (layout.Spacing * (layout.Columns - 1))) / layout.Columns
	cellHeight := (layout.rect.Size.Height - (layout.Spacing * (layout.Rows - 1))) / layout.Rows

	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}

	for i, comp := range layout.Components {
		if i >= layout.Rows*layout.Columns {
			break
		}

		row := i / layout.Columns
		col := i % layout.Columns

		x := layout.rect.Pos.Col + (col * (cellWidth + layout.Spacing))
		y := layout.rect.Pos.Row + (row * (cellHeight + layout.Spacing))

		if container, ok := comp.(Layout); ok {
			container.SetRect(Rect{
				Pos:  Position{Row: y, Col: x},
				Size: Size{Width: cellWidth, Height: cellHeight},
			})
		}
	}
}

func (layout *GridLayout) SetRect(rect Rect) {
	logging.Log("GridLayout.SetRect: Called with rect: %+v. Current internalBuffer isNil: %t", rect, layout.internalBuffer == nil)
	layout.rect = rect
	if layout.internalBuffer == nil {
		logging.Log("GridLayout.SetRect: internalBuffer is nil")
		if rect.Size.Width > 0 && rect.Size.Height > 0 {
			logging.Log("GridLayout.SetRect: Creating new internalBuffer %dx%d", rect.Size.Width, rect.Size.Height)
			layout.internalBuffer = types.NewBuffer(rect.Size.Width, rect.Size.Height)
		} else {
			logging.Log("GridLayout.SetRect: Rect size is zero/negative (W:%d, H:%d), creating 1x1 buffer", rect.Size.Width, rect.Size.Height)
			// No need to check if internalBuffer is nil again, it's in the nil block.
			layout.internalBuffer = types.NewBuffer(1, 1)
		}
	} else {
		logging.Log("GridLayout.SetRect: internalBuffer exists. Current size W:%d, H:%d. Resizing to W:%d, H:%d", layout.internalBuffer.Width(), layout.internalBuffer.Height(), rect.Size.Width, rect.Size.Height)
		if rect.Size.Width > 0 && rect.Size.Height > 0 {
			layout.internalBuffer.Resize(rect.Size.Width, rect.Size.Height)
			logging.Log("GridLayout.SetRect: Resize completed.")
		} else {
			logging.Log("GridLayout.SetRect: New rect size is zero/negative (W:%d, H:%d). Not resizing existing buffer down to 1x1, keeping current.", rect.Size.Width, rect.Size.Height)
		}
	}
	if layout.internalBuffer != nil {
		logging.Log("GridLayout.SetRect: Finished. Buffer new size W:%d, H:%d", layout.internalBuffer.Width(), layout.internalBuffer.Height())
	} else {
		logging.Log("GridLayout.SetRect: Finished. internalBuffer is STILL NIL. THIS IS A PROBLEM.")
	}
}

func (layout *GridLayout) Read(p []byte) (n int, err error) {
	logging.Log("GridLayout.Read: Called with len(p)=%d", len(p))
	if layout.internalBuffer == nil {
		logging.Log("GridLayout.Read: internalBuffer is nil, returning EOF")
		return 0, io.EOF
	}
	n, err = layout.internalBuffer.Read(p)
	logging.Log("GridLayout.Read: Read %d bytes from internalBuffer, err: %v. Content: %s", n, err, string(p[:n]))
	return n, err
}

func (layout *GridLayout) Write(p []byte) (n int, err error) {
	logging.Log("GridLayout.Write: Called with %d bytes: %s", len(p), string(p))
	if layout.internalBuffer == nil {
		logging.Log("GridLayout.Write: internalBuffer is nil, returning 0, EOF")
		return 0, io.EOF
	}
	n, err = layout.internalBuffer.Write(p)
	logging.Log("GridLayout.Write: internalBuffer.Write returned n=%d, err=%v", n, err)
	return n, err
}

func (layout *GridLayout) Close() error {
	logging.Log("GridLayout.Close: Called. Cancelling context.")
	layout.Cancel()
	layout.status = types.StateCanceled
	var err error
	if layout.internalBuffer != nil {
		logging.Log("GridLayout.Close: Closing internalBuffer")
		err = layout.internalBuffer.Close()
		if err != nil {
			logging.Log("GridLayout.Close: Error closing internalBuffer: %v", err)
		}
	} else {
		logging.Log("GridLayout.Close: internalBuffer was nil")
	}
	return err
}

func (layout *GridLayout) WithContext(ctx context.Context) {
	logging.Log("GridLayout.WithContext: Called")
	layout.Contextualizer.WithContext(ctx)
}

func WithComponents(components ...components.Component) GridLayoutOption {
	return func(l *GridLayout) {
		logging.Log("GridLayout.WithComponents: Setting %d components", len(components))
		l.Components = components
		if l.Context() == nil {
			logging.Log("GridLayout.WithComponents: Layout context is nil when setting components!")
		}
		for i, comp := range components {
			logging.Log("GridLayout.WithComponents: Setting context for component %d (%T)", i, comp)
			comp.WithContext(l.Context())
		}
	}
}

func WithRows(rows int) GridLayoutOption {
	return func(l *GridLayout) {
		if rows > 0 {
			l.Rows = rows
		}
	}
}

func WithColumns(columns int) GridLayoutOption {
	return func(l *GridLayout) {
		if columns > 0 {
			l.Columns = columns
		}
	}
}

func WithSpacing(spacing int) GridLayoutOption {
	return func(l *GridLayout) {
		if spacing >= 0 {
			l.Spacing = spacing
		}
	}
}
