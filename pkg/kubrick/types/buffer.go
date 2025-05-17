package types

import (
	"bytes"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/theapemachine/a2a-go/pkg/logging" // Import logging package
)

// DirtyRegion represents a rectangular area that needs updating
type DirtyRegion struct {
	StartRow, EndRow int
	StartCol, EndCol int
}

// Buffer represents a single screen buffer
type Buffer struct {
	data          [][]rune
	width         int
	height        int
	dirtyRegions  []DirtyRegion
	pooledRegions []DirtyRegion // Pre-allocated region pool
	regionMu      sync.RWMutex

	// For implementing io.Reader by serializing 'data'
	readBuffer *bytes.Buffer
	readMu     sync.Mutex // To protect readBuffer and serialization
	dataHash   uint64     // To detect changes in data for readBuffer invalidation (optional, simple reset for now)
}

// NewBuffer creates a new buffer with given dimensions
func NewBuffer(width, height int) *Buffer {
	// logging.Log("Buffer.NewBuffer: Creating buffer %dx%d", width, height) // Can be verbose
	b := &Buffer{
		width:         width,
		height:        height,
		data:          globalBufferPool.GetBuffer(height, width), // globalBufferPool is now in package types
		pooledRegions: make([]DirtyRegion, 0, 16),
		readBuffer:    bytes.NewBuffer(make([]byte, 0, width*height*2)), // Estimate initial capacity
	}
	return b
}

// Width returns the width of the buffer.
func (buffer *Buffer) Width() int {
	buffer.regionMu.RLock()
	defer buffer.regionMu.RUnlock()
	return buffer.width
}

// Height returns the height of the buffer.
func (buffer *Buffer) Height() int {
	buffer.regionMu.RLock()
	defer buffer.regionMu.RUnlock()
	return buffer.height
}

// MarkDirty marks a region as needing updates
func (buffer *Buffer) MarkDirty(region DirtyRegion) {
	buffer.regionMu.Lock()
	defer buffer.regionMu.Unlock()

	// Clamp region to buffer bounds
	if region.StartRow < 0 {
		region.StartRow = 0
	}
	if region.EndRow >= buffer.height {
		region.EndRow = buffer.height - 1
	}
	if region.StartCol < 0 {
		region.StartCol = 0
	}
	if region.EndCol >= buffer.width {
		region.EndCol = buffer.width - 1
	}

	// Try to merge with existing regions
	for i := 0; i < len(buffer.dirtyRegions); i++ {
		if buffer.regionsOverlap(region, buffer.dirtyRegions[i]) {
			// Merge regions
			buffer.dirtyRegions[i] = buffer.mergeRegions(region, buffer.dirtyRegions[i])
			return
		}
	}

	// No overlap found, add new region
	if len(buffer.pooledRegions) > 0 {
		// Reuse a pooled region
		buffer.dirtyRegions = append(buffer.dirtyRegions, region)
		buffer.pooledRegions = buffer.pooledRegions[:len(buffer.pooledRegions)-1]
	} else {
		buffer.dirtyRegions = append(buffer.dirtyRegions, region)
	}
}

// Clear removes all content and dirty regions
func (buffer *Buffer) Clear() {
	buffer.regionMu.Lock()

	// Return current buffer to pool and get a fresh one
	globalBufferPool.PutBuffer(buffer.data)
	buffer.data = globalBufferPool.GetBuffer(buffer.height, buffer.width)
	buffer.dirtyRegions = buffer.dirtyRegions[:0]

	// Invalidate read buffer
	buffer.readMu.Lock()
	if buffer.readBuffer != nil {
		buffer.readBuffer.Reset()
	}
	buffer.readMu.Unlock()
	buffer.regionMu.Unlock()
}

// GetDirtyRegions returns a copy of current dirty regions
func (buffer *Buffer) GetDirtyRegions() []DirtyRegion {
	buffer.regionMu.RLock()
	defer buffer.regionMu.RUnlock()

	result := make([]DirtyRegion, len(buffer.dirtyRegions))
	copy(result, buffer.dirtyRegions)
	return result
}

// ClearDirtyRegions removes all dirty region markers
func (buffer *Buffer) ClearDirtyRegions() {
	buffer.regionMu.Lock()
	buffer.pooledRegions = append(buffer.pooledRegions, buffer.dirtyRegions...)
	buffer.dirtyRegions = buffer.dirtyRegions[:0]
	buffer.regionMu.Unlock()
}

// regionsOverlap checks if two regions overlap or are adjacent
func (buffer *Buffer) regionsOverlap(r1, r2 DirtyRegion) bool {
	rowOverlap := r1.StartRow <= r2.EndRow+1 && r2.StartRow <= r1.EndRow+1
	colOverlap := r1.StartCol <= r2.EndCol+1 && r2.StartCol <= r1.EndCol+1
	return rowOverlap && colOverlap
}

