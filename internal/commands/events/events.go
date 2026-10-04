// Package events wires `wtm events`, the stream an editor, an agent or a
// terminal plugin reads instead of guessing what wtm did.
package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	wtmevents "github.com/LucasPcq/wtm/internal/service/events"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEvents,
		Short: "Stream the repository's worktree changes as they happen",
		Long: "Print the repository's worktrees, then every change made to them, whoever made it:\n" +
			"a command in another shell, an agent, or `wtm ui`. The stream opens on a snapshot of\n" +
			"every worktree and a ready line, then carries one event per change — created,\n" +
			"provisioned, updated, relocated, reparented, deprovisioned, removed. With\n" +
			"--output json each line is one JSON object (JSON Lines), the contract an\n" +
			"integration reads; its schema ships with wtm.\n" +
			"If the run daemon stops, the stream waits for it and opens again on a fresh\n" +
			"snapshot: treat every event as an upsert keyed by branch, and every snapshot as a\n" +
			"reset. It runs until interrupted or until the reader of its pipe goes away. It\n" +
			"ends on a code no retry can change in three cases: 12 in a repository wtm was never\n" +
			"initialized in, 21 outside a git repository, and 20 if it receives an event of a\n" +
			"schema newer than its own.",
		Example: `  # Watch this repository's worktrees
  wtm events

  # The JSON Lines contract, filtered with jq
  wtm events --output json | jq -c 'select(.type == "worktree.created")'

  # Another repository than the current one
  wtm events --repo ~/code/app --output json`,
		Args: cobra.NoArgs,
		RunE: runEvents,
	}
	cmd.Flags().String(domain.FlagRepo, "", "Watch the repository holding this path instead of the current one")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runEvents(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	dir, err := repoDir(cmd)
	if err != nil {
		return err
	}
	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = endWhenUnread(ctx, cmd)

	write := writerFor(writerForParams{Cmd: cmd, Format: format})
	err = wtmevents.Watch(ctx, wtmevents.WatchParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		ProxyPort:  rules.ProxyPort(cfg.Config.Global),
		OnEvent:    write,
		OnWarning: func(err error) {
			output.Warning(output.Barred(cmd.ErrOrStderr()), err.Error())
		},
	})
	// A reader that left between two writes is the poll's case reached first.
	if errors.Is(err, syscall.EPIPE) {
		return nil
	}
	return err
}

// endWhenUnread stops the stream once its reader has gone, as an interrupt
// would: `wtm events | head -n 2` returns as soon as head does.
func endWhenUnread(ctx context.Context, cmd *cobra.Command) context.Context {
	out, ok := cmd.OutOrStdout().(*os.File)
	if !ok {
		return ctx
	}
	ctx, cancel := context.WithCancel(ctx)
	gone := infra.ReaderGone(ctx, infra.ReaderGoneParams{File: out, Every: domain.EventsReaderGoneCheck})
	go func() {
		select {
		case <-gone:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

func repoDir(cmd *cobra.Command) (string, error) {
	repo, _ := cmd.Flags().GetString(domain.FlagRepo)
	if repo == "" {
		return os.Getwd()
	}
	refuse := func(reason string) error {
		return rules.InvalidFlagPath(rules.InvalidFlagPathParams{Flag: domain.FlagRepo, Path: repo, Reason: reason})
	}
	info, err := os.Stat(repo)
	if err != nil || !info.IsDir() {
		return "", refuse(domain.FlagPathNotADirectory)
	}
	inside, err := infra.InsideGitRepo(repo)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf(domain.FlagPathNotGitRepoFmt, domain.FlagRepo, repo, domain.ErrNotGitRepo)
	}
	return repo, nil
}

type writerForParams struct {
	Cmd    *cobra.Command
	Format string
}

// writerFor puts no frame around the stream: it never ends on its own, so
// there is no block to close. Each human line carries the bar on a terminal.
func writerFor(params writerForParams) func(wtmevents.Received) error {
	out := params.Cmd.OutOrStdout()
	if !rules.IsHumanFormat(params.Format) {
		return func(received wtmevents.Received) error { return output.WriteEventJSONLine(out, received.Raw) }
	}
	barred := output.Barred(out)
	return func(received wtmevents.Received) error { return output.WriteEventLine(barred, received.Event) }
}
