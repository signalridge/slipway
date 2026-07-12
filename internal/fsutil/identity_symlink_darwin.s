#include "textflag.h"

TEXT libc_freadlink_trampoline<>(SB),NOSPLIT,$0-0
	JMP libc_freadlink(SB)
GLOBL ·libcFreadlinkTrampolineAddr(SB), RODATA, $8
DATA ·libcFreadlinkTrampolineAddr(SB)/8, $libc_freadlink_trampoline<>(SB)
