package types

import (
	"sync"

	"github.com/theapemachine/a2a-go/pkg/logging"
)

const (
	smallBufferSize  = 80 * 5    // 5 lines at 80 chars each
	mediumBufferSize = 80 * 24   // 24 lines at 80 chars each
	largeBufferSize  = 200 * 100 // 100 lines at 200 chars each
)

// BufferPool manages memory pools for different buffer sizes
type BufferPool struct {
	small  sync.Pool
	medium sync.Pool
	large  sync.Pool
}

// NewBufferPool creates a new buffer pool
func NewBufferPool() *BufferPool {
	logging.Log("BufferPool.NewBufferPool: Called")
	return &BufferPool{
		small: sync.Pool{
			New: func() interface{} {
				buf := make([][]rune, 5)
				return &buf
			},
		},
		medium: sync.Pool{
			New: func() interface{} {
				buf := make([][]rune, 24)
				return &buf
			},
		},
		large: sync.Pool{
			New: func() interface{} {
				buf := make([][]rune, 100)
				return &buf
			},
		},
	}
}

// GetBuffer gets a buffer of appropriate size
func (p *BufferPool) GetBuffer(rows, cols int) [][]rune {
	size := rows * cols
	logging.Log("BufferPool.GetBuffer: Requesting buffer for %dx%d (size: %d)", rows, cols, size)
	var buf *[][]rune
	var poolName string // For logging

	switch {
	case size <= smallBufferSize:
		buf = p.small.Get().(*[][]rune)
		poolName = "small"
	case size <= mediumBufferSize:
		buf = p.medium.Get().(*[][]rune)
		poolName = "medium"
	default:
		buf = p.large.Get().(*[][]rune)
		poolName = "large"
	}

	logging.Log("BufferPool.GetBuffer: Got buffer from %s pool (initial len %d). Ensuring dimensions.", poolName, len(*buf))

	// Ensure buffer has correct dimensions
	if len(*buf) < rows {
		logging.Log("BufferPool.GetBuffer: Resizing rows from %d to %d", len(*buf), rows)
		newBuf := make([][]rune, rows)
		copy(newBuf, *buf)
		*buf = newBuf
	}
	for i := range *buf {
		if (*buf)[i] == nil || len((*buf)[i]) < cols { // Also check for nil sub-slice
			(*buf)[i] = make([]rune, cols) // Allocate fresh column slice
		}
	}

	if len(*buf) > 0 && (*buf)[0] != nil {
		logging.Log("BufferPool.GetBuffer: Returning buffer with actual dimensions %dx%d", len(*buf), len((*buf)[0]))
	} else if len(*buf) > 0 {
		logging.Log("BufferPool.GetBuffer: Returning buffer with rows %d but first col slice is nil", len(*buf))
	} else {
		logging.Log("BufferPool.GetBuffer: Returning empty buffer (0 rows)")
	}
	return *buf
}

// PutBuffer returns a buffer to the pool
func (p *BufferPool) PutBuffer(buf [][]rune) {
	if buf == nil {
		logging.Log("BufferPool.PutBuffer: Attempted to put nil buffer, ignoring.")
		return
	}
	if len(buf) == 0 {
		logging.Log("BufferPool.PutBuffer: Attempted to put buffer with 0 rows, ignoring.")
		return
	}
	if buf[0] == nil {
		logging.Log("BufferPool.PutBuffer: First row of buffer is nil (rows: %d), cannot determine size for pool. Ignoring.", len(buf))
		return
	}

	size := len(buf) * len(buf[0])
	logging.Log("BufferPool.PutBuffer: Putting buffer of size %dx%d (total: %d) back to pool", len(buf), len(buf[0]), size)

	// Clear buffer before returning to pool
	for i := range buf {
		if buf[i] != nil { // Check if row itself is nil
			for j := range buf[i] {
				buf[i][j] = ' '
			}
		}
	}

	bufPtr := &buf
	var poolName string
	switch {
	case size <= smallBufferSize:
		p.small.Put(bufPtr)
		poolName = "small"
	case size <= mediumBufferSize:
		p.medium.Put(bufPtr)
		poolName = "medium"
	default:
		p.large.Put(bufPtr)
		poolName = "large"
	}
	logging.Log("BufferPool.PutBuffer: Buffer placed in %s pool.", poolName)
}

// Global buffer pool instance - this needs to be accessible to types.Buffer
// It will be types.globalBufferPool
var globalBufferPool = NewBufferPool()
