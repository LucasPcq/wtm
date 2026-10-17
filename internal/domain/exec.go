package domain

type ExecStatus string

const (
	ExecStatusPassed      ExecStatus = "passed"
	ExecStatusFailed      ExecStatus = "failed"
	ExecStatusInterrupted ExecStatus = "interrupted"
	ExecStatusNotStarted  ExecStatus = "not_started"
)

type ExecResult struct {
	Branch string     `json:"branch"`
	Path   string     `json:"path"`
	Status ExecStatus `json:"status"`
	// ExitCode is a pointer because 0 is a passing code that must be written,
	// while a command that never ran has none.
	ExitCode   *int     `json:"exit_code,omitempty"`
	DurationMs int64    `json:"duration_ms,omitempty"`
	Log        string   `json:"log,omitempty"`
	Tail       []string `json:"tail,omitempty"`
	Output     string   `json:"output,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type ExecBeat struct {
	Index   int
	Started bool
	Result  ExecResult
}

type ExecCounts struct {
	Passed      int
	Failed      int
	Interrupted int
	NotStarted  int
}

// ProcessLink is one row of the process table: a process, its parent, its group.
type ProcessLink struct {
	PID  int
	PPID int
	PGID int
}
