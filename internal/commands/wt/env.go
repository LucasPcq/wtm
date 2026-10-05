package wt

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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
		Long: "Detect and fix .env drift in a worktree: add the keys its sources have and it\n" +
			"lacks, and with --mode refresh settle the values that diverge from the source.\n" +
			"Values come from the strategy the worktree was created with (example, main or\n" +
			"parent); --from overrides it for one run. When run.toml declares ports, the\n" +
			"values wtm owns are then settled on the worktree's isolation.\n\n" +
			"Pass a worktree, or omit it to pick one. --check reports and writes nothing.\n" +
			"A report prints only the values wtm writes (ports, owned values); the others,\n" +
			"secrets included, are withheld unless --show-values.\n" +
			"Unattended (--yes, no terminal, --output json) it applies safe additions only:\n" +
			"conflicts need --on-conflict, orphans --prune.\n\n" +
			"A run keeps how the worktree runs unless asked: the wizard offers to switch a\n" +
			"worktree between isolated and verbatim (--isolation), and the main checkout\n" +
			"between port and named addresses (--addressing) — see the isolation and\n" +
			"addressing guides.",
		Example: `  # Pick a worktree and reconcile its .env files
  wtm env

  # Read-only drift report, for a script
  wtm env feat/login --check --output json

  # Also settle the values that diverge, and drop the keys no source has
  wtm env feat/login --mode refresh --on-conflict overwrite --prune --yes

  # Give a worktree created before 0.28 its own ports and compose project
  wtm env feat/login --isolation isolated --yes

  # Reconcile the main checkout, moving its addresses back to ports
  wtm env main --addressing ports --yes`,
		Args: cobra.MaximumNArgs(1),
		RunE: runEnv,
	}

	cmd.Flags().String(domain.FlagMode, string(domain.EnvModeAdd), "Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts)")
	cmd.Flags().Bool(domain.FlagCheck, false, "Read-only drift report; write nothing")
	cmd.Flags().Bool(domain.FlagShowValues, false, "Print the values of keys wtm does not write (secrets included); withheld by default")
	cmd.Flags().Bool(domain.FlagPrune, false, "Remove orphan keys (present in the .env but in no source)")
	cmd.Flags().String(domain.FlagFrom, "", "Override the value source strategy (example, main, parent)")
	cmd.Flags().String(domain.FlagOnConflict, "", "Conflict resolution with --mode refresh: keep (default) or overwrite")
	cmd.Flags().String(domain.FlagIsolation, "", "Switch the worktree to isolated (its own ports, compose project and namespaces) or verbatim (the values wtm owns back to the source's)")
	cmd.Flags().String(domain.FlagAddressing, "", "Write the main checkout's linked addresses as ports (as without wtm) or names (served by the run proxy); default: what its .env spells")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (additions only)")
	shared.AddOutputFlag(cmd)
	shared.RequireYesInJSON(cmd, domain.FlagCheck)

	return cmd
}

func runEnv(cmd *cobra.Command, args []string) error {
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
	addressing, err := envAddressing(cmd)
	if err != nil {
		return err
	}
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	check, _ := cmd.Flags().GetBool(domain.FlagCheck)
	prune, _ := cmd.Flags().GetBool(domain.FlagPrune)
	showValues, _ := cmd.Flags().GetBool(domain.FlagShowValues)
	if err := rules.ValidateEnvFlags(rules.EnvFlagsParams{
		Check:         check,
		Prune:         prune,
		OnConflictSet: cmd.Flags().Changed(domain.FlagOnConflict),
		Mode:          mode,
		Isolation:     isolation,
	}); err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}
	if len(cfg.Config.Project.Env.Files) == 0 {
		return domain.ErrEnvNoFiles
	}

	worktreeArg := ""
	if len(args) == 1 {
		worktreeArg = args[0]
	}
	_, err = envflow.Run(cmd.Context(), envflow.Params{
		Context: shared.FlowContext(cfg),
		Request: envflow.Request{
			Worktree:   worktreeArg,
			Mode:       mode,
			From:       from,
			OnConflict: onConflict,
			Prune:      prune,
			Check:      check,
			Isolation:  isolation,
			Addressing: addressing,
		},
		// The wizard runs only fully interactively, and never for --check.
		Prompter:  shared.FlowPrompter(shared.FlowPrompterParams{Interactive: isInteractive() && rules.IsHumanFormat(format) && !yes && !check, Stderr: true}),
		Presenter: envPresenter{CLIPresenter: shared.NewPresenter(cmd, format), showValues: showValues},
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
	return "", rules.InvalidFlagValue(rules.InvalidFlagValueParams{Flag: domain.FlagMode, Value: v, Allowed: []string{string(domain.EnvModeAdd), string(domain.EnvModeRefresh)}})
}

// envFrom validates and returns the --from override, "" when unset.
func envFrom(cmd *cobra.Command) (string, error) {
	if !cmd.Flags().Changed(domain.FlagFrom) {
		return "", nil
	}
	v, _ := cmd.Flags().GetString(domain.FlagFrom)
	if err := rules.ValidateEnvStrategy(domain.EnvStrategy(v)); err != nil {
		return "", rules.InvalidFlagValue(rules.InvalidFlagValueParams{Flag: domain.FlagFrom, Value: v, Allowed: shared.EnvStrategyValues})
	}
	return v, nil
}

// envAddressing validates and returns the --addressing value, "" when unset.
func envAddressing(cmd *cobra.Command) (domain.Addressing, error) {
	v, _ := cmd.Flags().GetString(domain.FlagAddressing)
	switch domain.Addressing(v) {
	case "", domain.AddressingNames, domain.AddressingPorts:
		return domain.Addressing(v), nil
	}
	return "", rules.InvalidFlagValue(rules.InvalidFlagValueParams{Flag: domain.FlagAddressing, Value: v, Allowed: []string{string(domain.AddressingPorts), string(domain.AddressingNames)}})
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
	return "", rules.InvalidFlagValue(rules.InvalidFlagValueParams{Flag: domain.FlagOnConflict, Value: v, Allowed: []string{string(domain.EnvDecisionKeep), string(domain.EnvDecisionOverwrite)}})
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
