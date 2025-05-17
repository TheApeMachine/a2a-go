#include "textflag.h"

// func compareBuffersNEON(a, b []rune) bool
// Args:
//   a: R0=base, R1=len
//   b: R2=base, R3=len (Go compiler ensures len(a)==len(b) before calling)
// Ret: R0 (1 for true, 0 for false)
TEXT ·compareBuffersNEON(SB), NOSPLIT, $0-49
    MOVD a_base+0(FP), R0    // a.Data
    MOVD a_len+8(FP), R1     // a.Len
    MOVD b_base+24(FP), R2   // b.Data
    MOVD b_len+32(FP), R3    // b.Len

    // Handle zero length case
    CMP $0, R1
    BEQ equal

    // Calculate number of 32-byte (8 runes) blocks
    MOVD R1, R4              // Copy length
    LSR  $3, R4              // R4 = len / 8 (number of 8-rune blocks)
    CBZ  R4, small_blocks    // If no full blocks, try 4-rune blocks

    // Loop over 32-byte blocks
loop:
    // Load 8 runes (32 bytes) from each buffer
    VLD1.P 32(R0), [V0.S4, V1.S4]   // Load 8 runes from a
    VLD1.P 32(R2), [V2.S4, V3.S4]   // Load 8 runes from b

    // Compare both vectors
    WORD $0x6E231C00         // CMEQ V0.4S, V0.4S, V2.4S
    WORD $0x6E231C21         // CMEQ V1.4S, V1.4S, V3.4S

    // Combine results with AND
    WORD $0x4E201C20         // AND V0.16B, V0.16B, V1.16B

    // Find minimum value across lanes
    WORD $0x6E21A810         // UMINV S0, V0.4S
    WORD $0x0E213C08         // MOV W8, V0.S[0]
    
    // If minimum is 0, vectors had a difference
    CMP $0, R8
    BEQ not_equal

    // Decrement block counter and loop
    SUBS $1, R4
    BNE  loop

small_blocks:
    // Handle remaining blocks of 4 runes
    AND $7, R1, R4           // R4 = remaining runes
    LSR $2, R4, R4           // R4 = (remaining runes) / 4
    CBZ R4, remainder

small_loop:
    VLD1.P 16(R0), [V0.S4]   // Load 4 runes from a
    VLD1.P 16(R2), [V1.S4]   // Load 4 runes from b

    // Compare vectors
    WORD $0x6E231C00         // CMEQ V0.4S, V0.4S, V1.4S

    // Find minimum value across lanes
    WORD $0x6E21A810         // UMINV S0, V0.4S
    WORD $0x0E213C08         // MOV W8, V0.S[0]
    
    // If minimum is 0, vectors had a difference
    CMP $0, R8
    BEQ not_equal

    SUBS $1, R4
    BNE small_loop

remainder:
    // Handle remaining runes (0 to 3)
    AND $3, R1               // R1 = original_len % 4
    CBZ R1, equal           // If no remainder, buffers are equal

remainder_loop:
    // Load and compare one rune at a time
    MOVWU.P 4(R0), R6       // Load rune from a, increment pointer
    MOVWU.P 4(R2), R7       // Load rune from b, increment pointer
    CMP R6, R7
    BNE not_equal
    
    SUBS $1, R1             // Decrement counter
    BNE remainder_loop

equal:
    MOVD $1, R0             // Return true
    MOVB R0, ret+48(FP)
    RET

not_equal:
    MOVD $0, R0             // Return false
    MOVB R0, ret+48(FP)
    RET

