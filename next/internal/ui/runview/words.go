package runview

// The view's words.
const (
	jobsTitle      = "JOBS"
	sharedHeading  = "shared"
	reachTitle     = "Where to reach it"
	reachNothing   = "Nothing running here has an address."
	noJobs         = "No job to show."
	noMatchFmt     = "No job matches %q."
	paneWaiting    = "Waiting for output…"
	paneNoHistory  = "No output recorded for this job."
	paneNotStarted = "Not started."
	paneFailedFmt  = "No output to show: %v"
	labelLive      = "live"
	labelHistory   = "history"
	labelFocus     = "focus"
	scrolledFmt    = "scrolled %d lines back"
	runningFmt     = "%d/%d running"
	worktreesFmt   = "%d worktrees"
	separator      = " · "
	filterPrompt   = "filter: "
	filterHintFmt  = "filter %q · esc clears"
	helpBrowse     = "↑↓ job · / filter · pgup/pgdn scroll · enter focus · o open · a addresses · q detach"
	helpFilter     = "type to filter · enter apply · esc clear"
	focusHintFmt   = "focus %s · %s — every key goes to the job · esc twice gives them back"
	bandDismiss    = "esc dismisses this report"
	crashedFmt     = "%s · %s %s since the view opened"
	openFailedFmt  = "could not open the browser: %v"

	stateNotStarted = "not started"
	stateJoinedFmt  = "joined, running in %s"
	stateDetached   = "detached"
	stateFailed     = "failed"
	stateDone       = "done"
	stateReaped     = "reaped"
	exitFmt         = "%s (exit %d)"

	noTerminalFmt     = "%s runs under no terminal: nothing to type into."
	sharedReadOnlyFmt = "%s runs in %s: its output is read-only here."
	noStreamFmt       = "%s has no live stream to type into."
)

// The marks of a job's state, runes every monospace font draws in one column:
// a stack its launcher left and a shared service are up like any other, their
// group and their title saying the rest.
const (
	glyphCursor   = "›"
	glyphStopped  = "−"
	glyphStarting = "…"
	glyphUp       = "•"
	glyphDone     = "✓"
	glyphFailed   = "✗"
	glyphNotice   = "!"
)
