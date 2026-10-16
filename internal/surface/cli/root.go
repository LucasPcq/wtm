// Package cli is the command line: the Cobra tree, its run and its exit code.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
	"github.com/LucasPcq/wtm/internal/surface/cli/agents"
	"github.com/LucasPcq/wtm/internal/surface/cli/checkout"
	"github.com/LucasPcq/wtm/internal/surface/cli/configcmd"
	"github.com/LucasPcq/wtm/internal/surface/cli/daemon"
	"github.com/LucasPcq/wtm/internal/surface/cli/events"
	"github.com/LucasPcq/wtm/internal/surface/cli/initcmd"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/resolve"
	"github.com/LucasPcq/wtm/internal/surface/cli/run"
	"github.com/LucasPcq/wtm/internal/surface/cli/schema"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/surface/cli/shell"
	"github.com/LucasPcq/wtm/internal/surface/cli/ui"
	"github.com/LucasPcq/wtm/internal/surface/cli/upgrade"
	"github.com/LucasPcq/wtm/internal/surface/cli/versioncmd"
	"github.com/LucasPcq/wtm/internal/surface/cli/wt"
)

func init() {
	rootCmd.PersistentFlags().BoolP(domain.FlagQuiet, "q", false,
		"Silence human output; errors and the exit code are unaffected, and --output json still emits its document")

	rootCmd.AddGroup(
		&cobra.Group{ID: domain.CmdGroupWorktrees, Title: domain.CmdGroupWorktreesTitle},
		&cobra.Group{ID: domain.CmdGroupNavigate, Title: domain.CmdGroupNavigateTitle},
		&cobra.Group{ID: domain.CmdGroupStack, Title: domain.CmdGroupStackTitle},
		&cobra.Group{ID: domain.CmdGroupJobs, Title: domain.CmdGroupJobsTitle},
		&cobra.Group{ID: domain.CmdGroupGitHub, Title: domain.CmdGroupGitHubTitle},
		&cobra.Group{ID: domain.CmdGroupSetup, Title: domain.CmdGroupSetupTitle},
	)

	for _, cmd := range wt.NewCmds() {
		rootCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(newCreateCmd(Bind))
	rootCmd.AddCommand(run.NewCmd())

	uiCmd := ui.NewCmd(ui.NewCmdParams{Version: effectiveVersion})
	uiCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(uiCmd)

	eventsCmd := events.NewCmd()
	eventsCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(eventsCmd)

	resolveCmd := resolve.NewCmd()
	resolveCmd.GroupID = domain.CmdGroupNavigate
	rootCmd.AddCommand(resolveCmd)

	checkoutCmd := checkout.NewCmd()
	checkoutCmd.GroupID = domain.CmdGroupGitHub
	rootCmd.AddCommand(checkoutCmd)

	initCmd := initcmd.NewCmd()
	initCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(initCmd)

	shellInitCmd := shell.NewCmd()
	shellInitCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(shellInitCmd)

	configCmd := configcmd.NewCmd()
	configCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(configCmd)

	agentsCmd := agents.NewCmd()
	agentsCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(agentsCmd)

	schemaCmd := schema.NewCmd()
	schemaCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(schemaCmd)

	rootCmd.Version = effectiveVersion

	upgradeCmd := upgrade.NewCmd(upgrade.NewCmdParams{Version: effectiveVersion})
	upgradeCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(upgradeCmd)

	versionCmd := versioncmd.NewCmd(versioncmd.NewCmdParams{Version: effectiveVersion})
	versionCmd.GroupID = domain.CmdGroupSetup
	rootCmd.AddCommand(versionCmd)

	rootCmd.AddCommand(daemon.NewCmd())
	rootCmd.AddCommand(daemon.NewProxyForwardCmd())

	markUsageErrors(rootCmd)
}

// The one symbol goreleaser stamps, and the one the run daemon compares across
// its socket: a second stamped target would let a client and its daemon
// disagree about which build each is.
var version = domain.Version

// A package-level initializer, not init(): Go orders var initialization by
// dependency, so every command built in init already sees the final value.
var effectiveVersion = selfupdate.ResolveVersion(version)

var (
	updateCheck        *selfupdate.Check
	updateCheckStarted bool
)

// The exclusion list names top-level commands: the runnable leaves are
// `completion zsh` and `schema dump`, so cmd.Name() would let both slip through.
func topLevelName(cmd *cobra.Command) string {
	for cmd.HasParent() && cmd.Parent().HasParent() {
		cmd = cmd.Parent()
	}

	return cmd.Name()
}

// Called from PersistentPreRun and from the help function, since cobra answers
// --help before any hook fires; the guard keeps the second caller from
// discarding the check the first started.
func startUpdateCheck(cmd *cobra.Command) {
	if updateCheckStarted {
		return
	}
	updateCheckStarted = true

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	updateCheck = selfupdate.StartCheck(cmd.Context(), selfupdate.StartCheckParams{
		Version:     effectiveVersion,
		Format:      format,
		Command:     topLevelName(cmd),
		StderrIsTTY: term.IsTerminal(int(os.Stderr.Fd())),
		ConfigCheck: globalUpdateCheck,
	})
}

var rootCmd = &cobra.Command{
	Use:     domain.AppName,
	Short:   "Orchestrate git worktrees and team dev workflows from the terminal",
	Version: version,
	Example: `  # Once per repository
  wtm init

  # A worktree per branch, then jump into it
  wtm create feat/login
  wtm go feat/login

  # Every worktree, its PR and its services, on one screen
  wtm ui`,
	RunE: rootRunE,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := validateFlagGroups(cmd); err != nil {
			return err
		}
		if err := validateOutputFormat(cmd); err != nil {
			return err
		}
		if err := validateCorrelationID(); err != nil {
			return err
		}
		startUpdateCheck(cmd)
		silenceHumanOutput(cmd)
		shared.ClearCancelled(cmd)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, _ []string) error {
		if shared.Cancelled(cmd) {
			return fmt.Errorf("%w: %w", domain.ErrAborted, domain.ErrCancelled)
		}
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func rootRunE(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func globalUpdateCheck() *bool {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return nil
	}

	return cfg.Update.Check
}

// A suppressed check leaves updateCheck nil, and Notice is nil-safe.
func printUpdateNotice(ctx context.Context) {
	if quiet, _ := rootCmd.Flags().GetBool(domain.FlagQuiet); quiet {
		return
	}
	current, latest, method, ok := updateCheck.Notice(ctx, domain.UpdateNoticeWait)
	if !ok {
		return
	}

	render.Frame(os.Stderr, func(w io.Writer) {
		render.UpdateNotice(w, render.UpdateNoticeParams{Current: current, Latest: latest, Method: method})
	})
}

func init() {
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		startUpdateCheck(cmd)

		w := cmd.OutOrStdout()
		render.Blank(w)

		orig := cmd.OutOrStdout()
		var buf strings.Builder
		cmd.SetOut(&buf)
		defaultHelp(cmd, args)
		cmd.SetOut(orig)

		for _, line := range strings.Split(buf.String(), "\n") {
			if line == "" {
				render.Blank(w)
			} else {
				render.Message(w, line)
			}
		}
	})
}

func Root() *cobra.Command {
	return rootCmd
}

func Execute() {
	ctx, stop := interruptible()
	defer stop()
	code := execute(ctx, executeParams{Root: rootCmd, Stderr: os.Stderr})
	printUpdateNotice(ctx)
	if code != domain.ExitCodeOK {
		os.Exit(code)
	}
}

type executeParams struct {
	Root   *cobra.Command
	Stderr io.Writer
}

func execute(ctx context.Context, params executeParams) int {
	executed, runErr := params.Root.ExecuteContextC(ctx)
	backedOut := rules.BackedOut(rules.BackedOutParams{Err: runErr, Cancelled: executed != nil && shared.Cancelled(executed)})
	err := rules.Interrupted(rules.InterruptedParams{Err: backedOut, Signalled: ctx.Err() != nil})
	settle()
	if err == nil {
		return domain.ExitCodeOK
	}
	if !errors.Is(err, domain.ErrAborted) || humanOutputSilenced {
		render.Blank(params.Stderr)
		reportFailure(params.Stderr, err)
		render.Blank(params.Stderr)
	}
	return exitCode(err)
}
