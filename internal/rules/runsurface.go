package rules

import "github.com/LucasPcq/wtm/internal/domain"

type RunSurfaceParams struct {
	// Inline marks a run the caller waits on and reads back afterwards — a task,
	// whose output belongs to the scrollback rather than to a screen given back
	// when it ends.
	Inline bool
	Detach bool
	TTY    bool
	Format string
}

// DecideRunSurface picks who shows a run's jobs. The full-screen view needs all
// three: a terminal to draw on, a human format to draw for, and a caller that
// did not ask for its prompt back. Everything else — JSON, a pipe, `-d`, a task
// — is a stream of lines, so a machine reading a run never faces an alternate
// screen it cannot leave.
func DecideRunSurface(params RunSurfaceParams) domain.RunSurface {
	if !IsHumanFormat(params.Format) {
		return domain.RunSurfaceMachine
	}
	if params.Inline || params.Detach || !params.TTY {
		return domain.RunSurfaceStream
	}
	return domain.RunSurfaceView
}
