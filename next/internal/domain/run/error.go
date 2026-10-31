package run

// The codes of what a run daemon refuses or fails, each on its [Error].
const (
	CodeInvalidRequest = "daemon.invalid_request" // a request it cannot read
	CodeUnknownAction  = "daemon.unknown_action"  // an action it does not know
	CodeJobRunning     = "daemon.job_running"     // a start of a job already up
	CodeEmptyCommand   = "daemon.empty_command"   // a job with no cmd
	CodeLaunchFailed   = "daemon.launch_failed"   // a job that could not start, or a launcher that failed
	CodeTaskFailed     = "daemon.task_failed"     // a task that exited non-zero
	CodeStopFailed     = "daemon.stop_failed"     // a stop command that failed
	CodeStopping       = "daemon.stopping"        // a start once the daemon is stopping
	CodeJobNotFound    = "daemon.job_not_found"   // lines, an attach or a resize of a job the daemon does not hold
	CodeNoTerminal     = "daemon.no_terminal"     // a resize of a job that runs under no terminal
	CodeFollowerBehind = "daemon.follower_behind" // a follower of a job's lines that fell behind and was let go: it follows again
	CodeSkew           = "daemon.skew"            // a daemon of another build or protocol holding jobs: run daemon restart hands them over
	CodeLegacy         = "daemon.legacy"          // the legacy wtm's daemon holding jobs: run daemon restart hands them over
	CodeBusy           = "daemon.busy"            // a process holding the socket that is no run daemon
	CodeSocketTooLong  = "daemon.socket_path_too_long"
	CodeStartFailed    = "daemon.start_failed" // a daemon that did not start
	CodeStopTimeout    = "daemon.stop_timeout" // a daemon asked to stop that did not
)

// An Error is what a run daemon refused or failed: its Code, its Message, and
// the ExitCode of the job it ran, if one ran.
type Error struct {
	Code     string
	Message  string
	ExitCode *int
}

// Error returns its Message.
func (e *Error) Error() string { return e.Message }
