package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	envflow "github.com/LucasPcq/wtm/internal/flow/env"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newEnvCmd creates the wtm env subcommand.
func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEnv + " [worktree]",
		Short: "Reconcile a worktree's .env against its template and value sources",
		Long: "Detect and fix .env drift in a worktree: add expected-but-missing keys and\n" +
			"(with --mode refresh) settle values that diverge from the source.\n\n" +
			"Values come from the strategy the worktree was created with, shown in the report:\n" +
			"example → the template's placeholders, main → the main checkout, parent → the\n" +
			"parent worktree only (main is read only when the parent has no worktree or no such\n" +
			"file). Override it per run with --from.\n\n" +
			"Pass a worktree branch, or omit it to pick interactively. --check prints a\n" +
			"read-only drift report. Non-interactively (--yes / --output json) it applies only\n" +
			"safe additions; conflicts need --on-conflict and orphans need --prune.\n\n" +
			"When run.toml declares ports, a second pass follows on the reconciled files of an\n" +
			"isolated worktree: each [[env_port]] link and [[env]] value is settled on its own\n" +
			"ports and namespaces, and COMPOSE_PROJECT_NAME is written for a compose job. An invalid\n" +
			"run.toml skips that pass with a warning; the keys are still reconciled. The main\n" +
			"checkout is a worktree like any other here: `wtm env main` settles it on run.toml's\n" +
			"declared ports, which it never shifts.\n\n" +
			"A worktree created before the isolation choice existed (no isolation in its\n" +
			"meta.json) keeps its source's ports and COMPOSE_PROJECT_NAME: non-interactively\n" +
			"only its keys are reconciled, and the report says so. The wizard offers to adopt\n" +
			"isolation — a new compose project, so its current volumes are no longer used —\n" +
			"and --isolation isolated adopts it explicitly.\n\n" +
			"--isolation verbatim puts the values wtm owns (linked ports, [[env]] values,\n" +
			"COMPOSE_PROJECT_NAME) back to the source's and leaves every other key alone: the\n" +
			"worktree then shares its source's compose volumes and data. The wizard shows those\n" +
			"values first, and its recap can also keep a worktree verbatim from then on. Either\n" +
			"isolation is recorded only once the .env is in line with it: a run that fails or\n" +
			"is cancelled records nothing.",
		Example: `  # Pick a worktree and reconcile its .env files
  wtm env

  # Read-only drift report
  wtm env feat/login --check

  # Also settle the values that diverge from the source
  wtm env feat/login --mode refresh --on-conflict overwrite --yes

  # Give a worktree created before 0.28 its own ports and compose project
  wtm env feat/login --isolation isolated --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runEnv,
	}

	cmd.Flags().String(domain.FlagMode, string(domain.EnvModeAdd), "Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts)")
	cmd.Flags().Bool(domain.FlagCheck, false, "Read-only drift report; write nothing")
	cmd.Flags().Bool(domain.FlagPrune, false, "Remove orphan keys (present in the .env but in no source)")
	cmd.Flags().String(domain.FlagFrom, "", "Override the value source strategy (example, main, parent)")
	cmd.Flags().String(domain.FlagOnConflict, "", "Non-interactive conflict resolution: keep (default) or overwrite")
	cmd.Flags().String(domain.FlagIsolation, "", "Settle the worktree on an isolation, recorded once its .env is in line: isolated (wtm moves its ports, compose project and namespaces, in the .env and at run time) or verbatim (the values wtm owns go back to the source's, and it runs on the ports its .env keeps)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; apply safe additions and flag-driven decisions only")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runEnv(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	mode, err := envMode(cmd)
	if err != nil {
		return err
	}
	from, err := envFrom(cmd)
	if err != nil {
		return err
	}
	onConflict, err := envOnConflict(cmd)
	if err != nil {
		return err
	}
	isolation, err := shared.IsolationFlag(cmd)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	check, _ := cmd.Flags().GetBool(domain.FlagCheck)
	prune, _ := cmd.Flags().GetBool(domain.FlagPrune)

	// A read-only --check never prompts, so it needs no --yes even in JSON.
	if format == domain.OutputJSON && !yes && !check {
		return domain.ErrEnvJSONNeedsYes
	}
	if check && isolation != "" {
		return domain.ErrEnvIsolationWithCheck
	}

	if len(cfg.Config.Project.Env.Files) == 0 {
		return domain.ErrEnvNoFiles
	}

	worktreeArg := ""
	if len(args) == 1 {
		worktreeArg = args[0]
	}
	_, err = envflow.Run(envflow.Params{
		Context: shared.FlowContext(cfg),
		Request: envflow.Request{
			Worktree:   worktreeArg,
			Mode:       mode,
			From:       from,
			OnConflict: onConflict,
			Prune:      prune,
			Check:      check,
			Isolation:  isolation,
		},
		// The wizard runs only fully interactively, and never for --check.
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: isInteractive() && rules.IsHumanFormat(format) && !yes && !check, Stderr: true}),
		Presenter: envPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	return err
}

// envMode validates and returns the --mode value.
func envMode(cmd *cobra.Command) (domain.EnvMode, error) {
	v, _ := cmd.Flags().GetString(domain.FlagMode)
	switch domain.EnvMode(v) {
	case domain.EnvModeAdd, domain.EnvModeRefresh:
		return domain.EnvMode(v), nil
	}
	return "", fmt.Errorf("invalid --%s value %q: use %s or %s",
		domain.FlagMode, v, domain.EnvModeAdd, domain.EnvModeRefresh)
}

// envFrom validates and returns the --from override, "" when unset.
func envFrom(cmd *cobra.Command) (string, error) {
	if !cmd.Flags().Changed(domain.FlagFrom) {
		return "", nil
	}
	v, _ := cmd.Flags().GetString(domain.FlagFrom)
	if err := rules.ValidateEnvStrategy(domain.EnvStrategy(v)); err != nil {
		return "", fmt.Errorf("invalid --%s value %q: %w", domain.FlagFrom, v, err)
	}
	return v, nil
}

// envOnConflict validates and returns the --on-conflict decision, defaulting to
// keep (the safe default) when unset.
func envOnConflict(cmd *cobra.Command) (domain.EnvConflictDecision, error) {
	if !cmd.Flags().Changed(domain.FlagOnConflict) {
		return domain.EnvDecisionKeep, nil
	}
	v, _ := cmd.Flags().GetString(domain.FlagOnConflict)
	switch domain.EnvConflictDecision(v) {
	case domain.EnvDecisionKeep, domain.EnvDecisionOverwrite:
		return domain.EnvConflictDecision(v), nil
	}
	return "", fmt.Errorf("invalid --%s value %q: use %s or %s",
		domain.FlagOnConflict, v, domain.EnvDecisionKeep, domain.EnvDecisionOverwrite)
}
