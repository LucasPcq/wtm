package form

import (
	"context"
	"errors"
	"io"
	"maps"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// A ReadOutcome is how a read command's run ended, as [domain.Read] reports it: the failure it ended on and the envelope of the read.
type ReadOutcome interface {
	Failure() *domain.Error
	Envelope(params domain.EnvelopeParams) domain.Envelope
}

// A Reader is what the form calls of a read command's use case: the questions of a request, and its run.
type Reader[R, Q, O any] interface {
	Questions(ctx context.Context, req R) (Q, error)
	Run(ctx context.Context, req R, opts domain.RunOptions) (O, error)
}

// A Query is what a read command's cli hands the form: the use case it opens, how it words its fields, how it draws what it read, and what the user may pick of it on a terminal.
// A read has no plan and takes no lock: its flags are its schema's and --output, without --yes, --dry-run, --expect-fingerprint, --remember nor --ask. The one question it asks is its subject, a positional select left open, before it reads.
type Query[R Request[R], Q Questions, O ReadOutcome] struct {
	Open  func(ctx context.Context, dir string) (Reader[R, Q, O], error)
	Words Worder[Q]
	Draw  func(w io.Writer, read O, req R, view output.View) // what was read for req, drawn from view
	// Pick lets the user pick a follow-up of what was read, when the user can see and answer it: it returns the read with the follow-ups chosen. Nil only draws the read.
	Pick func(ctx context.Context, read O, req R, run *uicli.Run) (O, error)
	// Formats are the machine formats the read prints besides json, by the name --output gives them: each drawn unframed on stdout from what was read, never silenced by --quiet; a refusal is printed as text.
	Formats map[string]func(w io.Writer, read O)
	// Captured says the read's text is data a shell captures, `cd "$(wtm resolve feat/x)"`: Draw draws it bare on stdout, never framed nor silenced by --quiet, a failure goes to stderr, and its subject is asked on a terminal even while stdout is captured.
	// shortcut: a captured read's warnings are not drawn; draw them on stderr once a captured read has any.
	Captured bool

	io *uicli.Run
}

func (q *Query[R, Q, O]) schema() []worded {
	return wordedSchema(q.Words)
}

// Flags spells each field path of the request as the flag that sets it, for the failures.
func (q *Query[R, Q, O]) Flags() output.Flags {
	flags, _ := flagsOf(q.schema())
	return flags
}

// Bind registers on cmd a flag per field of the schema, by its kind, then --output, which takes Formats too; run is where the command prints its read.
func (q *Query[R, Q, O]) Bind(cmd *cobra.Command, run *uicli.Run) {
	q.io = run
	run.Formats = slices.Sorted(maps.Keys(q.Formats))
	bindFields(cmd, q.schema())
	cmd.Flags().StringVar(&run.Output, "output", uicli.OutputText, run.OutputHelp())
}

// Launch runs cmd: it refuses an invalid --output or correlation ID, builds the request from the flags and args, asked from where the command line stands, opens the use case on the project's directory, asks the subject the request leaves open and reads, then lets the user pick of the read when Pick is set, each only when the user can see and answer it; it prints how the read ended and returns the failure that sets the exit code.
func (q *Query[R, Q, O]) Launch(cmd *cobra.Command, args []string) error {
	if err := q.io.Begin(cmd); err != nil {
		return err
	}

	req := domain.CalledBy(folded[R](fieldAnswers(cmd, args, q.schema())), q.io.Env.Caller())

	ctx := cmd.Context()
	app, err := q.Open(ctx, q.io.Env.ProjectDir)
	if err != nil {
		return q.io.FailWith(domain.AsError(domain.AsErrorParams{Err: err}), nil)
	}

	req, err = q.ask(ctx, app, req, "")
	if err != nil {
		var none O
		return q.conclude(none, req, err)
	}

	opts := domain.RunOptions{Surface: domain.SurfaceCLI, CorrelationID: q.io.Env.CorrelationID}
	read, err := app.Run(ctx, req, opts)
	// A subject refused as ambiguous is asked again among what it matches, then read.
	if refused := ambiguous(err); refused != "" && q.interactive() {
		if req, err = q.ask(ctx, app, req, refused); err == nil {
			read, err = app.Run(ctx, req, opts)
		}
	}
	if err == nil && q.Pick != nil && q.interactive() {
		read, err = q.Pick(ctx, read, req, q.io)
	}

	return q.conclude(read, req, err)
}

// ask asks the subject of the read, the positional select its questions leave open, before the read: when the user can see and answer it, and the request neither answers it nor rules it out, or answers the field at refused with a value refused, asked again among the options its questions narrow it to.
func (q *Query[R, Q, O]) ask(ctx context.Context, app Reader[R, Q, O], req R, refused string) (R, error) {
	// The schema says whether the request leaves a subject to ask before the questions are read.
	if _, open := subjectOf(q.schema(), req, refused); !open || !q.interactive() {
		return req, nil
	}

	questions, err := app.Questions(ctx, req)
	if err != nil {
		return req, err
	}
	subject, open := subjectOf(wordedForm(questions, q.Words), req, refused)
	if !open {
		return req, nil
	}

	if err := Walk(ctx, SubjectScript(subject), q.io.Streams.In, q.io.Streams.Err); err != nil {
		return req, err
	}
	return req.Answer(subject.Field.Path, []string{subject.Picked()})
}

// A Skipper is a request that says why it leaves a field unasked, empty when it does not: what the field's Skip says once its questions are read.
type Skipper interface {
	Skips(path string) domain.Skip
}

// subjectOf returns the positional select of form req leaves open, and reports whether there is one: not ruled out, by the form or by req, and not answered, or the field at refused, whose value was refused ("" refuses none).
func subjectOf(form []worded, req domain.Valued, refused string) (*Subject, bool) {
	skipper, skips := req.(Skipper)
	for _, field := range form {
		subject, ok := field.Field.(domain.SelectField)
		if skips && ok && skipper.Skips(subject.Path) != "" {
			continue
		}
		if ok && positional(field) && subject.Skip == "" && (req.Values(subject.Path) == nil || subject.Path == refused) {
			return &Subject{Field: subject, Words: field.Words}, true
		}
	}

	return nil, false
}

// ambiguous is the path of the field err refuses as ambiguous, "" when it refuses none so.
func ambiguous(err error) string {
	failure, ok := errors.AsType[*domain.Error](err)
	if !ok {
		return ""
	}
	for _, problem := range failure.Fields {
		if problem.Code == domain.CodeAmbiguous {
			return problem.Path
		}
	}
	return ""
}

// interactive reports whether the user can see and answer a picker, and see what it ends on: stdout too is a terminal, since a read piped elsewhere is asked for its data, unless the read is Captured, whose data is what the user picks; the output is for a human and --quiet does not silence it.
func (q *Query[R, Q, O]) interactive() bool {
	return q.io.Streams.CanPrompt() && (q.Captured || q.io.Streams.Out.IsTerminal()) && q.io.Human() && !q.io.Quiet
}

// conclude prints how the read ended — the JSON envelope, a refusal, the read in a format of Formats, the follow-ups the user picked, or what Draw draws of the read — and returns the failure that sets the exit code.
func (q *Query[R, Q, O]) conclude(read O, req R, err error) error {
	failure := domain.AsError(domain.AsErrorParams{Err: err})
	doc := read.Envelope(domain.EnvelopeParams{Command: q.io.Command, Err: failure})
	switch {
	case q.io.Output == uicli.OutputJSON:
		return q.io.Envelope(doc)
	case failure != nil:
		return q.io.FailWith(failure, nil)
	}

	if draw, ok := q.Formats[q.io.Output]; ok {
		draw(q.io.Streams.Out, read)
		return q.io.Failed(read.Failure())
	}

	if q.Captured {
		q.Draw(q.io.Streams.Out, read, req, q.io.View())
		if failure := read.Failure(); failure != nil {
			return q.io.FailWith(failure, nil)
		}
		return nil
	}

	stdout, _ := q.io.Writers()
	// todo: run the follow-up the user picked, in this process, once every command it names has migrated; it is printed until then.
	if len(doc.FollowUps) > 0 {
		output.Frame(stdout, func(w io.Writer) { output.NextSteps(w, q.nextSteps(doc.FollowUps)) })
		return q.io.Failed(read.Failure())
	}

	output.Frame(stdout, func(w io.Writer) { q.Draw(w, read, req, q.io.View()) })
	return q.io.Failed(read.Failure())
}

func (q *Query[R, Q, O]) nextSteps(followUps []domain.FollowUp) []output.NextStep {
	steps := make([]output.NextStep, 0, len(followUps))
	for _, followUp := range followUps {
		steps = append(steps, output.FollowUpStep(followUp, q.io.CommandLine))
	}

	return steps
}

// ----------------------------------------------------------------------------
// The subject

// A Subject is the step that asks a read's subject, its Field in its Words, before the read: a select list of its options, the cursor on the field's value.
type Subject struct {
	Field  domain.SelectField
	Words  Words
	list   *widget.SelectList
	picked string
}

// SubjectScript is the walk that asks s alone, with nothing to read: the subject's options are the questions read before it.
func SubjectScript(s *Subject) Script {
	return Script{
		Steps:   []Step{s},
		Observe: func(context.Context) (func(), error) { return func() {}, nil },
		Label:   func(path string) string { return path },
	}
}

// Label is the field's label.
func (s *Subject) Label() string { return s.Words.Label }

// Description is the field's help.
func (s *Subject) Description() string { return s.Words.Help }

// Standing is the subject picked, none until it is: the step is always asked.
func (s *Subject) Standing() Standing { return Standing{Value: s.picked} }

// Open builds the list of the field's options on its value.
func (s *Subject) Open() tea.Cmd {
	s.list = widget.NewSelectList(widget.SelectParams{Options: Options(s.Field.Options, s.Words), Start: s.Field.Value})
	return nil
}

// Widget is the select list.
func (s *Subject) Widget() widget.Widget { return s.list }

// Answer records the option selected.
func (s *Subject) Answer() string {
	s.picked = s.list.Value()
	return shortOf(s.Words, s.picked)
}

// Reset forgets the subject picked.
func (s *Subject) Reset() { s.picked = "" }

// Picked is the value of the option picked.
func (s *Subject) Picked() string { return s.picked }
