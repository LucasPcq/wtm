package infra

import (
	"context"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

type ReaderGoneParams struct {
	File  *os.File
	Every time.Duration
}

// ReaderGone closes once nobody reads the pipe File writes to. A pipe whose
// reader left only fails the next write, which a quiet stream may never make.
// macOS reports the hang-up only to a poll asking for POLLOUT, and answers it
// at once while the pipe is writable, so it is asked on a timer rather than
// waited on. Anything but a pipe never closes it.
func ReaderGone(ctx context.Context, params ReaderGoneParams) <-chan struct{} {
	gone := make(chan struct{})
	info, err := params.File.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return gone
	}
	go func() {
		ticker := time.NewTicker(params.Every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if hungUp(params.File) {
				close(gone)
				return
			}
		}
	}()
	return gone
}

func hungUp(f *os.File) bool {
	fds := []unix.PollFd{{Fd: int32(f.Fd()), Events: unix.POLLOUT}}
	if _, err := unix.Poll(fds, 0); err != nil {
		return false
	}
	return fds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0
}
