package shell

import (
	"os/exec"
	"syscall"
)

// Detached starts name with args in a session of its own, away from the
// terminal and its signals, with env and no standard stream, and returns once
// it started. It never waits for the child but reaps it when it exits: a parent
// that lives as long as the child, wtm events and its daemon, would otherwise
// keep it as a zombie.
func Detached(name string, args, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() { _ = cmd.Wait() }()
	return nil
}
