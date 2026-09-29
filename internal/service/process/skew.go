package process

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// terminate is how a daemon too old to take a shutdown request is asked to exit.
// A variable so a test never signals its own process.
var terminate = func(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }

// DaemonPeerPID reads the PID of the process holding the socket from the kernel,
// for a daemon too old to stamp its own on its answers.
func DaemonPeerPID(socketPath string) (int, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return 0, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("connect to daemon: not a unix socket")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var pid int
	var pidErr error
	if err := raw.Control(func(fd uintptr) { pid, pidErr = peerPIDOf(fd) }); err != nil {
		return 0, err
	}
	return pid, pidErr
}

// Shutdown asks the daemon to exit and waits until its socket stops answering.
// A daemon predating the shutdown request refuses it as an unknown action; it
// is then sent the signal its own shutdown path was written for.
func Shutdown(socketPath string) error {
	resp, err := NewClient(socketPath).SendUnchecked(Request{Action: ActionShutdown})
	if err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	if resp.Status == StatusError && !strings.Contains(resp.Message, domain.DaemonUnknownActionPrefix) {
		return fmt.Errorf("stop daemon: %s", resp.Message)
	}
	if resp.Status == StatusError {
		if err := terminateOlder(socketPath); err != nil {
			return err
		}
	}
	return AwaitDaemonStopped(socketPath)
}

func terminateOlder(socketPath string) error {
	pid, err := DaemonPeerPID(socketPath)
	if err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	if pid <= 1 {
		return fmt.Errorf("stop daemon: no pid for the process holding %s", socketPath)
	}
	if err := terminate(pid); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop daemon (pid %d): %w", pid, err)
	}
	return nil
}

// EnsureCurrentDaemon is EnsureDaemon for a command about to start something: a
// daemon of another build runs the jobs its own way, so one holding nothing is
// replaced on the spot, and one holding jobs is refused by name — replacing it
// would stop a foreground service the user is working in.
func EnsureCurrentDaemon(params DaemonParams) error {
	if err := EnsureDaemon(params); err != nil {
		return err
	}
	resp, err := NewClient(params.SocketPath).SendUnchecked(Request{Action: ActionList})
	if err != nil {
		return err
	}
	if resp.Version == domain.Version {
		return nil
	}

	held := 0
	for _, job := range resp.Jobs {
		if rules.IsJobUp(job.Status) {
			held++
		}
	}
	if held > 0 {
		return fmt.Errorf("%w: %s", domain.ErrDaemonVersionMismatch, fmt.Sprintf(domain.DaemonSkewHoldsJobsFmt, rules.DaemonVersionLabel(resp.Version), held, domain.Version))
	}
	if err := Shutdown(params.SocketPath); err != nil {
		return err
	}
	return EnsureDaemon(params)
}
