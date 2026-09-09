package run

import (
	"io"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/output"
)

// noticeAddressingDrift is the callout the commands outside the run flows put
// out: `run init` where the addressing is decided, `run import` where a
// teammate receives it — run.toml is not committed, so that is the only way it
// travels — and `run open` at the moment a name is actually followed.
func noticeAddressingDrift(w io.Writer, config shared.ConfigResult, workDir string) {
	notice, ok := addressing.Notice(addressing.Params{
		Context:  shared.FlowContext(config),
		WorkDirs: []string{workDir},
	})
	if !ok {
		return
	}
	output.Callout(w, notice.Text, notice.Lines)
}
