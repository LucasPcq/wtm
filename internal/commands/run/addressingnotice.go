package run

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
)

// addressingDrift is the callout `run import` puts out where a teammate receives
// the addressing — run.toml is not committed, so that is the only way it travels.
// It answers whether there is anything to say, so a caller that has to open a
// frame can decide before opening it: a frame around nothing is two blank lines
// on every run that was fine.
func addressingDrift(config shared.ConfigResult, workDir string) (flow.Notice, bool) {
	return addressing.Notice(addressing.Params{
		Context:  shared.FlowContext(config),
		WorkDirs: []string{workDir},
	})
}
