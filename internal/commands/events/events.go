// Package events wires `wtm events`, the stream an editor, an agent or a
// terminal plugin reads instead of guessing what wtm did.
package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	wtmevents "github.com/LucasPcq/wtm/internal/service/events"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEvents,
		Short: "Stream worktree changes as they happen, in one repository or all of them",
		Long: "Print the repository's worktrees, then every change made to them, whoever made it:\n" +
			"a command in another shell, an agent, or `wtm ui`. The stream opens on a snapshot of\n" +
			"every worktree and a ready line, then carries one event per change — created,\n" +
			"provisioned, updated, relocated, reparented, deprovisioned, removed. With\n" +
			"--output json each line is one JSON object (JSON Lines), the contract an\n" +
			"integration reads; its schema ships with wtm.\n" +
			"If the run daemon stops, the stream waits for it and opens again on a fresh\n" +
			"snapshot: treat every event as an upsert keyed by branch, and every snapshot as a\n" +
			"reset. It runs until interrupted or until the reader of its pipe goes away.\n" +
			"With --all it follows every repository wtm was used in, whatever the current\n" +
			"directory or $GIT_DIR, as it does when run outside any repository without --repo:\n" +
			"a snapshot for each, one ready line, then repo.added and repo.removed as they\n" +
			"come and go, a new repository's snapshot right after its repo.added.\n" +
			"It ends on a code no retry can change: 2 for --all with --repo, 12 in a\n" +
			"repository wtm was never initialized in, 21 when --repo is not in a git\n" +
			"repository, and 20 if it receives an event of a schema newer than its own.",
		Example: `  # Watch this repository's worktrees
  wtm events

  # The JSON Lines contract, filtered with jq
  wtm events --output json | jq -c 'select(.type == "worktree.created")'

  # Another repository than the current one
  wtm events --repo ~/code/app --output json

  # Every repository wtm knows, wherever it runs
  wtm events --all --output json`,
		Args: cobra.NoArgs,
		RunE: runEvents,
	}
	cmd.Flags().String(domain.FlagRepo, "", "Watch the repository holding this path instead of the current one")
	cmd.Flags().Bool(domain.FlagAll, false, "Watch every repository wtm knows, whatever the current directory or $GIT_DIR")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runEvents(cmd *cobra.Command, _ []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	global, err := followsEveryRepo(cmd)
	if err != nil {
		return err
	}
	if global {
		return stream(streamParams{Cmd: cmd, Format: format, Global: true, Watch: func(ctx context.Context, watch watchHooks) error {
			return wtmevents.WatchAll(ctx, wtmevents.WatchAllParams{
				ProxyPort: rules.ProxyPort(globalConfig()),
				OnEvent:   watch.OnEvent,
				OnWarning: watch.OnWarning,
			})
		}})
	}
	dir, err := repoDir(cmd)
	if err != nil {
		return err
	}
	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}
	return stream(streamParams{Cmd: cmd, Format: format, Watch: func(ctx context.Context, watch watchHooks) error {
		return wtmevents.Watch(ctx, wtmevents.WatchParams{
			ProjectDir: cfg.ProjectDir,
			StateDir:   cfg.StateDir,
			ProxyPort:  rules.ProxyPort(cfg.Config.Global),
			OnEvent:    watch.OnEvent,
			OnWarning:  watch.OnWarning,
		})
	}})
}

// followsEveryRepo is the global stream's trigger: --all, or no --repo and no
// repository around the current directory to default to. Only --all drops the
// inherited git variables: the implicit trigger is a contract integrations
// already rely on, and stays as it shipped.
func followsEveryRepo(cmd *cobra.Command) (bool, error) {
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	repo, _ := cmd.Flags().GetString(domain.FlagRepo)
	if all && repo != "" {
		return false, fmt.Errorf("%w: %w", domain.ErrUsage, domain.ErrEventsAllWithRepo)
	}
	if all {
		return true, forgetRepoScopedGitEnv()
	}
	if repo != "" {
		return false, nil
	}
	if os.Getenv(domain.EnvProjectDir) != "" {
		return false, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	inside, err := infra.InsideGitRepo(cmd.Context(), cwd)
	if err != nil {
		return false, err
	}
	return !inside, nil
}

// forgetRepoScopedGitEnv keeps an inherited $GIT_DIR from answering for every
// repository the stream reads, and out of a daemon it starts.
func forgetRepoScopedGitEnv() error {
	for _, name := range domain.GitRepoScopedEnv {
		if err := os.Unsetenv(name); err != nil {
			return err
		}
	}
	return nil
}

// globalConfig falls back to the defaults: a global config that cannot be read
// changes the port a daemon this stream starts serves, never whether it runs.
func globalConfig() domain.GlobalConfig {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return domain.GlobalConfig{}
	}
	return cfg
}

type watchHooks struct {
	OnEvent   func(wtmevents.Received) error
	OnWarning func(error)
}

type streamParams struct {
	Cmd    *cobra.Command
	Format string
	Global bool
	Watch  func(context.Context, watchHooks) error
}

func stream(params streamParams) error {
	cmd := params.Cmd
	ctx := endWhenUnread(cmd.Context(), cmd)

	err := params.Watch(ctx, watchHooks{
		OnEvent: writerFor(writerForParams{Cmd: cmd, Format: params.Format, Global: params.Global}),
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
	inside, err := infra.InsideGitRepo(cmd.Context(), repo)
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
	Global bool
}

// writerFor puts no frame around the stream: it never ends on its own, so
// there is no block to close. Each human line carries the bar on a terminal.
func writerFor(params writerForParams) func(wtmevents.Received) error {
	out := params.Cmd.OutOrStdout()
	if !rules.IsHumanFormat(params.Format) {
		return func(received wtmevents.Received) error { return output.WriteEventJSONLine(out, received.Raw) }
	}
	barred := output.Barred(out)
	write := output.WriteEventLine
	if params.Global {
		write = output.WriteGlobalEventLine
	}
	return func(received wtmevents.Received) error { return write(barred, received.Event) }
}
