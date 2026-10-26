// Package form is a command's form, generic over its schema: [Command] (its flags, its request, its launch, the conclusion of its run, its command line), the session and a step per field kind, and the wizard that walks them to a recap and applies; the inputs a step asks with are ui/widget's.
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

// An Outcome is how a command's run ended, as [domain.Outcome] reports it: the fingerprint of the plan it previewed or applied, whether it applied it, the failure the apply ended on and the subject it stopped at, and the envelope of the run.
type Outcome interface {
	PlanFingerprint() string
	Applied() bool
	Failure() *domain.Error
	FailedAt() string
	Envelope(params domain.EnvelopeParams) domain.Envelope
}

// A UseCase is what the form calls of a command's use case: the questions of a request, and its run.
type UseCase[R, Q, O any] interface {
	Questions(ctx context.Context, req R) (Q, error)
	Run(ctx context.Context, req R, opts domain.RunOptions) (O, error)
}

// A Command is what a command's cli hands the form: the use case it opens, how it words its fields, what its recap and its dry run show of a plan, how it draws an apply, and the steps it asks its own way.
// Its flags, its request, its form and its command line are all spelled from its form's schema, the form of zero questions, worded by Words.
type Command[R Request[R], Q Questions, O Outcome] struct {
	YesUsage   string // the usage of --yes, which says what an unattended run requires
	Words      Worder[Q]
	Open       func(ctx context.Context, dir string) (UseCase[R, Q, O], error)
	Preview    func(previewed O) Preview                      // what the recap and the dry run show of the plan previewed
	Draw       func(w io.Writer, applied O, view output.View) // the outcome of an apply, drawn from view
	ApplyLabel func(previewed O) string                       // the recap's option that applies, nil for "Yes, apply"
	// Special returns the step a command asks its own way for the field at, nil for the step of the field's kind.
	Special func(s *Session[R, Q, O], at int, field domain.Field) Step
	// Skipped words why a step is not asked the command's own way, empty for the wizard's words.
	Skipped func(s *Session[R, Q, O], skip domain.Skip) string
	// NewProgress draws the events of a run its own way, nil for [output.NewProgress], which draws one unit at a time.
	NewProgress func(ctx context.Context, params output.ProgressParams) Progress
	// Unattended refuses a request a run without the wizard cannot take as it is, nil for none: a default pick the wizard offers is never an unattended answer.
	Unattended func(req R) error

	io     *uicli.Run
	yes    bool
	dryRun bool
	expect string
}

// A Progress draws the events of a run as they come, until Close.
type Progress interface {
	Report(event domain.Event)
	Close()
}

// A Preview is what the recap and the dry run show of a plan not yet applied: its rows, each answer with its mark, its warnings and the fields answered from memory, which the recap's hint names.
type Preview struct {
	Rows     []output.PlanRow
	Warnings []domain.Warning
	Recalled []string
}

// PreviewOf is what the recap and the dry run show of a mutation's plan previewed: its recap, the run's warnings and the fields the plan recalled.
func PreviewOf[P output.RecappedPlan, D any](previewed domain.Outcome[P, D]) Preview {
	recapped := previewed.Plan.Recaps()
	return Preview{Rows: output.RecapRows(recapped.Recap), Warnings: previewed.Warnings, Recalled: recapped.Recalled}
}

// text is the preview as the recap draws it, in width columns.
func (p Preview) text(width int, st *output.Styles) string {
	return strings.Join(output.PlanLines(output.PlanParams{Rows: p.Rows, Warnings: output.Messages(p.Warnings), Hint: output.RecalledHintOf(p.Recalled), Width: width, Styles: st}), "\n")
}

// Opener is open, a command's opener of its own use case, as a [Command]'s Open.
func Opener[R Request[R], Q Questions, O Outcome, U UseCase[R, Q, O]](open func(ctx context.Context, dir string) (U, error)) func(ctx context.Context, dir string) (UseCase[R, Q, O], error) {
	return func(ctx context.Context, dir string) (UseCase[R, Q, O], error) {
		return open(ctx, dir)
	}
}

// Ran is how a run went and the request it ran, which the wizard's answers may have changed.
type Ran[R, O any] struct {
	Request R
	Outcome O
}

func (c *Command[R, Q, O]) schema() []worded {
	return wordedSchema(c.Words)
}

