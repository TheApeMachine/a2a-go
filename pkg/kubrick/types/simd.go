//go:build amd64 && !nosimd
// +build amd64,!nosimd

package types

import (
	_ "unsafe" // Required for //go:linkname
)

// Implemented in assembly
//
//go:noescape
func hasAVX2Support() bool

var hasAVX2 = hasAVX2Support()

// CompareBuffers compares two rune buffers using SIMD instructions if available.
// Returns true if the buffers are identical.
//
//go:nosplit
//go:noescape
func CompareBuffers(a, b []rune) bool {
	if !hasAVX2 {
		// Fall back to generic implementation
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	return compareBuffersAVX2(a, b)
}

//go:noescape
func compareBuffersAVX2(a, b []rune) bool

// ClearBuffer sets all runes in the buffer to the given value using SIMD.
//
//go:nosplit
//go:noescape
func ClearBuffer(buf []rune, value rune) {
	if !hasAVX2 {
		// Fall back to generic implementation
		for i := range buf {
			buf[i] = value
		}
		return
	}
	clearBufferAVX2(buf, value)
}

//go:noescape
func clearBufferAVX2(buf []rune, value rune)

// CopyBuffer copies src to dst using SIMD instructions.
//
//go:nosplit
//go:noescape
func CopyBuffer(dst, src []rune) {
	if !hasAVX2 {
		// Fall back to generic implementation
		copy(dst, src)
		return
	}
	copyBufferAVX2(dst, src)
}

//go:noescape
func copyBufferAVX2(dst, src []rune)

// FindPattern searches for a pattern in the buffer using SIMD instructions.
// Returns the index of the first occurrence or -1 if not found.
//
//go:nosplit
//go:noescape
func FindPattern(buf []rune, pattern []rune) int {
	if !hasAVX2 {
		// Fall back to generic implementation
		if len(pattern) > len(buf) {
			return -1
		}
		for i := 0; i <= len(buf)-len(pattern); i++ {
			match := true
			for j := range pattern {
				if buf[i+j] != pattern[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
		return -1
	}
	return findPatternAVX2(buf, pattern)
}

//go:noescape
func findPatternAVX2(buf []rune, pattern []rune) int

// CountRuns counts consecutive identical runes using SIMD.
// Returns a slice of run lengths.
//
//go:nosplit
//go:noescape
func CountRuns(buf []rune) []int32

// ExpandRuns expands run-length encoded data using SIMD.
// Takes pairs of (rune, count) and expands them into a buffer.
//
//go:nosplit
//go:noescape
func ExpandRuns(dst []rune, values []rune, counts []int32) int

// DiffResult represents a difference between two buffers
type DiffResult struct {
	StartIndex int    // Start index of the difference
	Length     int    // Length of the different region
	OldRunes   []rune // Original runes in the region
	NewRunes   []rune // New runes in the region
}

// FindDifferences finds all differences between two buffers using SIMD instructions.
// Returns a slice of DiffResult containing the differences.
//
//go:nosplit
//go:noescape
func FindDifferences(old, new []rune) []DiffResult {
	if !hasAVX2 {
		// Fall back to generic implementation
		var diffs []DiffResult
		inDiff := false
		diffStart := 0

		minLen := min(len(old), len(new))

		for i := range minLen {
			if old[i] != new[i] {
				if !inDiff {
					inDiff = true
					diffStart = i
				}
			} else if inDiff {
				diffs = append(diffs, DiffResult{
					StartIndex: diffStart,
					Length:     i - diffStart,
					OldRunes:   old[diffStart:i],
					NewRunes:   new[diffStart:i],
				})
				inDiff = false
			}
		}

		if inDiff {
			diffs = append(diffs, DiffResult{
				StartIndex: diffStart,
				Length:     minLen - diffStart,
				OldRunes:   old[diffStart:minLen],
				NewRunes:   new[diffStart:minLen],
			})
		}

		if len(old) != len(new) {
			diffs = append(diffs, DiffResult{
				StartIndex: minLen,
				Length:     abs(len(old) - len(new)),
				OldRunes:   old[minLen:],
				NewRunes:   new[minLen:],
			})
		}

		return diffs
	}
	return findDifferencesAVX2(old, new)
}

//go:noescape
func findDifferencesAVX2(old, new []rune) []DiffResult

// FindFirstDifference finds the first difference between two buffers using SIMD.
// Returns the index of the first difference, or -1 if buffers are identical.
//
//go:nosplit
//go:noescape
func FindFirstDifference(old, new []rune) int {
	if !hasAVX2 {
		// Fall back to generic implementation
		minLen := min(len(old), len(new))

		for i := range old {
			if old[i] != new[i] {
				return i
			}
		}

		if len(old) != len(new) {
			return minLen
		}

		return -1
	}
	return findFirstDifferenceAVX2(old, new)
}

//go:noescape
func findFirstDifferenceAVX2(old, new []rune) int

// abs returns the absolute value of x
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
