// Package daemon is the run daemon's protocol and files: the server that answers each request on its socket, the client every command asks it through, and its index of the jobs up, jobs.json, shared with the legacy wtm.
package daemon

import (
	"cmp"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// The protocol: NDJSON on the unix socket, one request per connection, a
// request one line, then answer lines, each stamped with the daemon's protocol,
// build and pid. A start's answers are its lines, then ok or error.
const (
	actionList     = "list"
	actionStart    = "start"
	actionStop     = "stop"
	actionStopAll  = "stop_all"
	actionShutdown = "shutdown"
	actionLines    = "lines"
	actionAttach   = "attach"
	actionResize   = "resize"

	statusOK    = "ok"
	statusError = "error"
	statusLine  = "line"
)

// request is the one line a client writes.
type request struct {
	Action        string `json:"action"`
	Name          string `json:"name,omitempty"`
	WorkDir       string `json:"work_dir,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Spec          *spec  `json:"spec,omitempty"`
	Tail          int    `json:"tail,omitempty"`
	Follow        bool   `json:"follow,omitempty"`
	Cols          int    `json:"cols,omitempty"`
	Rows          int    `json:"rows,omitempty"`
}

// answer is one line the daemon writes back.
type answer struct {
	Status   string    `json:"status"`
	Protocol int       `json:"protocol,omitempty"`
	Version  string    `json:"version,omitempty"`
	PID      int       `json:"pid,omitempty"`
	Code     string    `json:"code,omitempty"`
	Message  string    `json:"message,omitempty"`
	ExitCode *int      `json:"exit_code,omitempty"`
	Jobs     []job     `json:"jobs,omitzero"`
	Job      *job      `json:"job,omitempty"`
	Outcome  string    `json:"outcome,omitempty"`
	At       time.Time `json:"at,omitzero"`
	Text     string    `json:"text,omitempty"`
}

// stamp is what an answer says of the daemon that wrote it.
func (a answer) stamp() run.Stamp {
	return run.Stamp{Protocol: a.Protocol, Version: a.Version, PID: a.PID}
}

// failure is the error an answer reports, nil for one that is no error.
func (a answer) failure() error {
	if a.Status != statusError {
		return nil
	}

	return &run.Error{Code: cmp.Or(a.Code, run.CodeInvalidRequest), Message: "run daemon: " + a.Message, ExitCode: a.ExitCode}
}

// job is a job as the daemon reports it. Status is the legacy daemon's word
// for how it stands: written for a legacy client, which counts the jobs a
// daemon holds by it, and read from a legacy daemon alone.
type job struct {
	Name      string         `json:"name"`
	Kind      string         `json:"kind,omitempty"`
	WorkDir   string         `json:"work_dir"`
	SharedDir string         `json:"shared_dir,omitempty"`
	State     string         `json:"state,omitempty"`
	Hold      string         `json:"hold,omitempty"`
	Reaped    bool           `json:"reaped,omitempty"`
	PID       int            `json:"pid,omitempty"`
	StartedAt time.Time      `json:"started_at,omitzero"`
	ExitCode  *int           `json:"exit_code,omitempty"`
	URL       string         `json:"url,omitempty"`
	Ports     map[string]int `json:"ports,omitempty"`
	LastLine  *line          `json:"last_line,omitempty"`
	Outcome   string         `json:"outcome,omitempty"`
	Status    string         `json:"status,omitempty"`
}

// line is a line a job printed, on the wire.
type line struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

func jobOf(j run.Job) job {
	wire := job{Name: j.Name, Kind: string(j.Kind), WorkDir: j.WorkDir, SharedDir: j.SharedDir, State: string(j.State), Hold: string(j.Hold), Reaped: j.Reaped, PID: j.PID, StartedAt: j.StartedAt, ExitCode: j.ExitCode, URL: j.URL, Ports: j.Ports, Status: legacyStatus(j)}
	if j.LastLine != nil {
		wire.LastLine = &line{At: j.LastLine.At, Text: j.LastLine.Text}
	}

	return wire
}

// legacyStatus is the legacy daemon's word for how j stands.
func legacyStatus(j run.Job) string {
	switch {
	case j.Hold == run.HoldClaim:
		return "joined"
	case j.Reaped:
		return "reaped"
	case j.State == run.Running && j.Hold == run.HoldDetached:
		return "detached"
	case j.State == run.Running, j.State == run.Crashed:
		return string(j.State)
	}

	return "stopped"
}

// runJob is the job as the client reads it, from a daemon of this protocol, or
// of the legacy wtm when legacy.
func (j job) runJob(legacy bool) run.Job {
	if legacy {
		return run.Job{Name: j.Name, WorkDir: j.WorkDir, Up: legacyUp(j.Status), Kind: project.JobKind(j.Kind), State: cmp.Or(run.State(j.State), legacyState(j.Status)), Hold: legacyHold(j.Status), PID: j.PID, URL: j.URL, ExitCode: j.ExitCode, StartedAt: j.StartedAt, SharedDir: j.SharedDir}
	}

	hold, state := run.Hold(j.Hold), run.State(j.State)
	read := run.Job{Name: j.Name, WorkDir: j.WorkDir, Up: hold == run.HoldClaim || state == run.Running, Kind: project.JobKind(j.Kind), State: state, Hold: hold, Reaped: j.Reaped, PID: j.PID, URL: j.URL, Ports: j.Ports, ExitCode: j.ExitCode, StartedAt: j.StartedAt, SharedDir: j.SharedDir}
	if j.LastLine != nil {
		read.LastLine = &run.Line{At: j.LastLine.At, Text: j.LastLine.Text}
	}

	return read
}

// legacyUp reports whether a job of the legacy status is up: running in the
// foreground, detached, or joined by its worktree (attached, as older daemons
// say).
func legacyUp(status string) bool {
	switch status {
	case "running", "detached", "joined", "attached":
		return true
	}

	return false
}

// legacyHold is what the legacy daemon holds of a job of status: a claim
// (joined, attached as older daemons say), a detached stack, or its own.
func legacyHold(status string) run.Hold {
	switch status {
	case "joined", "attached":
		return run.HoldClaim
	case "detached":
		return run.HoldDetached
	}

	return run.HoldOwn
}

// legacyState is the state of a job of the legacy status, for a daemon too old
// to say it: a detached service runs, a reaped one is stopped, a claim has
// none.
func legacyState(status string) run.State {
	switch status {
	case "running", "detached":
		return run.Running
	case "crashed":
		return run.Crashed
	case "stopped", "reaped":
		return run.Stopped
	}

	return ""
}

// spec is what a start asks for, on the wire.
type spec struct {
	Job     config            `json:"job"`
	WorkDir string            `json:"work_dir"`
	Env     map[string]string `json:"env,omitempty"`
	Unset   []string          `json:"unset,omitempty"`
	LogDir  string            `json:"log_dir,omitempty"`
	Origin  *origin           `json:"origin,omitempty"`
}

type origin struct {
	Root          string `json:"root"`
	CommonDir     string `json:"common_dir"`
	Branch        string `json:"branch,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func specOf(s run.Spec) *spec {
	wire := &spec{Job: configOf(s.Job), WorkDir: s.WorkDir, Env: s.Env, Unset: s.Unset, LogDir: s.LogDir}
	if s.Origin != nil {
		wire.Origin = &origin{Root: s.Origin.Root, CommonDir: s.Origin.CommonDir, Branch: s.Origin.Branch, CorrelationID: s.Origin.CorrelationID}
	}

	return wire
}

func (s spec) runSpec() run.Spec {
	runSpec := run.Spec{Job: s.Job.job(), WorkDir: s.WorkDir, Env: s.Env, Unset: s.Unset, LogDir: s.LogDir}
	if s.Origin != nil {
		runSpec.Origin = &run.Origin{Root: s.Origin.Root, CommonDir: s.Origin.CommonDir, Branch: s.Origin.Branch, CorrelationID: s.Origin.CorrelationID}
	}

	return runSpec
}

// config is a job of run.toml with the keys the legacy wtm's index spells it
// with, the protocol's and the index's.
type config struct {
	Name  string         `json:"name"`
	Kind  string         `json:"kind"`
	Cmd   string         `json:"cmd"`
	Stop  string         `json:"stop,omitempty"`
	Cwd   string         `json:"cwd,omitempty"`
	Ports map[string]int `json:"ports,omitempty"`
	URL   *urlConfig     `json:"url,omitempty"`
	legacyKeys
	Scope     string           `json:"scope,omitempty"`
	Namespace *namespaceConfig `json:"namespace,omitempty"`
}

// legacyKeys are the keys of a job of run.toml the legacy index keeps that the
// run daemon never reads: an index taken over keeps them, written back where
// and as legacy wrote them.
type legacyKeys struct {
	Probe       *bool    `json:"probe,omitempty"`
	BindsNoPort bool     `json:"binds_no_port,omitempty"`
	Runs        []string `json:"runs,omitempty"`
	Touches     []string `json:"touches,omitempty"`
}

type urlConfig struct {
	Port string `json:"port"`
	Host string `json:"host,omitempty"`
}

type namespaceConfig struct {
	Name   string            `json:"name"`
	Create string            `json:"create,omitempty"`
	Remove string            `json:"remove,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
}

func configOf(j project.Job) config {
	wire := config{Name: j.Name, Kind: string(j.Kind), Cmd: j.Cmd, Stop: j.Stop, Cwd: j.Cwd, Ports: j.Ports, Scope: string(j.Scope)}
	if j.URL != nil {
		wire.URL = &urlConfig{Port: j.URL.Port, Host: j.URL.Host}
	}
	if j.Namespace != nil {
		wire.Namespace = &namespaceConfig{Name: j.Namespace.Name, Create: j.Namespace.Create, Remove: j.Namespace.Remove, Env: j.Namespace.Env}
	}

	return wire
}

func (c config) job() project.Job {
	j := project.Job{Name: c.Name, Kind: project.JobKind(c.Kind), Cmd: c.Cmd, Stop: c.Stop, Cwd: c.Cwd, Ports: c.Ports, Scope: project.Scope(c.Scope)}
	if c.URL != nil {
		j.URL = &project.URL{Port: c.URL.Port, Host: c.URL.Host}
	}
	if c.Namespace != nil {
		j.Namespace = &project.Namespace{Name: c.Namespace.Name, Create: c.Namespace.Create, Remove: c.Namespace.Remove, Env: c.Namespace.Env}
	}

	return j
}
