package shell

import (
	"os"
	"os/exec"
	"syscall"
)

// DetachedParams is a child [Detached] starts: Name run with Args, with Env,
// its standard error appended to the file at Log ("" for none).
type DetachedParams struct {
	Name string
	Args []string
	Env  []string
	Log  string
}

// Detached starts a child in a session of its own, away from the terminal and
// its signals, with no standard input nor output, and returns once it started.
// It never waits for the child but reaps it when it exits: a parent that lives
// as long as the child, wtm events and its daemon, would otherwise keep it as
// a zombie.
func Detached(params DetachedParams) error {
	cmd := exec.Command(params.Name, params.Args...)
	cmd.Env = params.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if params.Log != "" {
		log, err := os.OpenFile(params.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		// The child holds its own copy once started.
		defer log.Close()
		cmd.Stderr = log
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
