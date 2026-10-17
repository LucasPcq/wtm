package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/LucasPcq/wtm/internal/domain"
)

func lockPathOf(socketPath string) string {
	return filepath.Join(filepath.Dir(socketPath), domain.DaemonLockName)
}

// acquireDaemonLock takes the lock a daemon holds until its process exits. The
// kernel releases it then, whatever the exit — which is what the socket, closed
// first, cannot say.
func acquireDaemonLock(socketPath string) (*os.File, error) {
	file, err := os.OpenFile(lockPathOf(socketPath), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open daemon lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, domain.ErrDaemonRunning
		}
		return nil, fmt.Errorf("lock daemon: %w", err)
	}
	return file, nil
}

func daemonLockHeld(socketPath string) bool {
	file, err := acquireDaemonLock(socketPath)
	if err != nil {
		return errors.Is(err, domain.ErrDaemonRunning)
	}
	file.Close()
	return false
}

// awaitDaemonGone waits while a daemon holds the lock without answering: one
// stopping its jobs, or one not listening yet. It returns as soon as either is
// over — the socket answering, or the lock free for a new daemon to take.
func awaitDaemonGone(ctx context.Context, socketPath string) error {
	return poll(ctx, pollParams{
		Timeout: domain.DaemonStopTimeout,
		Done:    func() bool { return IsDaemonRunning(socketPath) || !daemonLockHeld(socketPath) },
		Expired: fmt.Errorf("a stopping daemon did not exit within %v", domain.DaemonStopTimeout),
	})
}
