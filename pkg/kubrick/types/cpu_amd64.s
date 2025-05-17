#include "textflag.h"

// func hasAVX2Support() bool
TEXT ·hasAVX2Support(SB), NOSPLIT, $0-1
    // Check for AVX2 support using CPUID
    MOVL $7, AX
    MOVL $0, CX
    CPUID
    // AVX2 is bit 5 in EBX
    MOVL $1, DX
    SHLL $5, DX
    ANDL BX, DX
    SETNE AL
    MOVB AL, ret+0(FP)
    RET
