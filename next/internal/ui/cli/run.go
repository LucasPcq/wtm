package cli

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// A Run is what a command prints its run with: its name, its streams, the environment of its process, how its follow-ups are spelled, the --output format and --quiet the user gave, the flags its failures are spelled with and the exit codes of its own codes.
type Run struct {
	Command     string // the name the envelope reports
	Streams     *output.Streams
	Env         Env
	CommandLine output.CommandLine // given by the composition root, the one that knows every command
	Output      string             // bound to the command's --output flag
	Quiet       bool               // read from --quiet by [Run.Begin]
	Flags       output.Flags
	Exits       map[string]int
}

// Begin reads --quiet and refuses an --output format other than text or json, printed as text, the one format sure to be understood, followed by the way to cmd's help; then it refuses an invalid $WTM_CORRELATION_ID, before any repository is looked up.
func (r *Run) Begin(cmd *cobra.Command) error {
	r.Quiet, _ = cmd.Flags().GetBool(FlagQuiet)
	if r.Output != OutputText && r.Output != OutputJSON {
		refused := domain.InvalidMessage(`invalid --output "` + r.Output + `": expected one of text, json`)
		r.Output = OutputText
		return r.FailWith(refused, []output.NextStep{HelpStep(cmd)})
	}

	if err := domain.CorrelationProblem(r.Env.CorrelationID); err != nil {
		return r.FailWith(domain.InvalidMessage("$"+CorrelationIDEnv+" "+err.Error()+": invalid correlation id"), nil)
	}

	return nil
}

// Human reports whether the output is for a person, not the JSON envelope.
func (r *Run) Human() bool {
	return r.Output == OutputText
}

// Writers are stdout and stderr, both silenced by --quiet in human output.
func (r *Run) Writers() (io.Writer, io.Writer) {
	if r.Quiet && r.Human() {
		return io.Discard, io.Discard
	}

	return r.Streams.Out, r.Streams.Err
}

// Progress starts the progress of the operation ctx runs on stderr, live when stderr can animate and --quiet is off.
func (r *Run) Progress(ctx context.Context) *output.Progress {
	_, stderr := r.Writers()
	return output.NewProgress(ctx, output.ProgressParams{W: stderr, Human: r.Human(), Live: !r.Quiet && r.Streams.CanAnimate()})
}

// FailWith prints failure followed by next, or the envelope that carries it with --output json, and returns the failure that sets the exit code.
func (r *Run) FailWith(failure *domain.Error, next []output.NextStep) error {
	if !r.Human() {
		return r.Envelope(domain.Envelope{Command: r.Command, Err: failure})
	}

	_, stderr := r.Writers()
	output.Frame(stderr, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Next: next, Flags: r.Flags})
	})

	return r.Failed(failure)
}

// View is what the command's outcome is drawn from: the working directory and how its follow-ups are spelled.
func (r *Run) View() output.View {
	return output.View{Cwd: r.Env.Cwd, CommandLine: r.CommandLine}
}

// Envelope writes the v2 JSON envelope doc on stdout, each field it refuses spelled as its flag, and returns its failure; an envelope that cannot be written is the failure.
func (r *Run) Envelope(doc domain.Envelope) error {
	if err := output.Envelope(r.Streams.Out, doc, r.Flags.Spell); err != nil {
		return r.Failed(domain.AsError(domain.AsErrorParams{Err: err}))
	}

	return r.Failed(doc.Err)
}

// Failed is the error the root reads the exit code and the --quiet line from, nil for no failure.
func (r *Run) Failed(failure *domain.Error) error {
	if failure == nil {
		return nil
	}

	return &Failed{Err: failure, Quiet: r.Quiet && r.Human(), Flags: r.Flags, Exits: r.Exits}
}