// form is the schema as data, the form of zero questions.
func (c *Command[R, Q, O]) form() domain.Form {
	var zero Q
	return zero.Form()
}

// Flags spells each field path of the request as the argument or flag that sets it, for the failures and the wizard.
func (c *Command[R, Q, O]) Flags() output.Flags {
	flags, _ := flagsOf(c.schema())
	return flags
}

// ----------------------------------------------------------------------------
// The flags

// Bind registers on cmd a flag per field of the schema, by its kind, with its usage and the values it accepts, then the flags every command shares: --remember when a field is rememberable, --ask, --yes, --output, --dry-run and --expect-fingerprint; run is where the command prints its run, its follow-ups spelled by [Command.spell] over the command line it was handed.
func (c *Command[R, Q, O]) Bind(cmd *cobra.Command, run *uicli.Run) {
	c.io = run
	run.CommandLine = c.spell(cmd, run.CommandLine)
	usages := bindFields(cmd, c.schema())

	flags := cmd.Flags()
	if usage, rememberable := usages[domain.PathRemember]; rememberable {
		flags.StringArray(domain.PathRemember, nil, usage)
	}
	flags.Bool(domain.PathAsk, false, uicli.AskUsage)
	flags.BoolVarP(&c.yes, "yes", "y", false, c.YesUsage)
	flags.StringVar(&run.Output, "output", uicli.OutputText, uicli.OutputUsage)
	flags.BoolVarP(&c.dryRun, uicli.FlagDryRun, "n", false, uicli.DryRunUsage)
	flags.StringVar(&c.expect, uicli.FlagExpectFingerprint, "", uicli.ExpectUsage)
}

