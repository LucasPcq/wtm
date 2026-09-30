package process

import (
	"fmt"
	"math"
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

// maxWinsizeDimension is what a TIOCSWINSZ field holds. Past it a size does not
// fail, it wraps: 65536 columns reach the kernel as zero, and a zero-width
// window is the one thing spawnJob goes out of its way never to hand a job —
// Ink and friends fall back to plain log mode for good when they read one.
const maxWinsizeDimension = math.MaxUint16

type winsizeParams struct {
	File *os.File
	Cols int
	Rows int
}

// setWinsize is the package's only TIOCSWINSZ: a local terminal mirroring its
// own size onto the PTY it attached to, and a remote pane asking for the size
// it renders the job at, both land here. It is also where the size is
// validated, being where an int becomes a uint16: a caller that checked its own
// bounds elsewhere would leave the truncation unguarded for the next one.
//
// The ioctl goes through SyscallConn().Control, which holds a reference on the
// descriptor for its duration and refuses once the file is closed. File.Fd()
// takes no such reference: it reads a raw number that a concurrent Close may
// already have handed back to the OS — and the daemon recycles descriptors
// constantly (PTYs, log files, accepted sockets), so sizing one job through a
// stale number silently resizes whatever job now owns it.
func setWinsize(params winsizeParams) error {
	if params.Cols <= 0 || params.Rows <= 0 || params.Cols > maxWinsizeDimension || params.Rows > maxWinsizeDimension {
		return fmt.Errorf("invalid size %dx%d: each dimension must be between 1 and %d", params.Cols, params.Rows, maxWinsizeDimension)
	}

	ws := winsize{Rows: uint16(params.Rows), Cols: uint16(params.Cols)}

	conn, err := params.File.SyscallConn()
	if err != nil {
		return err
	}

	var errno syscall.Errno
	if err := conn.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(
			syscall.SYS_IOCTL,
			fd,
			syscall.TIOCSWINSZ,
			uintptr(unsafe.Pointer(&ws)),
		)
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}