// mergeRegions combines two overlapping or adjacent regions
func (buffer *Buffer) mergeRegions(r1, r2 DirtyRegion) DirtyRegion {
	return DirtyRegion{
		StartRow: min(r1.StartRow, r2.StartRow), // Using built-in min
		EndRow:   max(r1.EndRow, r2.EndRow),     // Using built-in max
		StartCol: min(r1.StartCol, r2.StartCol), // Using built-in min
		EndCol:   max(r1.EndCol, r2.EndCol),     // Using built-in max
	}
}

// WriteRunesAt writes content to the buffer at specified position
func (buffer *Buffer) WriteRunesAt(row, col int, content []rune) {
	if row < 0 || row >= buffer.height || col < 0 || col >= buffer.width {
		return
	}

	endCol := min(col+len(content), buffer.width) // Using built-in min
	copy(buffer.data[row][col:endCol], content[:endCol-col])

	buffer.MarkDirty(DirtyRegion{
		StartRow: row,
		EndRow:   row,
		StartCol: col,
		EndCol:   endCol - 1,
	})

	buffer.readMu.Lock()
	if buffer.readBuffer != nil {
		buffer.readBuffer.Reset()
	}
	buffer.readMu.Unlock()
}

// WriteString writes a string to the buffer at specified position
func (buffer *Buffer) WriteString(row, col int, content string) {
	buffer.WriteRunesAt(row, col, []rune(content))
}

// Write implements io.Writer for Buffer.
func (buffer *Buffer) Write(p []byte) (n int, err error) {
	logging.Log("Buffer.Write: Called with %d bytes. Current buffer %dx%d. Content: %s", len(p), buffer.width, buffer.height, string(p))
	buffer.regionMu.Lock()
	defer buffer.regionMu.Unlock()

	buffer.readMu.Lock()
	if buffer.readBuffer != nil {
		buffer.readBuffer.Reset()
	}
	buffer.readMu.Unlock()

	for r := 0; r < buffer.height; r++ {
		for c := 0; c < buffer.width; c++ {
			if buffer.data[r] != nil {
				buffer.data[r][c] = ' '
			}
		}
	}

	logging.Log("Buffer.Write: Cleared buffer. Starting to write new content.")
	writeRow, writeCol := 0, 0
	bytesProcessed := 0

	for len(p) > 0 && writeRow < buffer.height {
		r, size := utf8.DecodeRune(p)
		p = p[size:]
		bytesProcessed += size

		if r == utf8.RuneError && size == 1 {
			continue
		}

		if r == '\n' {
			writeRow++
			writeCol = 0
			continue
		}

		if writeCol < buffer.width {
			if buffer.data[writeRow] != nil {
				buffer.data[writeRow][writeCol] = r
			}
			writeCol++
		} else {
			writeRow++
			writeCol = 0
			if writeRow < buffer.height && writeCol < buffer.width {
				if buffer.data[writeRow] != nil {
					buffer.data[writeRow][writeCol] = r
				}
				writeCol++
			}
		}
	}

	buffer.MarkDirty(DirtyRegion{0, buffer.height - 1, 0, buffer.width - 1})
	logging.Log("Buffer.Write: Finished writing. Processed %d bytes. Marked entire buffer dirty.", bytesProcessed)
	return bytesProcessed, nil
}

// CopyFrom copies content from another buffer
func (buffer *Buffer) CopyFrom(other *Buffer) {
	buffer.regionMu.Lock()
	other.regionMu.RLock()
	defer buffer.regionMu.Unlock()
	defer other.regionMu.RUnlock()

	minHeight := min(buffer.height, other.height) // Using built-in min
	minWidth := min(buffer.width, other.width)    // Using built-in min

	for i := 0; i < minHeight; i++ {
		if buffer.data[i] != nil && other.data[i] != nil {
			copy(buffer.data[i][:minWidth], other.data[i][:minWidth])
		}
	}

	buffer.readMu.Lock()
	if buffer.readBuffer != nil {
		buffer.readBuffer.Reset()
	}
	buffer.readMu.Unlock()
	buffer.MarkDirty(DirtyRegion{0, minHeight - 1, 0, minWidth - 1})
}

