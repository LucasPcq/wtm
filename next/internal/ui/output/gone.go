package output

import (
	"context"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// goneEvery is how often [Stream.Gone] asks whether the reader left.
const goneEvery = 250 * time.Millisecond

// Gone returns a channel closed once nobody reads s any more: the reader of the
// pipe or of the socket s writes to left. That only fails the next write
// otherwise, which a quiet stream may never make. A stream over anything else
// never closes it; it stops asking when ctx is done.
func (s *Stream) Gone(ctx context.Context) <-chan struct{} {
	gone := make(chan struct{})
	file, ok := s.Writer.(*os.File)
	if !ok {
		return gone
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&(os.ModeNamedPipe|os.ModeSocket) == 0 {
		return gone
	}

	fd := int32(file.Fd())
	go func() {
		ticker := time.NewTicker(goneEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if hungUp(fd) {
				close(gone)
				return
			}
		}
	}()
	return gone
}

// hungUp reports whether the reader of the file fd writes to left. macOS
// reports it only to a poll asking for POLLOUT, which it answers at once while
// the file is writable: the file is asked on a timer rather than waited on.
func hungUp(fd int32) bool {
	fds := []unix.PollFd{{Fd: fd, Events: unix.POLLOUT}}
	if _, err := unix.Poll(fds, 0); err != nil {
		return false
	}
	return fds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0
}
