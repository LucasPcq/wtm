package cli

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawResolve prints the path of the worktree resolved, alone on its line: what `cd "$(wtm resolve …)"` enters.
func drawResolve(w io.Writer, read resolve.Outcome, _ resolve.Request, _ output.View) {
	for _, item := range read.Items {
		if item.Detail != nil {
			_, _ = fmt.Fprintln(w, item.Detail.Path)
		}
	}
}