// Resize adjusts buffer size, preserving content where possible
func (buffer *Buffer) Resize(width, height int) {
	logging.Log("Buffer.Resize: Called for buffer %dx%d to new size %dx%d", buffer.width, buffer.height, width, height)
	buffer.regionMu.Lock()
	defer buffer.regionMu.Unlock()

	if width == buffer.width && height == buffer.height {
		logging.Log("Buffer.Resize: No change in dimensions, returning.")
		return
	}

	logging.Log("Buffer.Resize: Getting new data buffer from pool for %dx%d", height, width)
	newData := globalBufferPool.GetBuffer(height, width)
	logging.Log("Buffer.Resize: Got new data buffer. Current old data isNil: %t", buffer.data == nil)

	// Copy existing content
	minH := min(buffer.height, height)
	minW := min(buffer.width, width)
	logging.Log("Buffer.Resize: Copying content (minH:%d, minW:%d)", minH, minW)
	for i := 0; i < minH; i++ {
		if buffer.data != nil && i < len(buffer.data) && buffer.data[i] != nil && newData[i] != nil {
			copy(newData[i][:minW], buffer.data[i][:minW])
		} else {
			// logging.Log("Buffer.Resize: Skipping copy for row %d due to nil/OOB source/dest", i) // Verbose
		}
	}

	if buffer.data != nil {
		logging.Log("Buffer.Resize: Returning old data buffer to pool")
		globalBufferPool.PutBuffer(buffer.data)
	} else {
		logging.Log("Buffer.Resize: Old data buffer was nil, not returning to pool.")
	}

	buffer.data = newData
	buffer.width = width
	buffer.height = height

	logging.Log("Buffer.Resize: Dimensions updated. Clearing dirty regions and marking full buffer dirty.")
	buffer.dirtyRegions = buffer.dirtyRegions[:0] // Clear old dirty regions
	// Inline MarkDirty logic for the full region, as we already hold regionMu.Lock
	fullRegion := DirtyRegion{StartRow: 0, EndRow: height - 1, StartCol: 0, EndCol: width - 1}
	// Simplified: after clearing, the only dirty region is the full new buffer
	buffer.dirtyRegions = append(buffer.dirtyRegions, fullRegion)
	// No need to manage pooledRegions here as we are just setting the whole buffer dirty after a resize.

	logging.Log("Buffer.Resize: Invalidating readBuffer.")
	buffer.readMu.Lock()
	if buffer.readBuffer != nil {
		buffer.readBuffer.Reset()
	}
	buffer.readMu.Unlock()
	logging.Log("Buffer.Resize: Finished.")
}

// Close releases the buffer's resources back to the pool
func (buffer *Buffer) Close() error {
	// logging.Log("Buffer.Close: Called for buffer %dx%d", buffer.width, buffer.height) // Can be verbose
	buffer.regionMu.Lock()

	if buffer.data != nil {
		// logging.Log("Buffer.Close: Returning data to globalBufferPool")
		globalBufferPool.PutBuffer(buffer.data)
		buffer.data = nil
	} else {
		// logging.Log("Buffer.Close: Data was already nil")
	}
	buffer.regionMu.Unlock()

	buffer.readMu.Lock()
	defer buffer.readMu.Unlock()
	if buffer.readBuffer != nil {
		// logging.Log("Buffer.Close: Setting readBuffer to nil")
		buffer.readBuffer = nil
	} else {
		// logging.Log("Buffer.Close: readBuffer was already nil")
	}

	return nil
}

