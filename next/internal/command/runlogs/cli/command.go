// Package cli is run logs' command line: its flags, its worktree picker, the live view of the jobs at a terminal, their lines elsewhere, followed with --follow, and its JSON, over run logs' use case.
package cli

import (
	"context"
	"errors"
	"io"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// Use is how the command line spells run logs after the program's name.
const Use = "run logs"

const (
	logsShort = "Read the output of a worktree's jobs"
	logsLong  = "Show what the jobs of the worktrees named printed: each job's last lines (1000 at most), from\n" +
		"its log, whether it runs, ended or crashed. --job reads one job alone.\n" +
		"\n" +
		"At a terminal it opens the live view on their jobs, after the worktree picker when none is\n" +
		"named (the one you stand in picked; --yes skips the picker for it). Leaving the view stops\n" +
		"nothing. Anywhere else (no terminal, --quiet, --output json) it reads the worktree you stand\n" +
		"in unless named, prints each job's lines as `[job] text` and ends; --follow (-f) then prints\n" +
		"each new line as the jobs print it, until they all ended or Ctrl+C, which exits 0.\n" +
		"\n" +
		"--search reads each job's whole log, its rotated files included, for the lines holding\n" +
		"a text (--regex, --match-case), its newest 1000 matches; --since reads it from a time.\n" +
		"\n" +
		"--output json is the read's v2 envelope, each job with its state and its lines; with\n" +
		"--follow, one typed JSON object per line instead: line, absent, warning, error.\n" +
		"It never starts the run daemon: with none running, the lines come from the logs."
	logsExample = `  # Pick worktrees, then watch their jobs live
  wtm run logs

  # Every error api ever logged
  wtm run logs feat/login --job api --search 'error|panic' --regex

  # One job's last lines, for a script or an agent
  wtm run logs feat/login --job api --output json

  # Follow a job's output without a terminal
  wtm run logs feat/login --job api --follow | grep ERROR`

	yesUsage = "Ask nothing: read the worktree you stand in unless named"
)

// query is run logs' form: a read, its subjects the worktrees.
type query = form.Query[runlogs.Request, runlogs.Questions, runlogs.Outcome]

// logsFlags spells the failures: the worktrees are positional, the others are flags.
var logsFlags = (&query{Words: words}).Flags()

type logsCommand struct {
	io      uicli.Run
	q       *query
	open    func(ctx context.Context, dir string) (runlogs.UseCase, error)
	view    View
	viewed  bool // the live view was shown: the read is not drawn again under it
	started bool // a line of a follow was printed
}

// New returns run logs' command, a subcommand of run: its arguments and flags become a runlogs.Request, the worktrees picked first when none is named and the user can see and answer the picker; at a terminal it shows the jobs live in view, elsewhere it draws their lines on streams, followed with --follow, or the v2 JSON envelope with --output json, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (runlogs.UseCase, error), commandLine output.CommandLine, view View) *cobra.Command {
	c := &logsCommand{io: uicli.Run{Command: runlogs.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: logsFlags}, open: open, view: view}
	c.q = &query{
		Words:    words,
		YesUsage: yesUsage,
		Open: func(ctx context.Context, dir string) (form.Reader[runlogs.Request, runlogs.Questions, runlogs.Outcome], error) {
			return open(ctx, dir)
		},
		Draw: c.draw,
		Pick: c.show,
	}
	cmd := &cobra.Command{Use: "logs [worktree...]", Short: logsShort, Long: logsLong, Example: logsExample, Args: cobra.ArbitraryArgs, RunE: c.run}
	c.q.Bind(cmd, &c.io)

	return cmd
}

// Line spells a follow-up that names run logs, from run logs' schema, as wtm-next runs it; none when its answers are not run logs'.
func Line(followUp domain.FollowUp) []string {
	return (&query{Words: words}).Spell(uicli.Program+" "+Use, followUp)
}

