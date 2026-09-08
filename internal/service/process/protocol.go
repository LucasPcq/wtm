package process

import "github.com/LucasPcq/wtm/internal/domain"

// RequestAction identifies the daemon command.
type RequestAction string

const (
	ActionStart    RequestAction = "start"
	ActionShutdown RequestAction = "shutdown"
	ActionStop     RequestAction = "stop"
	ActionStopAll  RequestAction = "stop_all"
	ActionList     RequestAction = "list"
	ActionAttach   RequestAction = "attach"
	ActionResize   RequestAction = "resize"
)

// Request is a JSON message sent from client to daemon.
type Request struct {
	Action  RequestAction     `json:"action"`
	Job     *domain.JobConfig `json:"job,omitempty"`
	Name    string            `json:"name,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	// LogDir is where the daemon persists the job's output. The client resolves
	// it, so the daemon never has to run git to find the state dir. Empty
	// persists nothing.
	LogDir string `json:"log_dir,omitempty"`
	// Env is what the job learns about the worktree it belongs to. The client
	// resolves it for the same reason it resolves LogDir, and one more: the
	// daemon is global and long-lived, so the environment it inherited belongs
	// to whichever worktree happened to fork it.
	Env map[string]string `json:"env,omitempty"`
	// Routes are the names the proxy is to serve once this job runs — its own,
	// and one per published job it runs itself. They are resolved by the client
	// for the same reason LogDir and Env are: the daemon is global, and only the
	// client can ask git which worktree and which repository this is.
	Routes []domain.JobRoute `json:"routes,omitempty"`
	// Cols and Rows size the job's PTY: once on ActionAttach, then on every
	// ActionResize as the pane rendering the job changes size.
	Cols int `json:"cols,omitempty"`
	Rows int `json:"rows,omitempty"`
}

// ResponseStatus is the status field in a daemon response. For long-lived
// actions like launching a task, the daemon emits multiple NDJSON responses:
// zero or more StatusOutput with Data chunks, followed by StatusDone (success)
// or StatusError (failure). Simple actions end with a single StatusOK.
type ResponseStatus string

const (
	StatusOK     ResponseStatus = "ok"
	StatusError  ResponseStatus = "error"
	StatusOutput ResponseStatus = "output" // streamed chunk of task output
	StatusDone   ResponseStatus = "done"   // task exited successfully
)

// Response is a JSON message sent from daemon to client.
type Response struct {
	Status ResponseStatus `json:"status"`
	// Version is the daemon's own build, stamped on every answer. The client
	// compares rather than asking: a daemon too old to know the field answers
	// with an empty one, which is exactly the divergence worth catching, and it
	// costs no round-trip on the nominal path.
	Version string `json:"version,omitempty"`
	// DaemonPID is what `run daemon status` reports, so a user always has a way
	// out even when the socket stops answering.
	DaemonPID int              `json:"daemon_pid,omitempty"`
	Message   string           `json:"message,omitempty"`
	Jobs      []domain.JobInfo `json:"jobs,omitempty"`
	Data      []byte           `json:"data,omitempty"`      // task output chunk
	ExitCode  *int             `json:"exit_code,omitempty"` // final task exit code
	// Ports are the ports the job just started actually bound, base plus this
	// worktree's offset. Reported rather than recomputed by the caller, so the
	// line it prints says what happened instead of what should have.
	Ports map[string]int `json:"ports,omitempty"`
	// ProxyPort is the port the run proxy is really serving on, zero when it is
	// off or could not bind. The daemon is the only side that knows: a client
	// only knows what it asked for, and a URL built on that would be a lie.
	ProxyPort int `json:"proxy_port,omitempty"`
	// ProxyPublicPort is what a named URL announces: ProxyPrivilegedPort when
	// the probe reached us behind it, ProxyPort otherwise.
	ProxyPublicPort int `json:"proxy_public_port,omitempty"`
}
