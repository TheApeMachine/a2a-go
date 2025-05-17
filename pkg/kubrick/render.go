package kubrick

import (
	"bufio"
	"strconv"

	"github.com/theapemachine/a2a-go/pkg/kubrick/types"
)

// RenderTo generates ANSI commands to update the terminal display,
// writing directly to the provided writer.
func (framebuffer *Framebuffer) RenderTo(writer *bufio.Writer) error {
	// Check if rendering is needed
	if !framebuffer.IsDirty() {
		// Only update cursor position if needed
		return framebuffer.setCursorPosition(writer, framebuffer.cursor.Row, framebuffer.cursor.Col)
	}

	var err error

	// Hide cursor during rendering
	if _, err = writer.WriteString(hideCursor); err != nil {
		return err
	}

	// Ensure cursor is shown after rendering
	defer writer.WriteString(showCursor)

	// Compare buffers using SIMD to find differences
	regions := framebuffer.backBuffer.CompareWith(framebuffer.frontBuffer)

	// Render each dirty region
	for _, region := range regions {
		if err = framebuffer.renderRegion(writer, region); err != nil {
			return err
		}
	}

	// Update cursor position
	if err = framebuffer.setCursorPosition(writer, framebuffer.cursor.Row, framebuffer.cursor.Col); err != nil {
		return err
	}

	// Swap buffers
	framebuffer.SwapBuffers()

	return nil
}

// renderRegion renders a specific region of the screen
func (framebuffer *Framebuffer) renderRegion(writer *bufio.Writer, region types.DirtyRegion) error {
	for row := region.StartRow; row <= region.EndRow; row++ {
		// Move cursor to start of line
		framebuffer.numBuf = framebuffer.numBuf[:0]
		framebuffer.numBuf = append(framebuffer.numBuf, '\033', '[')
		framebuffer.numBuf = strconv.AppendInt(framebuffer.numBuf, int64(row+1), 10)
		framebuffer.numBuf = append(framebuffer.numBuf, ';')
		framebuffer.numBuf = strconv.AppendInt(framebuffer.numBuf, int64(region.StartCol+1), 10)
		framebuffer.numBuf = append(framebuffer.numBuf, 'H')

		if _, err := writer.Write(framebuffer.numBuf); err != nil {
			return err
		}

		// Clear line from start position
		if _, err := writer.WriteString(clearLine); err != nil {
			return err
		}

		// Get the line slice that needs to be written
		line := framebuffer.backBuffer.GetRunes(row, region.StartCol, region.EndCol)

		// Write the line content
		for _, r := range line {
			if _, err := writer.WriteRune(r); err != nil {
				return err
			}
		}
	}

	return nil
}

// setCursorPosition generates the command to move the physical terminal cursor
func (framebuffer *Framebuffer) setCursorPosition(writer *bufio.Writer, row, col int) error {
	// Calculate screen position (1-based)
	screenRow := row + 1
	screenCol := col + 1

	// Clamp to visible area
	screenRow = max(1, min(screenRow, framebuffer.height))
	screenCol = max(1, min(screenCol, framebuffer.width))

	// Generate ANSI sequence
	framebuffer.numBuf = framebuffer.numBuf[:0]
	framebuffer.numBuf = append(framebuffer.numBuf, '\033', '[')
	framebuffer.numBuf = strconv.AppendInt(framebuffer.numBuf, int64(screenRow), 10)
	framebuffer.numBuf = append(framebuffer.numBuf, ';')
	framebuffer.numBuf = strconv.AppendInt(framebuffer.numBuf, int64(screenCol), 10)
	framebuffer.numBuf = append(framebuffer.numBuf, 'H')

	_, err := writer.Write(framebuffer.numBuf)
	return err
}
