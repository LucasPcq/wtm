package run

import (
	"maps"
	"path/filepath"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A Spec is what a client asks the daemon to start: the Job of run.toml, the
// worktree it runs in, the environment it runs with (every variable of the
// worktree, its ports included, resolved by the client: the daemon is blind to
// git) and the variables it never inherits from the daemon, and the Origin its
// events are published under, none publishing none.
type Spec struct {
	Job     project.Job
	WorkDir string
	Env     map[string]string
	Unset   []string
	Origin  *Origin
}

// An Origin is what the events of a job carry that only its client knows: the
// repository (its main worktree Root and git's CommonDir), the Branch of its
// worktree, and the CorrelationID of the command that started it.
type Origin struct {
	Root          string
	CommonDir     string
	Branch        string
	CorrelationID string
}

// Ports returns the ports the job binds: each port it declares, read from the
// environment the client resolved, the number the job itself reads; a port the
// environment does not spell is left out.
func (s Spec) Ports() map[string]int {
	ports := map[string]int{}
	for name := range s.Job.Ports {
		if port, err := strconv.Atoi(s.Env[name]); err == nil {
			ports[name] = port
		}
	}

	return ports
}

// Detached reports whether the job is a detached launcher: a service with a
// stop command of its own, whose cmd starts something that outlives it
// (`docker compose up -d`).
func (s Spec) Detached() bool {
	return s.Job.Kind == project.Service && s.Job.Stop != ""
}

// A Record is a job up as the daemon's index keeps it, for the next daemon to
// take over: its Spec, when it started, the process and group a foreground
// service runs as, whether it is a Claim on a shared job another worktree runs
// (with the main checkout that runs it, SharedDir, and whether main asked for
// it itself, MainHolds), and what a later part of the daemon keeps of it (its
// log directory, the routes its proxy serves), carried as they were.
type Record struct {
	Spec      Spec
	StartedAt time.Time
	PID       int
	PGID      int
	Claim     bool
	SharedDir string
	MainHolds bool
	LogDir    string
	Routes    []Route
}

// A Route is a name the proxy serves a job's port under.
type Route struct {
	Job  string
	Host string
	Port string
}

// Foreground reports whether the record is a process the daemon drains, the
// one kind it can find orphaned: a service with no stop command, no claim.
func (r Record) Foreground() bool {
	return !r.Claim && r.Spec.Job.Kind == project.Service && !r.Spec.Detached()
}

// Dir returns the directory the job runs in: its cwd, read from its worktree
// when relative, the worktree itself when it has none.
func (s Spec) Dir() string {
	switch {
	case s.Job.Cwd == "":
		return s.WorkDir
	case filepath.IsAbs(s.Job.Cwd):
		return s.Job.Cwd
	}

	return filepath.Join(s.WorkDir, s.Job.Cwd)
}

// URL returns where the job answers: on localhost at the port its url names,
// "" when it publishes none or that port is not bound.
func (s Spec) URL() string {
	if s.Job.URL == nil {
		return ""
	}
	port, bound := s.Ports()[s.Job.URL.Port]
	if !bound {
		return ""
	}

	return "http://localhost:" + strconv.Itoa(port)
}

// A Command is a process the daemon starts: Line, run by /bin/sh -c, in Dir,
// with the daemon's environment without Unset and with Env over it, under a
// terminal of its own when Terminal (a service the daemon drains) or through a
// pipe (a task, a launcher, a stop command), each in a process group of its
// own; what it prints goes to Output as it comes, from one goroutine.
type Command struct {
	Line     string
	Dir      string
	Env      map[string]string
	Unset    []string
	Terminal bool
	Output   func([]byte)
}

// Command returns the command that starts the job, its output to output. A
// service gets a colour terminal; a task, a launcher and a stop command a dumb
// one that still asks for colours, as CI would, unless the job's environment
// says otherwise.
func (s Spec) Command(output func([]byte)) Command {
	terminal := s.Job.Kind == project.Service && !s.Detached()
	defaults := map[string]string{"TERM": "dumb", "COLORTERM": "truecolor", "FORCE_COLOR": "1", "CI": "true"}
	if terminal {
		defaults = map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "FORCE_COLOR": "1"}
	}
	env := maps.Clone(defaults)
	maps.Copy(env, s.Env)

	return Command{Line: s.Job.Cmd, Dir: s.Dir(), Env: env, Unset: s.Unset, Terminal: terminal, Output: output}
}

// StopCommand returns the command a detached job's stack is stopped with: its
// stop, where and with what it started.
func (s Spec) StopCommand(output func([]byte)) Command {
	command := s.Command(output)
	command.Line = s.Job.Stop

	return command
}