// bindFields registers on cmd a flag per field of schema, by its kind, with its usage and the values it accepts, and returns the usage of each path.
func bindFields(cmd *cobra.Command, schema []worded) map[string]string {
	flags := cmd.Flags()
	_, usages := flagsOf(schema)

	for _, field := range bound(schema) {
		name, usage := flagName(field), usages[field.Spec().Path]
		switch field := field.Field.(type) {
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

	return usages
}

// bound are the fields of schema a flag sets, the first of each path, in the order of the form.
func bound(schema []worded) []worded {
	var fields []worded
	seen := map[string]bool{}
	for _, field := range schema {
		path := field.Spec().Path
		if seen[path] || positional(field) || trailing(field) {
			continue
		}
		seen[path] = true
		fields = append(fields, field)
	}

	return fields
}

func positional(field worded) bool {
	return strings.HasPrefix(field.Flag, "<")
}

// trailing reports whether field is spelled after --, the rest of the command line as one value (exec's -- <command>).
func trailing(field worded) bool {
	return strings.HasPrefix(field.Flag, dash+" ")
}

const dash = "--"

func flagName(field worded) string {
	return strings.TrimPrefix(field.Flag, "--")
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

// request is the request cmd's flags and args give, asked from where the command line stands: each flag the user set answers its field, and the arguments the positional field.
func (c *Command[R, Q, O]) request(cmd *cobra.Command, args []string) R {
	answers := fieldAnswers(cmd, args, c.schema())

	flags := cmd.Flags()
	if flags.Changed(domain.PathRemember) {
		remember, _ := flags.GetStringArray(domain.PathRemember)
		answers = append(answers, domain.Answered{Path: domain.PathRemember, Values: remember})
	}
	if ask, _ := flags.GetBool(domain.PathAsk); flags.Changed(domain.PathAsk) {
		answers = append(answers, domain.Answered{Path: domain.PathAsk, Values: []string{strconv.FormatBool(ask)}})
	}

	return domain.CalledBy(folded[R](answers), c.io.Env.Caller())
}

// folded is the request answers give; a flag holds a value of its field's own type, which the request always takes.
func folded[R Request[R]](answers []domain.Answered) R {
	var req R
	req, _ = domain.Fold(req, answers)
	return req
}

// fieldAnswers are the answers cmd's flags and args give the fields of schema: each flag the user set answers its field, and the arguments the positional field.
func fieldAnswers(cmd *cobra.Command, args []string, schema []worded) []domain.Answered {
	var answers []domain.Answered
	var after []string
	if at := cmd.ArgsLenAtDash(); at >= 0 && slices.ContainsFunc(schema, trailing) {
		args, after = args[:at], args[at:]
	}
	for _, field := range schema {
		if positional(field) && len(args) > 0 {
			answers = append(answers, domain.Answered{Path: field.Spec().Path, Values: arguments(field.Field, args)})
			break
		}
	}
	for _, field := range schema {
		if rest := strings.TrimSpace(strings.Join(after, " ")); trailing(field) && rest != "" {
			answers = append(answers, domain.Answered{Path: field.Spec().Path, Values: []string{rest}})
			break
		}
	}

	flags := cmd.Flags()
	for _, field := range bound(schema) {
		name := flagName(field)
		if !flags.Changed(name) {
			continue
		}

		var values []string
		switch field.Field.(type) {
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

	return answers
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
		return c.io.FailWith(domain.AsError(domain.AsErrorParams{Err: err}), nil)
	}

	progress := c.progress(ctx)
	ran, err := c.launch(ctx, app, req, progress.Report)
	progress.Close()

	return conclude(cmd, ran, err)
}

// progress is the progress of the run ctx runs: the command's own when it draws one, else output's, one unit at a time; only the one built watches for a cancel.
func (c *Command[R, Q, O]) progress(ctx context.Context) Progress {
	if c.NewProgress != nil {
		return c.NewProgress(ctx, c.io.ProgressParams())
	}
	return output.NewProgress(ctx, c.io.ProgressParams())
}

func (c *Command[R, Q, O]) launch(ctx context.Context, app UseCase[R, Q, O], req R, report domain.Report) (Ran[R, O], error) {
	if c.interactive() {
		return c.Ask(ctx, AskParams[R, Q, O]{App: app, Request: req, Options: domain.RunOptions{DryRun: c.dryRun, CorrelationID: c.io.Env.CorrelationID, Report: report}, In: c.io.Streams.In, Out: c.io.Streams.Err})
	}

	if c.Unattended != nil {
		if err := c.Unattended(req); err != nil {
			return Ran[R, O]{Request: req}, err
		}
	}

	outcome, err := app.Run(ctx, req, domain.RunOptions{DryRun: c.dryRun, ExpectFingerprint: c.expect, Surface: domain.SurfaceCLI, CorrelationID: c.io.Env.CorrelationID, Report: report})
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
	s := c.Session(ctx, params.App, params.Request, params.Options)
	err := Run(ctx, s.Script(), params.In, params.Out)

	return Ran[R, O]{Outcome: s.outcome, Request: s.req}, err
}

// ----------------------------------------------------------------------------
// The conclusion

// Conclude prints how the run ended — the JSON envelope, a refusal, the dry run and the line that applies it, or the outcome Draw draws — and returns the failure that sets the exit code.
func (c *Command[R, Q, O]) Conclude(cmd *cobra.Command, ran Ran[R, O], err error) error {
	failure := domain.AsError(domain.AsErrorParams{Err: err})
	if !c.io.Human() {
		return c.io.Envelope(ran.Outcome.Envelope(domain.EnvelopeParams{Command: c.io.Command, Err: failure, Answers: domain.AnswersOf(c.form(), ran.Request)}))
	}

	argv := c.Argv(cmd.CommandPath(), ran.Request)

	stdout, _ := c.io.Writers()
	switch {
	case failure != nil:
		return c.io.FailWith(c.replanned(failure, ran.Request), nil)
	case !ran.Outcome.Applied():
		previewed, fingerprint := c.Preview(ran.Outcome), ran.Outcome.PlanFingerprint()
		output.Frame(stdout, func(w io.Writer) {
			output.DryRun(w, output.DryRunParams{Command: c.io.Command, Rows: previewed.Rows, Warnings: previewed.Warnings, Fingerprint: fingerprint, Apply: applyLine(argv, fingerprint), CommandLine: c.io.CommandLine})
		})
		return nil
	}

	output.Frame(stdout, func(w io.Writer) { c.Draw(w, ran.Outcome, c.io.View()) })
	return c.io.Failed(c.quietlyNamed(ran.Outcome))
}

// quietlyNamed is the failure an apply ended on, led by the subject it stopped at when --quiet prints it alone and it does not name it already: the outcome that names each subject is not drawn.
func (c *Command[R, Q, O]) quietlyNamed(outcome O) *domain.Error {
	failure, subject := outcome.Failure(), outcome.FailedAt()
	if failure == nil || subject == "" || !c.io.Quiet || strings.Contains(failure.Message, subject) {
		return failure
	}

	named := *failure
	named.Message = subject + ": " + failure.Message
	return &named
}

// replanned is failure followed by the request run again as a dry run when it is a stale plan, as the envelope's follow_ups give it; failure itself otherwise.
func (c *Command[R, Q, O]) replanned(failure *domain.Error, req R) *domain.Error {
	replan := domain.Replan(failure, c.io.Command, domain.AnswersOf(c.form(), req))
	if len(replan) == 0 || failure.FollowUp != nil {
		return failure
	}

	followed := *failure
	followed.FollowUp = &replan[0]
	return &followed
}

// spell is the command line of a follow-up: the injected one, unless the follow-up names this command, which [Command.Spell] spells, never the injected line.
func (c *Command[R, Q, O]) spell(cmd *cobra.Command, injected output.CommandLine) output.CommandLine {
	return func(followUp domain.FollowUp) []string {
		if followUp.Command == "" || followUp.Command != c.io.Command {
			return injected(followUp)
		}

		return c.Spell(cmd.CommandPath(), followUp)
	}
}

// Spell spells followUp, which names this command, as command (its path on the command line) runs it: its request folded and spelled from the schema (Argv), with --dry-run when it asks for one; none when its request refuses the answers.
func (c *Command[R, Q, O]) Spell(command string, followUp domain.FollowUp) []string {
	var zero R
	req, err := domain.Fold(zero, followUp.Answers)
	if err != nil {
		return nil
	}

	argv := c.Argv(command, req)
	if followUp.DryRun {
		argv = withFlags(argv, "--"+uicli.FlagDryRun)
	}
	return argv
}

// applyLine is argv, the command line of the request, held to the fingerprint of the plan it previewed.
func applyLine(argv []string, fingerprint string) string {
	return output.Line(withFlags(argv, "--"+uicli.FlagExpectFingerprint, fingerprint))
}

// withFlags is argv with flags added before its --, after which every word is the trailing field's.
func withFlags(argv []string, flags ...string) []string {
	at := slices.Index(argv, dash)
	if at < 0 {
		return append(slices.Clone(argv), flags...)
	}

	return slices.Concat(argv[:at], flags, argv[at:])
}

// ----------------------------------------------------------------------------
// The command line

// Argv spells command and req word by word, unquoted: the positional field's words, each flag given sorted by name, then --ask, then -- and the trailing field's value.
func (c *Command[R, Q, O]) Argv(command string, req R) []string {
	argv := strings.Fields(command)
	for _, field := range c.schema() {
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
	for _, field := range bound(c.schema()) {
		flags = append(flags, given{name: flagName(field), words: flagWords(field, req.Values(field.Spec().Path))})
	}

	// --remember is a string array, which reads each value as it is, not as CSV.
	remember := worded{Field: domain.SelectField{}, Words: Words{Flag: "--" + domain.PathRemember}}
	flags = append(flags, given{name: domain.PathRemember, words: flagWords(remember, req.Values(domain.PathRemember))})
	slices.SortFunc(flags, func(a, b given) int { return cmp.Compare(a.name, b.name) })
	for _, flag := range flags {
		argv = append(argv, flag.words...)
	}

	if slices.Equal(req.Values(domain.PathAsk), []string{"true"}) {
		argv = append(argv, "--"+domain.PathAsk)
	}
	for _, field := range c.schema() {
		if values := req.Values(field.Spec().Path); trailing(field) && len(values) > 0 {
			argv = append(argv, append([]string{dash}, values...)...)
		}
	}

	return argv
}

// csvField is value as a list flag reads it back as one entry: pflag reads a list flag as a line of CSV, so a value holding a comma, a quote or a newline is quoted the CSV way.
func csvField(value string) string {
	if !strings.ContainsAny(value, ",\"\n") {
		return value
	}

	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// flagWords are the words that give field its values: a switch as --name or --name=false, a list as one flag per value, any other as the flag then its value.
func flagWords(field worded, values []string) []string {
	name := "--" + flagName(field)
	var words []string
	for _, value := range values {
		switch field.Field.(type) {
		case domain.ConfirmField:
			if value == "true" {
				words = append(words, name)
			} else {
				words = append(words, name+"="+value)
			}
		case domain.MultiSelectField, domain.TextListField:
			words = append(words, name, csvField(value))
		case domain.SelectField, domain.TextField:
			words = append(words, name, value)
		}
	}

	return words
}
