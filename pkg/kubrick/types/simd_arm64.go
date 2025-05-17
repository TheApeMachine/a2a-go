//go:build arm64 && !nosimd
// +build arm64,!nosimd

package types

// CompareBuffers compares two rune buffers using ARM NEON instructions.
// Returns true if the buffers are identical.
func CompareBuffers(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	return compareBuffersNEON(a, b)
}

//go:noescape
func compareBuffersNEON(a, b []rune) bool

// ClearBuffer sets all runes in the buffer to the given value using ARM NEON.
func ClearBuffer(buf []rune, value rune) {
	clearBufferNEON(buf, value)
}

//go:noescape
func clearBufferNEON(buf []rune, value rune)

// CopyBuffer copies src to dst using ARM NEON instructions.
func CopyBuffer(dst, src []rune) {
	if len(dst) < len(src) {
		panic("destination buffer too small")
	}
	if len(src) == 0 {
		return
	}
	copyBufferNEON(dst, src)
}

//go:noescape
func copyBufferNEON(dst, src []rune)

// FindPattern searches for a pattern in the buffer using ARM NEON instructions.
// Returns the index of the first occurrence or -1 if not found.
func FindPattern(buf []rune, pattern []rune) int {
	if len(pattern) > len(buf) {
		return -1
	}
	return findPatternNEON(buf, pattern)
}

//go:noescape
func findPatternNEON(buf []rune, pattern []rune) int

// DiffResult represents a difference between two buffers
type DiffResult struct {
	StartIndex int    // Start index of the difference
	Length     int    // Length of the different region
	OldRunes   []rune // Original runes in the region
	NewRunes   []rune // New runes in the region
}

// FindDifferences finds all differences between two buffers using ARM NEON.
func FindDifferences(old, new []rune) []DiffResult {
	return findDifferencesNEON(old, new)
}

//go:noescape
func findDifferencesNEON(old, new []rune) []DiffResult

// FindFirstDifference finds the first difference between two buffers using ARM NEON.
// Returns the index of the first difference, or -1 if buffers are identical.
func FindFirstDifference(old, new []rune) int {
	return findFirstDifferenceNEON(old, new)
}

//go:noescape
func findFirstDifferenceNEON(old, new []rune) int

// CountRuns counts consecutive identical runes using ARM NEON.
// Returns a slice of run lengths.
func CountRuns(buf []rune) []int32 {
	if len(buf) == 0 {
		return nil
	}
	// Pre-allocate result slice with maximum possible size
	// Each run must be at least 1 rune long, so len(buf) is the maximum number of runs
	result := make([]int32, len(buf))
	n := countRunsNEON(buf, result)
	if n <= 0 || n > int32(len(buf)) {
		// Invalid count returned, fall back to generic implementation
		var runs []int32
		currentRun := int32(1)
		currentValue := buf[0]

		for i := 1; i < len(buf); i++ {
			if buf[i] == currentValue {
				currentRun++
			} else {
				runs = append(runs, currentRun)
				currentRun = 1
				currentValue = buf[i]
			}
		}
		runs = append(runs, currentRun)
		return runs
	}
	return result[:n]
}

//go:noescape
func countRunsNEON(buf []rune, result []int32) int32

// ExpandRuns expands run-length encoded data using ARM NEON.
// Takes pairs of (rune, count) and expands them into a buffer.
// Returns the total number of runes written.
func ExpandRuns(dst []rune, values []rune, counts []int32) int {
	if len(values) != len(counts) {
		panic("values and counts slices must have the same length")
	}

	// Calculate total length needed
	totalLen := int32(0)
	for _, count := range counts {
		if count <= 0 {
			continue
		}
		totalLen += count
	}

	if len(dst) < int(totalLen) {
		panic("destination buffer too small")
	}

	if len(values) == 0 {
		return 0
	}

	n := expandRunsNEON(dst, values, counts)
	if n < 0 {
		// Fall back to generic implementation if NEON version fails
		pos := 0
		for i := range values {
			for j := int32(0); j < counts[i] && pos < len(dst); j++ {
				dst[pos] = values[i]
				pos++
			}
		}
		return pos
	}
	return n
}

//go:noescape
func expandRunsNEON(dst []rune, values []rune, counts []int32) int
