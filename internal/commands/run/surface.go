package run

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/tui/runview"
)

// isTTY reports whether this command owns a terminal. It is a variable so a
// test can answer yes: which surface a run gets hangs on it, and nothing else
// makes a pipe look like a terminal.
var isTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// showRunView is the full-screen surface, a variable for the same reason: a
// test can watch a command hand over to it without bubbletea taking over a
// terminal the test does not have.
var showRunView = openRunView

type viewParams struct {
	Cmd       *cobra.Command
	Board     runlogs.Board
	Job       string
	Profile   string
	Worktrees []string
	Warnings  []string
	Start     runlogs.StartFunc
	// Hyperlinks says whether a job's URL may be wrapped in an OSC-8 sequence,
	// for the lines a detached run prints once the view is gone.
	Hyperlinks bool
}

// openRunView hands the terminal to the full-screen view and frames what it
// leaves behind. The view returns its recap rather than printing it: the
// alternate screen has to be given back before anything is written to the
// scrollback underneath it. What the run concluded goes back to the flow, which
// is what turns it into an exit code.
func openRunView(params viewParams) (runlogs.Outcomes, error) {
	out := params.Cmd.OutOrStdout()
	rest := &detachedRun{params: params}
	result, err := runview.Run(runview.Params{
		Board:     params.Board,
		Job:       params.Job,
		Profile:   params.Profile,
		Worktrees: params.Worktrees,
		Warnings:  params.Warnings,
		Start:     params.Start,
		Open:      integration.OpenURL,
		Detach:    runview.Detach{Notice: rest.open, Sink: rest, Await: true},
	})
	if err != nil {
		return nil, err
	}

	if result.Detached {
		output.FrameEnd(out)
		return result.Outcomes, nil
	}
	if result.Recap != "" {
		output.Frame(out, func(w io.Writer) { fmt.Fprintln(w, result.Recap) })
	}
	return result.Outcomes, nil
}

// detachedRun reports what is left of a run once the reader has closed the
// view: the same lines `-d` prints, in a frame opened only if they actually
// left. It is built before the view opens and stays inert until then, because
// the view is the only thing that knows whether they did.
type detachedRun struct {
	params  viewParams
	printer *output.RunPrinter
}

// open runs with the terminal already given back, which is what makes writing
// to the scrollback safe: the alternate screen is gone by the time the view has
// said the reader left.
func (d *detachedRun) open() {
	out, errOut := d.params.Cmd.OutOrStdout(), d.params.Cmd.ErrOrStderr()
	output.FrameStart(out)
	output.Message(out, domain.RunDetachedNotice)
	d.printer = output.NewRunPrinter(output.RunPrinterParams{
		Out:        out,
		Err:        errOut,
		Profile:    d.params.Profile,
		Worktrees:  d.params.Worktrees,
		Hyperlinks: d.params.Hyperlinks,
	})
}

func (d *detachedRun) Emit(event runlogs.Event) {
	if d.printer == nil {
		return
	}
	d.printer.Emit(event)
}

type streamParams struct {
	Cmd       *cobra.Command
	Profile   string
	Worktrees []string
	Warnings  []string
	Start     runlogs.StartFunc
	// Hyperlinks says whether a job's URL may be wrapped in an OSC-8 sequence.
	Hyperlinks bool
}

// runOnStream reports a start sequence as lines on the terminal it was launched
// from — `-d`, a pipe, a CI job. An aborted run ends on stderr, which is where
// its frame closes.
func runOnStream(params streamParams) (runlogs.Outcomes, error) {
	out, errOut := params.Cmd.OutOrStdout(), params.Cmd.ErrOrStderr()

	output.FrameStart(out)
	outcomes, err := params.Start(params.Cmd.Context(), output.NewRunPrinter(output.RunPrinterParams{
		Out:        out,
		Err:        errOut,
		Profile:    params.Profile,
		Worktrees:  params.Worktrees,
		Hyperlinks: params.Hyperlinks,
	}))
	if err != nil {
		return nil, err
	}

	if outcomes.Aborted() {
		output.FrameEnd(errOut)
		return outcomes, nil
	}
	output.FrameEnd(out)
	// A stream has no band to hold it, so the warning follows the lines it
	// qualifies rather than sitting above them.
	if len(params.Warnings) > 0 {
		output.Frame(errOut, func(w io.Writer) {
			output.Callout(w, domain.AddressingDriftTitle, params.Warnings)
		})
	}
	return outcomes, nil
}

// runForMachine emits the run's outcome as a JSON document, then fails when the
// profile aborted. The document is complete either way: the module's rule is
// that the shape follows the arity and the exit code follows the success, and
// an exit code has never made a document unreadable (LUC-198).
func runForMachine(params streamParams) (runlogs.Outcomes, error) {
	outcomes, err := params.Start(params.Cmd.Context(), nil)
	if err != nil {
		return nil, err
	}
	if err := output.WriteRunOutcomesJSON(params.Cmd.OutOrStdout(), outcomes); err != nil {
		return nil, err
	}
	return outcomes, nil
}
