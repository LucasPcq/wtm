package form

import (
	"cmp"
	"context"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// A Request is a command's request: it takes the answer of a field at a time and reads each back, spelled the same way.
type Request[R any] interface {
	domain.Answerable[R]
	// Values is the request's answer to the field at path, nil when it leaves the field to its default.
	Values(path string) []string
}

// Questions is a command's request as a form.
type Questions interface {
	Form() domain.Form
}

// An Outcome is how a command's run ended; PlanFingerprint is the fingerprint of the plan it previewed or applied.
type Outcome interface {
	PlanFingerprint() string
}

// A UseCase is what the form calls of a command's use case: the questions of a request, and its run.
type UseCase[R, Q, O any] interface {
	Questions(ctx context.Context, req R) (Q, error)
	Run(ctx context.Context, req R, opts domain.RunOptions) (O, error)
}

// A Command is what a command's cli hands the form: the use case it opens, how its recap draws, and the steps it asks its own way.
// Its flags, its request, its form and its command line are all spelled from its form's schema, the form of zero questions.
type Command[R Request[R], Q Questions, O Outcome] struct {
	YesUsage   string // the usage of --yes, which says what an unattended run requires
	Open       func(ctx context.Context, dir string) (UseCase[R, Q, O], error)
	Plan       func(previewed O, width int, st *output.Styles) string // the plan the recap draws
	ApplyLabel func(previewed O) string                               // the recap's option that applies, nil for "Yes, apply"
	// Special returns the step a command asks its own way for the field at, nil for the step of the field's kind.
	Special func(s *Session[R, Q, O], at int, field domain.Field) Step

	io     *uicli.Run
	yes    bool
	dryRun bool
	expect string
}

// Ran is how a run went and the request it ran, which the wizard's answers may have changed.
type Ran[R, O any] struct {
	Request R
	Outcome O
}

func (c *Command[R, Q, O]) schema() domain.Form {
	var zero Q
	return zero.Form()
}

// Flags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
func (c *Command[R, Q, O]) Flags() output.Flags {
	flags, _ := c.schema().Flags()
	return flags
}

// ----------------------------------------------------------------------------
// The flags

// Bind registers on cmd a flag per field of the schema, by its kind, with its usage and the values it accepts, then the flags every command shares: --remember when a field is rememberable, --ask, --yes, --output, --dry-run and --expect-fingerprint; run is where the command prints its run.
func (c *Command[R, Q, O]) Bind(cmd *cobra.Command, run *uicli.Run) {
	c.io = run
	flags := cmd.Flags()
	_, usages := c.schema().Flags()

	for _, field := range c.bound() {
		name, usage := flagName(field), usages[field.Spec().Path]
		switch field := field.(type) {
		case domain.ConfirmField:
			flags.Bool(name, false, usage)
		case domain.MultiSelectField, domain.TextListField:
			flags.StringSlice(name, nil, usage)
		case domain.SelectField:
			flags.String(name, "", usage)
			if accepted := accepting(field.Options); len(accepted) > 0 {
				_ = cmd.RegisterFlagCompletionFunc(name, cobra.FixedCompletions(accepted, cobra.ShellCompDirectiveNoFileComp))
			}
		case domain.TextField:
			flags.String(name, "", usage)
		}
	}

	if usage, rememberable := usages[domain.PathRemember]; rememberable {
		flags.StringArray(domain.PathRemember, nil, usage)
	}
	flags.Bool(domain.PathAsk, false, uicli.AskUsage)
	flags.BoolVarP(&c.yes, "yes", "y", false, c.YesUsage)
	flags.StringVar(&run.Output, "output", uicli.OutputText, uicli.OutputUsage)
	flags.BoolVarP(&c.dryRun, "dry-run", "n", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, "expect-fingerprint", "", uicli.ExpectUsage)
}

// bound are the fields a flag sets, the first of each path, in the order of the form.
func (c *Command[R, Q, O]) bound() []domain.Field {
	var fields []domain.Field
	seen := map[string]bool{}
	for _, field := range c.schema().Fields {
		path := field.Spec().Path
		if seen[path] || positional(field) {
			continue
		}
		seen[path] = true
		fields = append(fields, field)
	}

	return fields
}

func positional(field domain.Field) bool {
	return strings.HasPrefix(field.Spec().Flag, "<")
}

func flagName(field domain.Field) string {
	return strings.TrimPrefix(field.Spec().Flag, "--")
}

// accepting are the values of options, those a flag accepts, an option with no value excepted.
func accepting(options []domain.Option) []string {
	var accepted []string
	for _, o := range options {
		if o.Value != "" {
			accepted = append(accepted, o.Value)
		}
	}

	return accepted
}

// request is the request cmd's flags and args give: each flag the user set answers its field, and the arguments the positional field.
func (c *Command[R, Q, O]) request(cmd *cobra.Command, args []string) R {
	var answers []domain.Answered
	for _, field := range c.schema().Fields {
		if positional(field) && len(args) > 0 {
			answers = append(answers, domain.Answered{Path: field.Spec().Path, Values: arguments(field, args)})
			break
		}
	}

	flags := cmd.Flags()
	for _, field := range c.bound() {
		name := flagName(field)
		if !flags.Changed(name) {
			continue
		}

		var values []string
		switch field.(type) {
		case domain.ConfirmField:
			on, _ := flags.GetBool(name)
			values = []string{strconv.FormatBool(on)}
		case domain.MultiSelectField, domain.TextListField:
			values, _ = flags.GetStringSlice(name)
		case domain.SelectField, domain.TextField:
			value, _ := flags.GetString(name)
			values = []string{value}
		}
		answers = append(answers, domain.Answered{Path: field.Spec().Path, Values: values})
	}

	if flags.Changed(domain.PathRemember) {
		remember, _ := flags.GetStringArray(domain.PathRemember)
		answers = append(answers, domain.Answered{Path: domain.PathRemember, Values: remember})
	}
	if ask, _ := flags.GetBool(domain.PathAsk); flags.Changed(domain.PathAsk) {
		answers = append(answers, domain.Answered{Path: domain.PathAsk, Values: []string{strconv.FormatBool(ask)}})
	}

	// A flag holds a value of its field's own type, which the request always takes.
	var req R
	req, _ = domain.Fold(req, answers)
	return req
}

// arguments are the words given to the positional field, trimmed: every one for a list, the first otherwise.
func arguments(field domain.Field, args []string) []string {
	words := make([]string, 0, len(args))
	for _, arg := range args {
		words = append(words, strings.TrimSpace(arg))
	}

	switch field.(type) {
	case domain.TextListField, domain.MultiSelectField:
		return words
	case domain.SelectField, domain.TextField, domain.ConfirmField:
	}

	return words[:1]
}

// ----------------------------------------------------------------------------
// The launch

// Launch runs cmd: it refuses an invalid --output or correlation ID, builds the request from the flags and args, opens the use case on the project's directory, then runs the request in the wizard when the user can see and answer it, else unattended, with the progress of its operation; conclude prints how the run ended and returns the failure that sets the exit code.
func (c *Command[R, Q, O]) Launch(cmd *cobra.Command, args []string, conclude func(cmd *cobra.Command, ran Ran[R, O], err error) error) error {
	if err := c.io.Begin(cmd); err != nil {
		return err
	}

	req := c.request(cmd, args)

	ctx := cmd.Context()
	app, err := c.Open(ctx, c.io.Env.ProjectDir)
	if err != nil {
		return c.io.Refuse(domain.AsError(domain.AsErrorParams{Err: err}), c.Argv(cmd.CommandPath(), req))
	}

	progress := c.io.Progress(ctx)
	ran, err := c.launch(ctx, app, req, progress.Report)
	progress.Close()

	return conclude(cmd, ran, err)
}

func (c *Command[R, Q, O]) launch(ctx context.Context, app UseCase[R, Q, O], req R, report domain.Report) (Ran[R, O], error) {
	if c.interactive() {
		return c.Ask(ctx, AskParams[R, Q, O]{App: app, Request: req, Options: domain.RunOptions{DryRun: c.dryRun, CorrelationID: c.io.Env.CorrelationID, Report: report}, In: c.io.Streams.In, Out: c.io.Streams.Err})
	}

	outcome, err := app.Run(ctx, req, domain.RunOptions{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: uicli.SurfaceCLI, CorrelationID: c.io.Env.CorrelationID, Report: report})
	return Ran[R, O]{Outcome: outcome, Request: req}, err
}

// interactive reports whether the wizard runs: the user can see and answer it, the output is for a human, and neither --yes nor a previewed plan to apply asks for an unattended run.
func (c *Command[R, Q, O]) interactive() bool {
	return c.io.Streams.CanPrompt() && c.expect == "" && c.io.Human() && !c.yes
}

// AskParams is what the wizard asks: the session's use case, request and options, the keys it reads and the stream it draws on.
type AskParams[R, Q, O any] struct {
	App     UseCase[R, Q, O]
	Request R
	Options domain.RunOptions
	In      io.Reader
	Out     *output.Stream
}

// Ask runs the wizard on the terminal through to the apply; it returns the request as answered and its outcome, with [Run]'s error: a cancel, a failed read of the repository, or the refusal of the run.
func (c *Command[R, Q, O]) Ask(ctx context.Context, params AskParams[R, Q, O]) (Ran[R, O], error) {
	s := c.Session(ctx, SessionParams[R, Q, O]{App: params.App, Request: params.Request, Options: params.Options})
	err := Run(ctx, s.Params(), params.In, params.Out)

	return Ran[R, O]{Outcome: s.outcome, Request: s.req}, err
}

// ----------------------------------------------------------------------------
// The command line

// Argv spells command and req word by word, unquoted: the positional field's words, each flag given sorted by name, then --ask.
func (c *Command[R, Q, O]) Argv(command string, req R) []string {
	argv := strings.Fields(command)
	for _, field := range c.schema().Fields {
		if positional(field) {
			argv = append(argv, req.Values(field.Spec().Path)...)
			break
		}
	}

	type given struct {
		name  string
		words []string
	}
	var flags []given
	for _, field := range c.bound() {
		flags = append(flags, given{name: flagName(field), words: flagWords(field, req.Values(field.Spec().Path))})
	}

	remember := domain.TextListField{FieldSpec: domain.FieldSpec{Flag: "--" + domain.PathRemember}}
	flags = append(flags, given{name: domain.PathRemember, words: flagWords(remember, req.Values(domain.PathRemember))})
	slices.SortFunc(flags, func(a, b given) int { return cmp.Compare(a.name, b.name) })
	for _, flag := range flags {
		argv = append(argv, flag.words...)
	}

	if slices.Equal(req.Values(domain.PathAsk), []string{"true"}) {
		argv = append(argv, "--"+domain.PathAsk)
	}

	return argv
}

// flagWords are the words that give field its values: a switch as --name or --name=false, a list as one flag per value, any other as the flag then its value.
func flagWords(field domain.Field, values []string) []string {
	name := "--" + flagName(field)
	var words []string
	for _, value := range values {
		switch field.(type) {
		case domain.ConfirmField:
			if value == "true" {
				words = append(words, name)
			} else {
				words = append(words, name+"="+value)
			}
		case domain.SelectField, domain.TextField, domain.MultiSelectField, domain.TextListField:
			words = append(words, name, value)
		}
	}

	return words
}
