// Package events wires `wtm events`, the stream an editor, an agent or a
// terminal plugin reads instead of guessing what wtm did.
package events

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
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
			"updated, relocated, reparented, removed. With --output json each line is one JSON\n" +
			"object (JSON Lines), the contract an integration reads; its schema ships with wtm.\n" +
			"If the run daemon stops, the stream waits for it and opens again on a fresh\n" +
			"snapshot: treat every event as an upsert keyed by branch, and every snapshot as a\n" +
			"reset. It runs until interrupted, and exits with code 20 if it receives an event of\n" +
			"a schema newer than its own.",
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

	write := writerFor(writerForParams{Cmd: cmd, Format: format})
	return wtmevents.Watch(ctx, wtmevents.WatchParams{
		ProjectDir: cfg.ProjectDir,
		StateDir:   cfg.StateDir,
		OnEvent:    write,
		OnWarning: func(err error) {
			output.Warning(output.Barred(cmd.ErrOrStderr()), err.Error())
		},
	})
}

func repoDir(cmd *cobra.Command) (string, error) {
	repo, _ := cmd.Flags().GetString(domain.FlagRepo)
	if repo == "" {
		return os.Getwd()
	}
	info, err := os.Stat(repo)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf(domain.EventsRepoNotADirFmt, domain.FlagRepo, repo, domain.ErrUsage)
	}
	return repo, nil
}

type writerForParams struct {
	Cmd    *cobra.Command
	Format string
}

// writerFor puts no frame around the stream: it never ends on its own, so
// there is no block to close. Each human line carries the bar on a terminal.
func writerFor(params writerForParams) func(domain.Event) error {
	out := params.Cmd.OutOrStdout()
	if !rules.IsHumanFormat(params.Format) {
		return func(event domain.Event) error { return output.WriteEventJSONLine(out, event) }
	}
	barred := output.Barred(out)
	return func(event domain.Event) error { return output.WriteEventLine(barred, event) }
}