// Read implements io.Reader for Buffer.
func (buffer *Buffer) Read(p []byte) (n int, err error) {
	buffer.readMu.Lock()
	// No defer, unlock before returning to avoid holding lock during p.Read if p is blocking

	if buffer.readBuffer == nil {
		// Initialize readBuffer if it's nil. This might happen if NewBuffer doesn't init it.
		// Or, if this is an undesirable state, log an error.
		// For now, let's assume it should be initialized here if nil.
		buffer.readBuffer = new(bytes.Buffer)
		// logging.Log("Buffer.Read: Initialized nil readBuffer instance.")
	}

	if buffer.readBuffer.Len() == 0 {
		// logging.Log("Buffer.Read: readBuffer empty, serializing content.")
		buffer.regionMu.RLock() // Lock for reading buffer.data, height, width
		if buffer.data == nil {
			buffer.regionMu.RUnlock()
			buffer.readMu.Unlock() // Unlock before returning
			// logging.Log("Buffer.Read: data is nil, returning EOF.")
			return 0, io.EOF
		}

		// Prepend ANSI screen clear and cursor home codes
		// Use actual escape characters for 
		buffer.readBuffer.WriteString("\x1b[H\x1b[2J")

		for rIdx := 0; rIdx < buffer.height; rIdx++ {
			if buffer.data[rIdx] != nil {
				for cIdx := 0; cIdx < buffer.width; cIdx++ {
					_, writeErr := buffer.readBuffer.WriteRune(buffer.data[rIdx][cIdx])
					if writeErr != nil {
						buffer.regionMu.RUnlock()
						buffer.readMu.Unlock() // Unlock before returning
						logging.Log("Buffer.Read: Error writing rune to readBuffer: %v", writeErr)
						return 0, writeErr
					}
				}
			}
			// Add CRLF for all but the last conceptual line of the buffer area if it makes sense for the terminal.
			// If the buffer represents the full screen, newlines separate rows.
			if rIdx < buffer.height-1 { // Avoid adding a newline after the very last row of the screen
				buffer.readBuffer.WriteString("\r\n")
			}
		}
		buffer.regionMu.RUnlock()
		// logging.Log("Buffer.Read: Finished serializing %d bytes to readBuffer. Content starts with: %s", buffer.readBuffer.Len(), string(buffer.readBuffer.Bytes()[:min(10, buffer.readBuffer.Len())]))
	}

	if buffer.readBuffer.Len() == 0 {
		buffer.readMu.Unlock() // Unlock before returning
		// logging.Log("Buffer.Read: readBuffer still empty after potential serialization, returning EOF")
		return 0, io.EOF
	}

	// Unlock readMu BEFORE calling readBuffer.Read(p) because p.Read could block
	// or p could be another part of the system that needs to acquire locks.
	// The readBuffer itself is a bytes.Buffer, which is internally synchronized for Read/Write.
	buffer.readMu.Unlock()
	n, err = buffer.readBuffer.Read(p)
	// logging.Log("Buffer.Read: Read %d bytes from readBuffer into p. Error: %v", n, err)

	// Special handling for io.EOF from the bytes.Buffer:
	// If we read some data (n > 0) AND the internal buffer is now empty (err == io.EOF),
	// we should return n, nil to encourage io.Copy to call Read again.
	// The next call to this Read method will then re-trigger serialization.
	if err == io.EOF && n > 0 {
		// logging.Log("Buffer.Read: Hit EOF on readBuffer but read n=%d bytes. Returning n, nil.", n)
		return n, nil
	}

	return n, err
}

// GetRunes returns a slice of runes for the specified row and column range.
// It provides read-only access to a part of the buffer's data.
func (buffer *Buffer) GetRunes(row, startCol, endCol int) []rune {
	buffer.regionMu.RLock()
	defer buffer.regionMu.RUnlock()

	if row < 0 || row >= buffer.height || startCol < 0 || endCol >= buffer.width || startCol > endCol {
		return nil // Or an empty slice, depending on desired behavior for invalid ranges
	}
	// Return a copy to prevent external modification if necessary, though for rendering, direct slice might be fine.
	// For now, returning a direct slice for performance in rendering.
	// Ensure the slice is within the bounds of data[row]
	actualEndCol := min(endCol+1, len(buffer.data[row]))
	return buffer.data[row][startCol:actualEndCol]
}

// CompareWith efficiently compares this buffer with another using SIMD
// The SIMD functions (CompareBuffers, FindDifferences) will need to be in this 'types' package
// or be imported if they remain in 'kubrick' (which would re-introduce a cycle if not careful).
// Assuming they will be moved to 'types' as well.
func (buffer *Buffer) CompareWith(other *Buffer) []DirtyRegion {
	if buffer.width != other.width || buffer.height != other.height {
		return []DirtyRegion{{0, buffer.height - 1, 0, buffer.width - 1}}
	}

	var regions []DirtyRegion
	var currentRegion *DirtyRegion

	for row := 0; row < buffer.height; row++ {
		if !CompareBuffers(buffer.data[row], other.data[row]) { // This will be types.CompareBuffers or just CompareBuffers
			diffs := FindDifferences(buffer.data[row], other.data[row]) // Same here

			for _, diff := range diffs {
				if currentRegion == nil {
					regions = append(regions, DirtyRegion{
						StartRow: row,
						EndRow:   row,
						StartCol: diff.StartIndex,
						EndCol:   diff.StartIndex + diff.Length - 1,
					})
					currentRegion = &regions[len(regions)-1]
				} else if currentRegion.EndRow == row-1 &&
					currentRegion.StartCol == diff.StartIndex &&
					currentRegion.EndCol == diff.StartIndex+diff.Length-1 {
					currentRegion.EndRow = row
				} else {
					regions = append(regions, DirtyRegion{
						StartRow: row,
						EndRow:   row,
						StartCol: diff.StartIndex,
						EndCol:   diff.StartIndex + diff.Length - 1,
					})
					currentRegion = &regions[len(regions)-1]
				}
			}
		} else {
			currentRegion = nil
		}
	}

	return regions
}
