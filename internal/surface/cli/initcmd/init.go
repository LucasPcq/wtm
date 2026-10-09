package initcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
	initwizard "github.com/LucasPcq/wtm/internal/surface/tui/inittui"
)

// NewCmd creates the wtm init command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize wtm configuration",
		Long: "Interactive wizard to set up global config and project config in <git-common-dir>/wtm/config.toml.\n" +
			"Pass --yes (or any config flag) to bootstrap from flags + auto-detection instead; without a\n" +
			"terminal, init does so on its own and never prompts.\n" +
			"Use --only env|hooks|worktrees to re-run init for specific sections and regenerate them cleanly.\n" +
			"Services & tasks are configured separately with `wtm run init`.",
		Example: `  # The wizard
  wtm init

  # Unattended, from detection
  wtm init --yes

  wtm init --yes --base-path ../acme.trees --install-command "pnpm install"

  # Regenerate the hooks section only
  wtm init --only hooks`,
		RunE: runInit,
	}

	cmd.Flags().String(domain.FlagShell, "", "Global shell: zsh, bash, or fish")
	cmd.Flags().String(domain.FlagBasePath, "", "Worktree directory, relative to repo root")
	cmd.Flags().String(domain.FlagBaseBranch, "", "Default base branch for new worktrees")
	cmd.Flags().String(domain.FlagEnvStrategy, "", "Env provisioning strategy: example, main, or parent")
	cmd.Flags().String(domain.FlagInstallCommand, "", "Command to run after creating a worktree")
	cmd.Flags().String(domain.FlagCleanCommand, "", "Command to run before removing a worktree")
	cmd.Flags().Bool(domain.FlagSkipEnv, false, "Skip .env provisioning config")
	cmd.Flags().Bool(domain.FlagSkipHooks, false, "Skip on_create hooks config")
	cmd.Flags().Bool(domain.FlagSkipClean, false, "Skip on_clean hooks config")
	cmd.Flags().StringSlice(domain.FlagOnly, nil, "Re-init only these sections (env, hooks, worktrees); regenerates them cleanly")
	shared.AddYesFlag(cmd, "Run unattended: bootstrap (or re-init) from flags + auto-detection; never prompt")

	return cmd
}

func interactive(cmd *cobra.Command) bool {
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	return shared.Interactive(shared.UnattendedParams{TTY: term.IsTerminal(int(os.Stdin.Fd())), Format: format, Yes: yes})
}

// initFlagged reports whether init takes the flag-driven path: nobody can be
// asked, or a config flag already answered.
func initFlagged(cmd *cobra.Command) bool {
	if !interactive(cmd) {
		return true
	}
	for _, name := range []string{
		domain.FlagShell,
		domain.FlagBasePath, domain.FlagBaseBranch, domain.FlagEnvStrategy,
		domain.FlagInstallCommand, domain.FlagCleanCommand,
		domain.FlagSkipEnv, domain.FlagSkipHooks, domain.FlagSkipClean,
	} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func runInit(cmd *cobra.Command, _ []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	flagged := initFlagged(cmd)

	if err := ensureGlobalConfig(cmd, flagged); err != nil {
		return err
	}

	stateDir, err := shared.StateDir(cmd.Context(), dir)
	if err != nil {
		return fmt.Errorf("wtm must be run inside a git repository: %w", err)
	}

	if only, _ := cmd.Flags().GetStringSlice(domain.FlagOnly); len(only) > 0 {
		sections, err := parseSections(only)
		if err != nil {
			return err
		}
		return runReinit(cmd, dir, stateDir, sections)
	}

	if detect.ProjectConfigExists(stateDir) {
		render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			render.Unchanged(w, fmt.Sprintf(domain.InitAlreadyExistsFmt, filepath.Join(stateDir, domain.ConfigFileName)))
			render.Blank(w)
			render.NextSteps(w, []render.NextStepParams{
				{Command: domain.InitReconfigureCmd, Note: domain.InitReconfigureNote},
				{Command: domain.InitEditCmd, Note: domain.InitEditNote},
				{Command: domain.InitRunInitCmd, Note: domain.InitRunInitNote},
			})
		})
		return nil
	}

	return createProjectConfig(cmd, dir, stateDir, flagged)
}

