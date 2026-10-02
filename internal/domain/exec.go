package domain

import "time"

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

const (
	FlagPrint = "print"

	ExecLogDirName          = "exec"
	ExecLogFileExt          = ".log"
	ExecJSONTailLines       = 20
	ExecConclusionTailLines = 10
	// ExecPartialLineCap bounds a line that never ends (binary output, a
	// progress bar without \r) so the tail cannot grow without limit.
	ExecPartialLineCap = 4096
	ExecInterruptGrace = 5 * time.Second

	ExecWizardErrLabel       = "exec"
	ExecSelectionLabel       = "Worktrees"
	ExecSelectionTitle       = "Run in which worktrees?"
	ExecSelectAtLeastOne     = "select at least one worktree"
	ExecSelectionRequiredFmt = "no worktree selected: pass worktree names or --%s (a run with --%s or --%s %s cannot open the picker)"
	ExecConfirmLabel         = "Confirm"
	ExecConfirmTitle         = "Run this command?"
	ExecConfirmOption        = "Run"
	ExecConfirmValue         = "run"
	ExecRecapWorktrees       = "Worktrees:   "
	ExecRecapCommand         = "Command:     "
	ExecRecapJobs            = "Concurrency: "
	ExecNeedsTerminal        = "wtm exec needs a terminal to pick worktrees: pass worktree names or --all, with --yes"

	ExecQueuedLabel      = "queued"
	ExecRunningLabel     = "running"
	ExecInterruptedLabel = "interrupted"
	ExecNotStartedLabel  = "not started"
	ExecExitFmt          = "exit %d"
	ExecPassedLabelFmt   = "%s (%s)"
	ExecFailedLabelFmt   = "%s (%s, %s)"
	ExecStateLabelFmt    = "%s  %s"
	ExecAllPassedFmt     = "%s · %d worktrees (%s)"
	ExecSomeFailedFmt    = "%s · %d of %d worktrees failed"
	ExecPassedCountFmt   = "%d passed"
	ExecLogLabel         = "log"
)