// func clearBufferNEON(buf []rune, value rune)
// Args:
//   buf: R0=base, R1=len
//   value: R2 (32-bit rune value)
TEXT ·clearBufferNEON(SB), NOSPLIT, $0-32
    MOVD buf_base+0(FP), R0   // buf.Data
    MOVD buf_len+8(FP), R1    // buf.Len
    MOVW value+24(FP), R2     // rune value (32-bit)

    // Handle zero length case
    CMP $0, R1
    BEQ clear_done

    // Duplicate the 32-bit rune value across a 128-bit vector register
    WORD $0x4E040040  // DUP V0.4S, W2  (replacing VDUP)

    // Calculate number of 16-byte (4 runes) blocks
    MOVD R1, R3             // Copy length
    LSR $2, R3, R3          // R3 = len / 4 (number of 4-rune blocks)
    CMP $0, R3
    BEQ clear_remainder     // If no full blocks, go straight to remainder

    // Loop over 16-byte blocks
clear_loop:
    // Store the vector register (4 runes)
    VST1 [V0.B16], (R0)      // Store V0 to buf base address R0
    ADD $16, R0             // Advance pointer R0           << CORRECTION

    // Decrement block counter and loop
    SUB $1, R3, R3
    CMP $0, R3
    BNE clear_loop

clear_remainder:
    // Handle remaining runes (0 to 3)
    AND $3, R1, R1          // R1 = original_len % 4 (number of remaining runes)
    CMP $0, R1
    BEQ clear_done          // If no remainder, done

clear_remainder_loop:
    // Store 1 rune (4 bytes)
    MOVW R2, (R0)           // Store 32-bit value from W2 (implicitly R2) << CORRECTION (MOVW or STR W?)
    ADD $4, R0              // Advance pointer R0                       << CORRECTION

    // Decrement remainder counter and loop
    SUB $1, R1, R1
    CMP $0, R1
    BNE clear_remainder_loop

clear_done:
    RET

// func copyBufferNEON(dst, src []rune)
// Args:
//   dst: R0=base, R1=len
//   src: R2=base, R3=len (Go wrapper ensures len(dst) >= len(src))
TEXT ·copyBufferNEON(SB), NOSPLIT, $0-40
    MOVD dst_base+0(FP), R0   // dst.Data
    // R1 = dst.Len (unused, assuming sufficient space)
    MOVD src_base+24(FP), R2  // src.Data
    MOVD src_len+32(FP), R3   // src.Len (the amount to copy)

    // Handle zero length case
    CMP $0, R3
    BEQ copy_done

    // Calculate number of 16-byte (4 runes) blocks
    MOVD R3, R4             // Copy length
    LSR $2, R4, R4          // R4 = len / 4 (number of 4-rune blocks)
    CMP $0, R4
    BEQ copy_remainder      // If no full blocks, go straight to remainder

    // Loop over 16-byte blocks
copy_loop:
    // Load 16 bytes (4 runes) from src
    VLD1 (R2), [V0.B16]      // Load from src base R2
    ADD $16, R2             // Advance pointer R2    << CORRECTION
    // Store 16 bytes (4 runes) to dst
    VST1 [V0.B16], (R0)      // Store to dst base R0
    ADD $16, R0             // Advance pointer R0   << CORRECTION

    // Decrement block counter and loop
    SUB $1, R4, R4
    CMP $0, R4
    BNE copy_loop

copy_remainder:
    // Handle remaining runes (0 to 3)
    AND $3, R3, R3          // R3 = original_len % 4 (number of remaining runes)
    CMP $0, R3
    BEQ copy_done           // If no remainder, done

copy_remainder_loop:
    // Load 1 rune (4 bytes) from src
    MOVWU (R2), R4          // Load 32-bit rune from src << CORRECTION
    ADD $4, R2              // Advance pointer R2        << CORRECTION
    // Store 1 rune (4 bytes) to dst
    MOVW R4, (R0)           // Store 32-bit rune to dst << CORRECTION
    ADD $4, R0              // Advance pointer R0       << CORRECTION

    // Decrement remainder counter and loop
    SUB $1, R3, R3
    CMP $0, R3
    BNE copy_remainder_loop

copy_done:
    RET


