//go:build darwin

// launch_activate_socket is the only way to receive the privileged socket
// launchd created for us, and it lives in libxpc.dylib. Reaching it the way cgo
// would costs CGO_ENABLED=0 in .goreleaser.yaml, so this calls it the way the
// standard library calls libc on darwin: a dynamic import declared at link time
// plus an assembly trampoline. The symbol stays visible to otool and nm — this
// is not dlopen/dlsym symbol hiding.
//
// Adapted from github.com/bored-engineer/go-launchd (MIT, Copyright (c) 2023
// Luke Young).

package proxy

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// maxSocketFDs refuses an implausible count rather than trusting a length that
// sizes a slice.
const maxSocketFDs = 1 << 10

var libxpcLaunchActivateSocketTrampolineAddr uintptr

//go:cgo_import_dynamic libxpc_launch_activate_socket launch_activate_socket "/usr/lib/system/libxpc.dylib"

var libcFreeTrampolineAddr uintptr

//go:cgo_import_dynamic libc_free free "/usr/lib/libSystem.B.dylib"

//go:linkname syscallSyscall syscall.syscall
func syscallSyscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

// LaunchdListeners are the sockets launchd bound on our behalf under the given
// key of the job's Sockets dictionary. It is how a plain user process ends up
// serving a privileged port without ever holding a privilege.
func LaunchdListeners(name string) ([]net.Listener, error) {
	fds, err := activateSocket(name)
	if err != nil {
		return nil, err
	}

	listeners := make([]net.Listener, 0, len(fds))
	for _, fd := range fds {
		listener, listenErr := net.FileListener(os.NewFile(fd, name))
		if listenErr != nil {
			return nil, listenErr
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func activateSocket(name string) ([]uintptr, error) {
	namePtr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return nil, err
	}

	var fdsPtr *uintptr
	defer func() {
		if fdsPtr != nil {
			syscallSyscall(libcFreeTrampolineAddr, uintptr(unsafe.Pointer(fdsPtr)), 0, 0)
		}
	}()

	var count uint
	res, _, _ := syscallSyscall(
		libxpcLaunchActivateSocketTrampolineAddr,
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&fdsPtr)),
		uintptr(unsafe.Pointer(&count)),
	)
	runtime.KeepAlive(namePtr)

	if res != 0 {
		return nil, fmt.Errorf("launch_activate_socket %q: %w", name, syscall.Errno(res))
	}
	if count == 0 || count > maxSocketFDs {
		return nil, fmt.Errorf("launch_activate_socket %q returned %d sockets", name, count)
	}

	raw := (*[maxSocketFDs]int32)(unsafe.Pointer(fdsPtr))
	out := make([]uintptr, count)
	for i, fd := range raw[:count] {
		out[i] = uintptr(fd)
	}
	return out, nil
}
