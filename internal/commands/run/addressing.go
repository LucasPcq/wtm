package run

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	addressingflow "github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/output"
)

func newAddressingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdAddressing + " [names|ports]",
		Short: "Switch how the .env files spell a job's address",
		Long: "Set run.toml's addressing — named URLs (http://api.feat-x.myrepo.localhost) or\n" +
			"port URLs (http://localhost:4012) — then settle the .env of the worktrees that spell\n" +
			"the other one. Settling runs even when the mode is already the one given, for a\n" +
			"worktree an earlier switch left out of step.\n\n" +
			"The main checkout is settled back to ports, never onto names: it is the checkout\n" +
			"that works without wtm, and `wtm env main` is how it is moved onto names.\n\n" +
			"Without an argument, prompts for the mode; under --yes the argument is required\n" +
			"and the worktrees are settled unless --keep-env is passed.",
		Example: `  # Pick the mode
  wtm run addressing

  wtm run addressing ports --yes

  # Switch run.toml only, leaving the .env files as they are
  wtm run addressing names --yes --keep-env`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{string(domain.AddressingNames), string(domain.AddressingPorts)},
		RunE:      runAddressing,
	}
	cmd.Flags().Bool(domain.FlagKeepEnv, false, domain.FlagKeepEnvDesc)
	shared.AddYesFlag(cmd, "Skip the prompts; [names|ports] is then required")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runAddressing(cmd *cobra.Command, args []string) error {
	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	keepEnv, _ := cmd.Flags().GetBool(domain.FlagKeepEnv)
	outcome, err := addressingflow.Switch(cmd.Context(), addressingflow.SwitchParams{
		Context: ctx.FlowContext(),
		Request: addressingflow.SwitchRequest{
			Mode:    domain.Addressing(runctx.FirstArg(args)),
			KeepEnv: keepEnv,
			Config:  ctx.Run,
		},
		Prompter:  ctx.Prompter(cmd.Context(), ctx.Interactive),
		Presenter: addressingPresenter{CLIPresenter: ctx.CLI(cmd)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted {
		return shared.EndAborted(cmd)
	}
	return nil
}

type addressingPresenter struct {
	shared.CLIPresenter
}

func (p addressingPresenter) Switched(outcome addressingflow.SwitchOutcome) error {
	result := output.AddressingResult{
		Addressing: outcome.Current,
		Previous:   outcome.Previous,
		Changed:    outcome.Changed,
		Settled:    outcome.Settled,
		Pending:    outcome.Pending,
		MainLeft:   outcome.MainLeft,
	}
	out := p.Cmd.OutOrStdout()
	if p.Format == domain.OutputJSON {
		return output.WriteAddressingResultJSON(out, result)
	}
	output.Frame(out, func(w io.Writer) { output.AddressingSwitched(w, result) })
	return nil
}
