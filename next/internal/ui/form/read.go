package form

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// A ReadOutcome is how a read command's run ended, as [domain.Read] reports it: the failure it ended on and the envelope of the read.
type ReadOutcome interface {
	Failure() *domain.Error
	Envelope(command string, err *domain.Error) domain.Envelope
}

// A Reader is what the form calls of a read command's use case: its run.
type Reader[R, O any] interface {
	Run(ctx context.Context, req R, opts domain.RunOptions) (O, error)
}

// A Query is what a read command's cli hands the form: the use case it opens, how it draws what it read, and what the user may pick of it on a terminal.
// A read asks no question, has no plan and takes no lock: its flags are its schema's and --output, without --yes, --dry-run, --expect-fingerprint, --remember nor --ask.
type Query[R Request[R], Q Questions, O ReadOutcome] struct {
	Open func(ctx context.Context, dir string) (Reader[R, O], error)
	Draw func(w io.Writer, read O, cwd string) // what was read, cwd the directory its paths are shown from
	// Pick lets the user pick a follow-up of what was read, when the user can see and answer it: it returns the read with the follow-ups chosen. Nil only draws the read.
	Pick func(ctx context.Context, read O, req R, run *uicli.Run) (O, error)

	io *uicli.Run
}

func (q *Query[R, Q, O]) schema() domain.Form {
	var zero Q
	return zero.Form()
}

// Flags spells each field path of the request as the flag that sets it, for the failures.
func (q *Query[R, Q, O]) Flags() output.Flags {
	flags, _ := q.schema().Flags()
	return flags
}

// Bind registers on cmd a flag per field of the schema, by its kind, then --output; run is where the command prints its read.
func (q *Query[R, Q, O]) Bind(cmd *cobra.Command, run *uicli.Run) {
	q.io = run
	bindFields(cmd, q.schema())
	cmd.Flags().StringVar(&run.Output, "output", uicli.OutputText, uicli.OutputUsage)
}

// Launch runs cmd: it refuses an invalid --output or correlation ID, builds the request from the flags and args, opens the use case on the project's directory and reads, then lets the user pick of the read when Pick is set and the user can see and answer it; it prints how the read ended and returns the failure that sets the exit code.
func (q *Query[R, Q, O]) Launch(cmd *cobra.Command, args []string) error {
	if err := q.io.Begin(cmd); err != nil {
		return err
	}

	req := folded[R](fieldAnswers(cmd, args, q.schema()))

	ctx := cmd.Context()
	app, err := q.Open(ctx, q.io.Env.ProjectDir)
	if err != nil {
		return q.io.FailWith(domain.AsError(domain.AsErrorParams{Err: err}), nil)
	}

	read, err := app.Run(ctx, req, domain.RunOptions{Surface: uicli.SurfaceCLI, CorrelationID: q.io.Env.CorrelationID})
	if err == nil && q.Pick != nil && q.interactive() {
		read, err = q.Pick(ctx, read, req, q.io)
	}

	return q.conclude(read, err)
}

// interactive reports whether the user can see and answer a picker, and see what it ends on: stdout too is a terminal, since a read piped elsewhere is asked for its data, the output is for a human and --quiet does not silence it.
func (q *Query[R, Q, O]) interactive() bool {
	return q.io.Streams.CanPrompt() && q.io.Streams.Out.IsTerminal() && q.io.Human() && !q.io.Quiet
}

// conclude prints how the read ended — the JSON envelope, a refusal, the follow-ups the user picked, or what Draw draws of the read — and returns the failure that sets the exit code.
func (q *Query[R, Q, O]) conclude(read O, err error) error {
	failure := domain.AsError(domain.AsErrorParams{Err: err})
	doc := read.Envelope(q.io.Command, failure)
	switch {
	case !q.io.Human():
		return q.io.Envelope(doc)
	case failure != nil:
		return q.io.FailWith(failure, nil)
	}

	stdout, _ := q.io.Writers()
	// todo: run the follow-up the user picked, in this process, once every command it names has migrated; it is printed until then.
	if len(doc.FollowUps) > 0 {
		output.Frame(stdout, func(w io.Writer) { output.NextSteps(w, nextSteps(doc.FollowUps)) })
		return q.io.Failed(read.Failure())
	}

	output.Frame(stdout, func(w io.Writer) { q.Draw(w, read, q.io.Env.Cwd) })
	return q.io.Failed(read.Failure())
}

func nextSteps(followUps []domain.FollowUp) []output.NextStep {
	steps := make([]output.NextStep, 0, len(followUps))
	for _, followUp := range followUps {
		steps = append(steps, output.NextStep{Command: output.Line(followUp.Argv), Note: followUp.Message})
	}

	return steps
}