// run reads, or follows when --follow asks it and no live view does it.
func (c *logsCommand) run(cmd *cobra.Command, args []string) error {
	if err := c.io.Begin(cmd); err != nil {
		return err
	}
	req := form.Stream[runlogs.Request, runlogs.Questions]{Words: words}.Request(cmd, args, c.io.Env)
	if !req.Follow || c.interactive() {
		return c.q.Launch(cmd, args)
	}

	return c.follow(cmd, req)
}

// interactive reports whether the user can see the live view, as the read's picker: stdin, stdout and stderr terminals, human output, no --quiet.
func (c *logsCommand) interactive() bool {
	return c.io.Streams.CanPrompt() && c.io.Streams.Out.IsTerminal() && c.io.Human() && !c.io.Quiet
}

// ----------------------------------------------------------------------------
// The follow

// follow streams the jobs' lines until they ended, an interrupt or the reader leaving, which exit 0.
func (c *logsCommand) follow(cmd *cobra.Command, req runlogs.Request) error {
	uicli.FailOnBrokenPipe()
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	gone := c.io.Streams.Out.Gone(ctx)
	go func() {
		select {
		case <-gone:
			cancel()
		case <-ctx.Done():
		}
	}()

	app, err := c.open(ctx, c.io.Env.ProjectDir)
	if err == nil {
		prefix := prefixer(len(req.Worktrees) > 1)
		err = app.Watch(ctx, req, runlogs.Out{Emit: func(entry runlogs.Entry) error { return c.emit(entry, prefix) }, Absent: c.absent, Ended: c.ended, Warn: c.warn})
	}
	if err == nil || readerLeft(gone, err) {
		return nil
	}
	return c.fail(domain.AsError(domain.AsErrorParams{Err: err}))
}

// emit writes entry as a line of JSON, or draws it for a person; --quiet silences the drawing alone.
func (c *logsCommand) emit(entry runlogs.Entry, prefix func(worktree, job string) string) error {
	return c.write(entry, func(w io.Writer) { drawLine(w, prefix(entry.Worktree, entry.Job), entry.Text) })
}

// absent says a worktree where no job ran, or a job named that never ran there, as the read says it.
func (c *logsCommand) absent(absent runlogs.Absent) error {
	return c.write(absent, func(w io.Writer) { drawAbsent(w, absent.Worktree, absent.Job) })
}

// ended writes a job followed to its end as a line of JSON; a person reads it in the job's last lines.
func (c *logsCommand) ended(ended runlogs.Ended) error {
	if c.io.Human() {
		return nil
	}
	return c.write(ended, nil)
}

// write writes v as a line of JSON, or draws it for a person; --quiet silences the drawing alone.
func (c *logsCommand) write(v any, draw func(io.Writer)) error {
	c.started = true
	out := c.io.Streams.Out
	switch {
	case !c.io.Human():
		return writeLine(out, v)
	case c.io.Quiet:
		return nil
	}

	return out.Print(draw)
}

// warn says what kept lines away: a line of JSON among the lines, else on stderr whatever --quiet says.
func (c *logsCommand) warn(warning domain.Warning) {
	if !c.io.Human() {
		c.started = true
		_ = writeLine(c.io.Streams.Out, warning)
		return
	}
	output.WarningLine(c.io.Streams.Err, warning.Message)
}

// fail prints failure: before the first line as every command does, the JSON envelope on stdout with --output json; once lines were printed, as the stream's last line of JSON, or on stderr, stdout carrying the lines alone.
func (c *logsCommand) fail(failure *domain.Error) error {
	switch {
	case !c.started:
		return c.io.FailWith(failure, nil)
	case !c.io.Human():
		_ = writeLine(c.io.Streams.Out, failure)
		return c.io.Failed(failure)
	}

	output.Frame(c.io.Streams.Err, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Next: output.FollowUpSteps(failure, c.io.CommandLine), Flags: c.io.Flags})
	})
	return c.io.Failed(failure)
}

// readerLeft reports whether the follow ended because nobody reads it any more.
func readerLeft(gone <-chan struct{}, err error) bool {
	select {
	case <-gone:
		return true
	default:
		return errors.Is(err, syscall.EPIPE)
	}
}
