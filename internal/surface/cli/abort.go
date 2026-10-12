package cli

import (
	"errors"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

// abortLine is what a silenced run says instead of nothing. A wrapped cause is
// printed as it is; the bare sentinel has no text worth reading, so it points at
// the flag that took the report away rather than pretending to explain.
func abortLine(err error) string {
	if errors.Is(err, domain.ErrLeftBehind) {
		return err.Error()
	}
	if errors.Is(err, domain.ErrCancelled) {
		return domain.AbortedMessage
	}
	if errors.Is(err, domain.ErrAborted) && err.Error() == domain.ErrAborted.Error() {
		return domain.QuietAbortedMessage
	}
	return err.Error()
}

// reportFailure prints a bare interrupt in the register of a run the user backed
// out of — `=`, nothing failed — and anything else, a worktree an interrupt left
// behind included, as the failure it is.
func reportFailure(w io.Writer, err error) {
	if errors.Is(err, domain.ErrCancelled) && !errors.Is(err, domain.ErrLeftBehind) {
		render.Unchanged(w, abortLine(err))
		return
	}
	render.Error(w, abortLine(err))
}
