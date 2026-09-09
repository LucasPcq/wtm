// Package cmd defines the Cobra command tree for the wtm CLI.
package cmd

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/agents"
	"github.com/LucasPcq/wtm/internal/commands/checkout"
	"github.com/LucasPcq/wtm/internal/commands/configcmd"
	"github.com/LucasPcq/wtm/internal/commands/daemon"
	"github.com/LucasPcq/wtm/internal/commands/initcmd"
	"github.com/LucasPcq/wtm/internal/commands/resolve"
	"github.com/LucasPcq/wtm/internal/commands/run"
	"github.com/LucasPcq/wtm/internal/commands/schema"
	"github.com/LucasPcq/wtm/internal/commands/shell"
	"github.com/LucasPcq/wtm/internal/commands/ui"
	"github.com/LucasPcq/wtm/internal/commands/upgrade"
	"github.com/LucasPcq/wtm/internal/commands/wt"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func init() {
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
	rootCmd.AddCommand(run.NewCmd())

	uiCmd := ui.NewCmd(ui.NewCmdParams{Version: effectiveVersion})
	uiCmd.GroupID = domain.CmdGroupWorktrees
	rootCmd.AddCommand(uiCmd)

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

	rootCmd.AddCommand(daemon.NewCmd())
	rootCmd.AddCommand(daemon.NewProxyForwardCmd())
}

// version reads the one symbol goreleaser stamps, domain.Version — the same one
// the run daemon compares across its socket. A second stamped target would let a
// client and its daemon disagree about which build each is.
var version = domain.Version

// effectiveVersion is what every consumer reads: the linked version, or the
// module version recovered from the build info for a `go install` binary.
// Resolved as a package-level initializer, not inside init(): Go orders var
// initialization by dependency, so every command constructed below already sees
// the final value regardless of the order they are registered in.
var effectiveVersion = selfupdate.ResolveVersion(version)

var (
	updateCheck        *selfupdate.Check
	updateCheckStarted bool
)

// topLevelName is the executed command's name directly under root, which is what
// the exclusion list names: the runnable leaves are `completion zsh` and
// `schema dump`, not their parents, so cmd.Name() would let both slip through.
func topLevelName(cmd *cobra.Command) string {
	for cmd.HasParent() && cmd.Parent().HasParent() {
		cmd = cmd.Parent()
	}

	return cmd.Name()
}

// startUpdateCheck arms the passive check once per run. It is called from both
// PersistentPreRun and the help function because cobra short-circuits on the
// --help flag before any Run hook fires; the guard keeps the second caller from
// discarding an in-flight check started by the first.
func startUpdateCheck(cmd *cobra.Command) {
	if updateCheckStarted {
		return
	}
	updateCheckStarted = true

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	updateCheck = selfupdate.StartCheck(selfupdate.StartCheckParams{
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
	RunE:    rootRunE,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		startUpdateCheck(cmd)
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

// printUpdateNotice drains the passive check started in PersistentPreRun. It is
// nil-safe: a suppressed check leaves updateCheck nil.
func printUpdateNotice() {
	current, latest, method, ok := updateCheck.Notice(domain.UpdateNoticeWait)
	if !ok {
		return
	}

	output.Frame(os.Stderr, func(w io.Writer) {
		output.UpdateNotice(w, output.UpdateNoticeParams{Current: current, Latest: latest, Method: method})
	})
}

func init() {
	// Override the global help function to add consistent padding around help text.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		startUpdateCheck(cmd)

		w := cmd.OutOrStdout()
		output.Blank(w)

		// Temporarily swap the output writer to capture and indent the help text.
		orig := cmd.OutOrStdout()
		var buf strings.Builder
		cmd.SetOut(&buf)
		defaultHelp(cmd, args)
		cmd.SetOut(orig)

		for _, line := range strings.Split(buf.String(), "\n") {
			if line == "" {
				output.Blank(w)
			} else {
				output.Message(w, line)
			}
		}
	})
}

// Root returns the fully assembled root command. It exists so tooling (e.g. the
// docs generator under tools/gendocs) can walk the command tree without invoking it.
func Root() *cobra.Command {
	return rootCmd
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// ErrAborted means the command already printed its own report; just
		// propagate the non-zero exit without a second error line.
		if !errors.Is(err, domain.ErrAborted) {
			output.Blank(os.Stderr)
			output.Error(os.Stderr, err.Error())
			output.Blank(os.Stderr)
		}
		printUpdateNotice()
		os.Exit(rules.ExitCode(err))
	}

	printUpdateNotice()
}