// func findFirstDifferenceNEON(old, new []rune) int
// Args:
//   old: R0=base, R1=len
//   new: R2=base, R3=len
// Ret: R0 (index of first difference, or -1 if identical)
TEXT ·findFirstDifferenceNEON(SB), NOSPLIT, $0-57 // size + 1 for ret int
    MOVD old_base+0(FP), R0   // old.Data
    MOVD old_len+8(FP), R1    // old.Len
    MOVD new_base+24(FP), R2  // new.Data
    MOVD new_len+32(FP), R3   // new.Len

    // Determine minimum length
    CMP R1, R3
    MOVD R1, R4              // R4 = min_len, assume R1 is smaller
    BGT min_r3              // If R1 > R3, use R3
    B min_done
min_r3:
    MOVD R3, R4             // R4 = R3 (the smaller value)
min_done:
    // Initialize index
    MOVD $0, R5             // R5 = current index

    // Handle zero length case
    CMP $0, R4
    BEQ check_lengths       // If min_len is 0, just check if original lengths differ

    // Calculate number of 16-byte (4 runes) blocks in min_len
    MOVD R4, R6             // Copy min_len
    LSR $2, R6, R6          // R6 = min_len / 4 (number of 4-rune blocks)
    CMP $0, R6
    BEQ find_remainder      // If no full blocks, go straight to remainder

    // Loop over 16-byte blocks
find_loop:
    // Load 16 bytes (4 runes) from each buffer (without advancing yet)
    VLD1 (R0), [V0.B16]      // Load from old base R0
    VLD1 (R2), [V1.B16]      // Load from new base R2

    // Compare the loaded vectors
    WORD $0x4E21D400  // FCMEQ V0.4S, V0.4S, V1.4S

    // Check if any difference was found in this block
    WORD $0x4E31B800  // ADDV B0, V0.16B
    WORD $0x0E213C20  // MOV W0, V1.S[0]
    CMP $0xFF, R0     // Compare with 0xFF
    BNE diff_in_block // If min byte is not 0xFF, a difference exists

    // No difference in this block, advance pointers and index
    ADD $16, R0             // Advance old pointer << CORRECTION
    ADD $16, R2             // Advance new pointer << CORRECTION
    ADD $4, R5, R5          // Advance index by 4 runes

    // Decrement block counter and loop
    SUB $1, R6, R6
    CMP $0, R6
    BNE find_loop
    B find_remainder        // Finished blocks, check remainder

diff_in_block:
    // Difference found within V0/V1. Find the first differing rune.
    // Extract the 4 comparison results (0xFFFFFFFF if equal, 0 if different)
    WORD $0x4E083C07  // MOV X7, V0.D[0]
    WORD $0x4E183C08  // MOV X8, V0.D[1]
    WORD $0x4E283C09  // MOV X9, V0.D[2]
    WORD $0x4E383C0A  // MOV X10, V0.D[3]

    // Check lanes sequentially
    CMP $0, R7             // Check rune at index R5 + 0
    BEQ diff_found          // If 0, it's different
    ADD $1, R5, R5          // Increment index
    CMP $0, R8             // Check rune at index R5 + 1
    BEQ diff_found          // If 0, it's different
    ADD $1, R5, R5          // Increment index
    CMP $0, R9             // Check rune at index R5 + 2
    BEQ diff_found          // If 0, it's different
    ADD $1, R5, R5          // Increment index
    // CMP W10, $0 must be true if we got here and diff_in_block was entered
    B diff_found

find_remainder:
    // Handle remaining runes (0 to 3) based on min_len
    AND $3, R4, R4          // R4 = min_len % 4 (number of remaining runes)
    CMP $0, R4
    BEQ check_lengths       // If no remainder, check original lengths

find_remainder_loop:
    // Load 1 rune (4 bytes) from each
    MOVWU (R0), R7          // Load 32-bit rune from old << CORRECTION
    ADD $4, R0              // Advance pointer R0       << CORRECTION
    MOVWU (R2), R8          // Load 32-bit rune from new << CORRECTION
    ADD $4, R2              // Advance pointer R2       << CORRECTION
    CMP R7, R8
    BNE diff_found          // If different, return current index R5

    // No difference, advance index
    ADD $1, R5, R5

    // Decrement remainder counter and loop
    SUB $1, R4, R4
    CMP $0, R4
    BNE find_remainder_loop

