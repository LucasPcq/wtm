package cli

import (
	"errors"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

// The bare ErrAborted has no text worth reading: a silenced run points at the
// flag that took its report away rather than pretending to explain.
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

func reportFailure(w io.Writer, err error) {
	if errors.Is(err, domain.ErrCancelled) && !errors.Is(err, domain.ErrLeftBehind) {
		render.Unchanged(w, abortLine(err))
		return
	}
	render.Error(w, abortLine(err))
}
