package kubrick

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
	"golang.org/x/term"
)

const (
	clearScreenHome = "\033[2J\033[H"
	clearLine       = "\033[K"
	hideCursor      = "\033[?25l"
	showCursor      = "\033[?25h"
	redText         = "\033[31m"
	grayText        = "\033[90m"
	resetAttrs      = "\033[0m"
	reverseVideo    = "\033[7m"

	// Minimum dimensions
	minWidth  = 40
	minHeight = 10
)

// Framebuffer manages the terminal display state
type Framebuffer struct {
	frontBuffer *types.Buffer
	backBuffer  *types.Buffer
	cursor      Position
	viewOffset  Position

	height    int
	heightStr string
	width     int
	widthStr  string

	// Status message display
	statusMessage     string
	statusMessageTime time.Time

	// Reusable buffer for number conversions
	numBuf []byte
}

// Position represents 2D coordinates in the buffer
type Position struct {
	Row int
	Col int
}

// NewFramebuffer creates a new framebuffer
func NewFramebuffer() *Framebuffer {
	fb := &Framebuffer{
		cursor:     Position{0, 0},
		viewOffset: Position{0, 0},
		numBuf:     make([]byte, 0, 16),
		width:      minWidth,  // Set default width
		height:     minHeight, // Set default height
	}

	// Get initial size
	fb.updateTerminalSize()

	// Initialize buffers with current dimensions
	fb.frontBuffer = types.NewBuffer(fb.width, fb.height)
	fb.backBuffer = types.NewBuffer(fb.width, fb.height)

	return fb
}

// updateTerminalSize gets the current terminal dimensions
func (framebuffer *Framebuffer) updateTerminalSize() bool {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < minWidth || height < minHeight {
		width = minWidth
		height = minHeight
	}

	if width != framebuffer.width || height != framebuffer.height {
		framebuffer.width = width
		framebuffer.height = height

		// Update string versions
		framebuffer.numBuf = framebuffer.numBuf[:0]
		framebuffer.widthStr = string(strconv.AppendInt(framebuffer.numBuf, int64(framebuffer.width), 10))
		framebuffer.numBuf = framebuffer.numBuf[:0]
		framebuffer.heightStr = string(strconv.AppendInt(framebuffer.numBuf, int64(framebuffer.height), 10))

		// Resize buffers
		if framebuffer.frontBuffer != nil {
			framebuffer.frontBuffer.Resize(width, height)
		}
		if framebuffer.backBuffer != nil {
			framebuffer.backBuffer.Resize(width, height)
		}
		return true
	}
	return false
}

// SwapBuffers swaps the front and back buffers
func (framebuffer *Framebuffer) SwapBuffers() {
	framebuffer.frontBuffer, framebuffer.backBuffer = framebuffer.backBuffer, framebuffer.frontBuffer
}

// IsDirty checks if the framebuffer requires rendering
func (framebuffer *Framebuffer) IsDirty() bool {
	return len(framebuffer.backBuffer.GetDirtyRegions()) > 0
}

// Clear clears both buffers
func (framebuffer *Framebuffer) Clear() {
	framebuffer.frontBuffer.Clear()
	framebuffer.backBuffer.Clear()
	framebuffer.cursor = Position{0, 0}
	framebuffer.viewOffset = Position{0, 0}
}

// Write writes content to the back buffer
func (framebuffer *Framebuffer) Write(row, col int, content string) {
	framebuffer.backBuffer.WriteString(row, col, content)
}

// WriteAt writes content at a specific position with optional attributes
func (framebuffer *Framebuffer) WriteAt(row, col int, content string, attrs ...string) {
	if len(attrs) > 0 {
		// Apply attributes
		var builder strings.Builder
		for _, attr := range attrs {
			builder.WriteString(attr)
		}
		builder.WriteString(content)
		builder.WriteString(resetAttrs)
		content = builder.String()
	}
	framebuffer.backBuffer.WriteString(row, col, content)
}

// SetCursor sets the cursor position
func (framebuffer *Framebuffer) SetCursor(row, col int) {
	framebuffer.cursor.Row = row
	framebuffer.cursor.Col = col
}

// ShowCursor makes the cursor visible
func (framebuffer *Framebuffer) ShowCursor() {
	framebuffer.backBuffer.MarkDirty(types.DirtyRegion{
		StartRow: framebuffer.cursor.Row,
		EndRow:   framebuffer.cursor.Row,
		StartCol: framebuffer.cursor.Col,
		EndCol:   framebuffer.cursor.Col,
	})
}

// HideCursor makes the cursor invisible
func (framebuffer *Framebuffer) HideCursor() {
	framebuffer.backBuffer.MarkDirty(types.DirtyRegion{
		StartRow: framebuffer.cursor.Row,
		EndRow:   framebuffer.cursor.Row,
		StartCol: framebuffer.cursor.Col,
		EndCol:   framebuffer.cursor.Col,
	})
}

// SetStatusMessage sets a temporary status message
func (framebuffer *Framebuffer) SetStatusMessage(msg string) {
	framebuffer.statusMessage = msg
	framebuffer.statusMessageTime = time.Now()
	framebuffer.backBuffer.MarkDirty(types.DirtyRegion{
		StartRow: framebuffer.height - 1,
		EndRow:   framebuffer.height - 1,
		StartCol: 0,
		EndCol:   framebuffer.width - 1,
	})
}

// HandleResize should be called when terminal size changes
func (framebuffer *Framebuffer) HandleResize() {
	if framebuffer.updateTerminalSize() {
		framebuffer.backBuffer.MarkDirty(types.DirtyRegion{
			StartRow: 0,
			EndRow:   framebuffer.height - 1,
			StartCol: 0,
			EndCol:   framebuffer.width - 1,
		})
	}
}

// GetSize returns the current terminal dimensions
func (framebuffer *Framebuffer) GetSize() (width, height int) {
	return framebuffer.width, framebuffer.height
}