check_lengths:
    // Buffers matched up to min_len. Check if original lengths differ.
    CMP R1, R3              // Compare old_len and new_len
    BEQ no_diff             // If equal, buffers are identical

    // Lengths differ, the first difference is at min_len (which is current R5)
    // R5 already holds min_len because we incremented it fully
    B diff_found

no_diff:
    MOVD $-1, R5            // Return -1 (identical)

diff_found:
    MOVD R5, R0             // Move result index to return register R0
    MOVD R0, ret+48(FP)     // Store return value
    RET

// func countRunsNEON(data []rune) int
TEXT ·countRunsNEON(SB), NOSPLIT, $0-24
    MOVD data_base+0(FP), R0    // data pointer
    MOVD data_len+8(FP), R1     // length
    
    // Initialize count
    MOVD $0, R2                 // R2 = count
    
    // Handle zero length case
    CMP $0, R1
    BEQ done
    
    // Initialize previous rune
    MOVW (R0), R3              // Load first rune
    ADD $4, R0                 // Advance pointer
    SUB $1, R1                 // Decrease length
    MOVD $1, R2               // Initialize count to 1
    
loop:
    CMP $0, R1
    BEQ done
    
    // Load current rune
    MOVW (R0), R4
    ADD $4, R0
    SUB $1, R1
    
    // Compare with previous
    CMP R3, R4
    BEQ same_run
    
    // Different rune, increment count
    ADD $1, R2
    MOVD R4, R3               // Update previous
    
same_run:
    B loop
    
done:
    MOVD R2, ret+16(FP)       // Return count
    RET

// func expandRunsNEON(dst []rune, values []rune, counts []int32) int
TEXT ·expandRunsNEON(SB), NOSPLIT, $0-72
    MOVD dst_base+0(FP), R0        // destination pointer
    MOVD dst_len+8(FP), R1         // destination length
    MOVD values_base+24(FP), R2    // values pointer
    MOVD values_len+32(FP), R3     // values length
    MOVD counts_base+48(FP), R4    // counts pointer
    
    MOVD $0, R5                    // source index
    MOVD $0, R6                    // destination index
    
    // Calculate total length needed
    MOVD $0, R7                    // total length
calc_total:
    CMP R3, R5                     // Check if we're done with source
    BGE check_total
    
    // Load count and add to total
    MOVW (R4)(R5<<2), R8          // Load count as 32-bit
    SXTW R8, R8                    // Sign extend to 64-bit
    CMP $0, R8                     // Skip if count <= 0
    BLE next_total
    ADD R8, R7                     // Add to total
next_total:
    ADD $1, R5                     // Next source index
    B calc_total
    
check_total:
    // Check if we have enough space
    CMP R1, R7                     // Compare with destination length
    BGT expand_fail                // Return -1 if not enough space
    
    // Reset source index for expansion
    MOVD $0, R5                    // source index
    MOVD $0, R6                    // destination index
    
expand_loop:
    CMP R3, R5                     // Check if we're done with source
    BGE expand_done
    
    // Load current rune and count
    MOVWU (R2)(R5<<2), R7         // Load rune from values
    MOVW (R4)(R5<<2), R8          // Load count as 32-bit
    SXTW R8, R8                    // Sign extend to 64-bit
    
    // Check if count is positive
    CMP $0, R8
    BLE next_rune
    
    // Store count copies of the rune
copy_loop:
    MOVW R7, (R0)(R6<<2)          // Store rune
    ADD $1, R6                     // Increment destination index
    SUBS $1, R8                    // Decrement count
    BNE copy_loop
    
next_rune:
    ADD $1, R5                     // Next source rune
    B expand_loop
    
expand_done:
    MOVD R6, R0                    // Move result to return register
    MOVD R0, ret+64(FP)           // Return number of runes written
    RET
    
