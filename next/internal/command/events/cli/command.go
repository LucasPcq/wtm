// Package cli is the command line of wtm events: its flags, and each line of the stream drawn for a person or written as JSON, over events' use case.
package cli

import (
	"context"
	"errors"
	"io"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	eventsShort = "Stream worktree changes as they happen, in one repository or all of them"
	eventsLong  = "Print the repository's worktrees, then every change made to them, whoever made it:\n" +
		"a command in another shell, an agent, an editor. The stream opens on a snapshot of\n" +
		"every worktree and a ready line, then carries one event per change: worktree.created,\n" +
		"worktree.updated, worktree.provisioned, operation.started, unit.finished and\n" +
		"operation.finished, and repo.added and repo.removed as repositories come and go.\n" +
		"A person reads the worktree and repository changes; with --output json each line is\n" +
		"one JSON object (JSON Lines), every event, the contract an integration reads.\n\n" +
		"If the event bus stops, the stream starts it again and opens on a fresh snapshot:\n" +
		"treat every event as an upsert keyed by branch, and every snapshot as a reset. It runs\n" +
		"until interrupted or until the reader of its output goes away, and exits 0 then.\n\n" +
		"With --all it follows every repository wtm was used in, whatever the current\n" +
		"directory or $GIT_DIR, as it does when run outside any repository without --repo:\n" +
		"a snapshot of each, one ready line, a new repository's snapshot right after its\n" +
		"repo.added.\n\n" +
		"It ends on a code no retry can change: 2 for --all with --repo or a --repo that is no\n" +
		"directory, 12 in a repository wtm was never initialized in or once it no longer is,\n" +
		"19 when interrupted before its first line, 20 on an event of a schema newer than its\n" +
		"own, 21 when --repo is not in a git repository."
	eventsExample = `  # Watch this repository's worktrees
  wtm events

  # The JSON Lines contract, filtered with jq
  wtm events --output json | jq -c 'select(.type == "worktree.created")'

  # Another repository than the current one
  wtm events --repo ~/code/app --output json

  # Every repository wtm knows, wherever it runs
  wtm events --all --output json`
)

// stream is events' form: flags only.
type stream = form.Stream[events.Request, events.Questions]

// eventsFlags spells the failures.
var eventsFlags = stream{}.Flags()

type eventsCommand struct {
	io      uicli.Run
	app     events.UseCase
	started bool // a line was printed
}

// New returns the events command over app: its flags become an events.Request, watched from the project's directory, each line drawn on streams for a person or written as JSON with --output json.
func New(streams *output.Streams, env uicli.Env, app events.UseCase) *cobra.Command {
	// exits are the exit codes of events' own codes, ahead of those of their kind.
	exits := map[string]int{events.CodeConfigNotFound: uicli.ExitConfigNotFound, events.CodeConfigInvalid: uicli.ExitError, events.CodeSchemaNewer: uicli.ExitStreamNewer}
	c := &eventsCommand{io: uicli.Run{Command: events.Name, Streams: streams, Env: env, Flags: eventsFlags, Exits: exits}, app: app}
	cmd := &cobra.Command{Use: events.Name, Short: eventsShort, Long: eventsLong, Example: eventsExample, Args: cobra.NoArgs, RunE: c.run}
	stream{}.Bind(cmd, &c.io)

	return cmd
}

// run streams until interrupted, the reader gone or a failure: it exits 0 once the stream started and ends by an interrupt or its reader leaving.
func (c *eventsCommand) run(cmd *cobra.Command, args []string) error {
	if err := c.io.Begin(cmd); err != nil {
		return err
	}
	uicli.FailOnBrokenPipe()

	req := stream{}.Request(cmd, args)
	req.Dir, req.Named = c.io.Env.ProjectDir, c.io.Env.Named

	// The reader leaving ends the stream as an interrupt would: wtm events | head -n 2 returns as soon as head does.
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

	err := c.app.Watch(ctx, req, events.Out{Emit: c.emit, Warn: c.warn})
	if err == nil || readerLeft(gone, err) {
		return nil
	}
	return c.fail(domain.AsError(domain.AsErrorParams{Err: err}))
}

// readerLeft reports whether the stream ended because nobody reads it any more.
func readerLeft(gone <-chan struct{}, err error) bool {
	select {
	case <-gone:
		return true
	default:
		return errors.Is(err, syscall.EPIPE)
	}
}

// fail prints failure: before the first line as every command does, the JSON envelope on stdout with --output json; once lines were printed, on stderr, stdout carrying the stream's lines alone.
func (c *eventsCommand) fail(failure *domain.Error) error {
	if !c.started {
		return c.io.FailWith(failure, nil)
	}

	output.Frame(c.io.Streams.Err, func(w io.Writer) {
		output.Failure(w, output.FailureParams{Err: failure, Flags: c.io.Flags})
	})
	return c.io.Failed(failure)
}

// emit writes received as JSON, or draws it for a person; --quiet silences the drawing alone.
func (c *eventsCommand) emit(received events.Received) error {
	c.started = true
	out := c.io.Streams.Out
	switch {
	case !c.io.Human():
		_, err := out.Write(append(received.JSON, '\n'))
		return err
	case c.io.Quiet:
		return nil
	}

	return out.Print(func(w io.Writer) { drawLine(w, received) })
}

// warn prints what kept the stream from a line or a repository for a while, on stderr whatever --quiet says.
func (c *eventsCommand) warn(err error) {
	output.WarningLine(c.io.Streams.Err, err.Error())
}
