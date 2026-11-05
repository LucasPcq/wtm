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
	helpBrowse     = "↑↓ job · enter focus · s stop · r restart · o open · q detach · / filter · a addresses · pgup/pgdn scroll"
	helpFilter     = "type to filter · enter apply · esc clear"
	focusHintFmt   = "focus %s · %s — every key goes to the job · esc twice gives them back"
	bandDismiss    = "esc dismisses this report"
	crashedFmt     = "%s · %s %s since the view opened"
	openFailedFmt  = "could not open the browser: %v"
	typeFailedFmt  = "could not type into %s: %v"
	streamBrokeFmt = "%s: its output stopped: %v"

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

	actingFmt        = "%s %s…"
	actedFmt         = "%s %s"
	actFailedFmt     = "could not %s %s: %v"
	underWayFmt      = "%s is under way: wait for it"
	restartSharedFmt = "%s runs for every worktree holding it: stop it, then run start"
	notActedFmt      = "%s: nothing to %s here"
	cutShortFmt      = "restart of %s cut short: %s is stopped — %s starts it again"
	unendedFmt       = "%s still under way when the view was left: check it with %s"
	leavingFmt       = "leaving once %s ends · Ctrl+C again leaves now"
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