func ensureGlobalConfig(cmd *cobra.Command, flagged bool) error {
	if detect.GlobalConfigExists() {
		return nil
	}

	answers, err := resolveGlobalAnswers(cmd, flagged)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := config.WriteGlobal(answers); err != nil {
		return fmt.Errorf("write global config: %w", err)
	}

	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		render.InitGlobalRecap(w, render.InitGlobalRecapParams{
			Fields: rules.InitGlobalRecapFields(answers),
			NextSteps: []render.NextStepParams{
				{Command: domain.InitNextStepShell, Note: domain.InitNextStepShellNote},
			},
		})
	})

	return nil
}

// resolveGlobalAnswers builds the global config either from flags or the
// interactive wizard.
func resolveGlobalAnswers(cmd *cobra.Command, flagged bool) (domain.InitGlobalAnswers, error) {
	if flagged {
		shell, _ := cmd.Flags().GetString(domain.FlagShell)
		return rules.BuildGlobalAnswers(rules.InitGlobalFlags{Shell: shell})
	}

	render.Message(cmd.OutOrStdout(), "No global config found. Let's set one up.")

	answers, err := initwizard.RunGlobalWizard()
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return domain.InitGlobalAnswers{}, err
		}
		return domain.InitGlobalAnswers{}, fmt.Errorf("global wizard: %w", err)
	}
	return answers, nil
}

// resolveProjectAnswers builds the project config either from flags + detection
// or the interactive wizard.
func resolveProjectAnswers(cmd *cobra.Command, projectDir string, flagged bool, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	if flagged {
		basePath, _ := cmd.Flags().GetString(domain.FlagBasePath)
		baseBranch, _ := cmd.Flags().GetString(domain.FlagBaseBranch)
		envStrategy, _ := cmd.Flags().GetString(domain.FlagEnvStrategy)
		installCommand, _ := cmd.Flags().GetString(domain.FlagInstallCommand)
		cleanCommand, _ := cmd.Flags().GetString(domain.FlagCleanCommand)
		skipEnv, _ := cmd.Flags().GetBool(domain.FlagSkipEnv)
		skipHooks, _ := cmd.Flags().GetBool(domain.FlagSkipHooks)
		skipClean, _ := cmd.Flags().GetBool(domain.FlagSkipClean)
		return rules.BuildProjectAnswers(rules.InitProjectFlags{
			BasePath:       basePath,
			BaseBranch:     baseBranch,
			EnvStrategy:    envStrategy,
			InstallCommand: installCommand,
			CleanCommand:   cleanCommand,
			Unattended:     !interactive(cmd),
			SkipEnv:        skipEnv,
			SkipHooks:      skipHooks,
			SkipClean:      skipClean,
		}, detection)
	}

	answers, err := initwizard.RunProjectWizard(cmd.Context(), projectDir, detection)
	if err != nil {
		if errors.Is(err, domain.ErrUserAborted) {
			return domain.InitProjectAnswers{}, err
		}
		return domain.InitProjectAnswers{}, fmt.Errorf("project wizard: %w", err)
	}
	return answers, nil
}

func createProjectConfig(cmd *cobra.Command, dir, stateDir string, flagged bool) error {
	var detection domain.InitDetectionResult
	_ = components.RunLoading(cmd.Context(), components.LoadingParams{
		Message: "Detecting project settings…",
		Animate: shared.Animate(cmd, !flagged),
		Work:    func() error { detection = detect.ProjectEnvironment(cmd.Context(), dir); return nil },
	})

	answers, err := resolveProjectAnswers(cmd, dir, flagged, detection)
	if errors.Is(err, domain.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers:  answers,
	}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}
	if root, err := shared.ProjectRoot(cmd.Context(), dir); err == nil {
		shared.Register(shared.RegisterParams{Root: root, StateDir: stateDir, CorrelationID: shared.CorrelationID(cmd)})
	}

	render.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		render.InitProjectRecap(w, render.InitProjectRecapParams{
			ConfigPath: rules.DisplayPath(rules.DisplayPathParams{Base: dir, Target: filepath.Join(stateDir, domain.ConfigFileName)}),
			Fields:     rules.InitProjectRecapFields(answers),
			NextSteps: []render.NextStepParams{
				{Command: domain.InitNextStepCreate, Note: domain.InitNextStepCreateNote},
				{Command: domain.InitNextStepRelocate, Note: domain.InitNextStepRelocateNote},
				{Command: domain.InitNextStepRunInit, Note: domain.InitNextStepRunInitNote},
			},
		})
	})
	return nil
}