expand_fail:
    MOVD $-1, R0                   // Load -1 into register
    MOVD R0, ret+64(FP)           // Return -1 to indicate failure
    RET

// func findDifferencesNEON(a, b []rune) []DiffResult
TEXT ·findDifferencesNEON(SB), NOSPLIT, $0-72
    MOVD a_base+0(FP), R0         // a pointer
    MOVD a_len+8(FP), R1         // a length
    MOVD b_base+24(FP), R2       // b pointer
    MOVD b_len+32(FP), R3        // b length
    
    // Get minimum length
    CMP R1, R3
    CSEL LT, R1, R3, R4         // R4 = min(a_len, b_len)
    
    // Initialize result slice
    MOVD $0, ret+48(FP)         // ptr = nil
    MOVD $0, ret+56(FP)         // len = 0
    MOVD $0, ret+64(FP)         // cap = 0
    
    // Fall back to Go implementation for now
    RET

// func findPatternNEON(haystack, needle []rune) int
TEXT ·findPatternNEON(SB), NOSPLIT, $0-56
    MOVD haystack_base+0(FP), R0    // haystack pointer
    MOVD haystack_len+8(FP), R1     // haystack length
    MOVD needle_base+24(FP), R2     // needle pointer
    MOVD needle_len+32(FP), R3      // needle length
    
    // Handle edge cases
    CMP $0, R3                      // Empty needle
    BEQ not_found
    CMP R3, R1                      // Needle longer than haystack
    BGT not_found
    
    // Initialize index
    MOVD $0, R4                     // current index
    SUB R3, R1, R5                  // R5 = max start position (haystack_len - needle_len)
    ADD $1, R5                      // Include last position

    // Load first rune of needle for quick check
    MOVWU (R2), R6                  // R6 = first rune of needle

search_loop:
    CMP R5, R4                      // Check if we're done
    BGE not_found

    // Quick check first rune
    MOVWU (R0), R7                  // Load current rune from haystack
    CMP R6, R7                      // Compare with first needle rune
    BNE next_pos

    // First rune matches, check rest of needle
    MOVD R0, R8                     // temp haystack pointer
    MOVD R2, R9                     // temp needle pointer
    MOVD R3, R10                    // temp length
    
    // Use NEON for blocks of 4 runes
    LSR $2, R10, R11               // R11 = needle_len / 4
    CBZ R11, check_remainder

vector_loop:
    VLD1 (R8), [V0.S4]             // Load 4 runes from haystack
    VLD1 (R9), [V1.S4]             // Load 4 runes from needle
    WORD $0x6E201C00               // CMEQ V0.4S, V0.4S, V1.4S
    
    // Check if all elements were equal
    WORD $0x4E31B800               // ADDV B0, V0.16B
    WORD $0x0E213C20               // MOV W0, V1.S[0]
    CMP $0xFF, R0
    BNE next_pos                    // Mismatch found, try next position
    
    ADD $16, R8                     // Next 4 runes in haystack
    ADD $16, R9                     // Next 4 runes in needle
    SUBS $1, R11                    // Decrement block counter
    BNE vector_loop

check_remainder:
    AND $3, R3, R10                // R10 = remaining runes (0-3)
    CBZ R10, found                 // If no remainder, we found a match

remainder_loop:
    MOVWU (R8), R7                 // Load rune from haystack
    MOVWU (R9), R6                 // Load rune from needle
    CMP R6, R7
    BNE next_pos                   // Mismatch found, try next position
    
    ADD $4, R8                     // Next rune in haystack
    ADD $4, R9                     // Next rune in needle
    SUBS $1, R10                   // Decrement counter
    BNE remainder_loop
    
found:
    MOVD R4, ret+48(FP)            // Return current index
    RET
    
next_pos:
    ADD $4, R0                      // Move to next rune in haystack
    ADD $1, R4                      // Increment index
    B search_loop
    
not_found:
    MOVD $-1, R0
    MOVD R0, ret+48(FP)            // Return -1
    RET
