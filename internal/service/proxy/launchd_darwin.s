//go:build darwin

#include "textflag.h"

TEXT libxpc_launch_activate_socket_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libxpc_launch_activate_socket(SB)

GLOBL	·libxpcLaunchActivateSocketTrampolineAddr(SB), RODATA, $8
DATA	·libxpcLaunchActivateSocketTrampolineAddr(SB)/8, $libxpc_launch_activate_socket_trampoline<>(SB)

TEXT libc_free_trampoline<>(SB),NOSPLIT,$0-0
	JMP	libc_free(SB)

GLOBL	·libcFreeTrampolineAddr(SB), RODATA, $8
DATA	·libcFreeTrampolineAddr(SB)/8, $libc_free_trampoline<>(SB)
