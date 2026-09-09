package run

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newDownCmd creates the wtm run down subcommand.
func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdDown + " [worktree...]",
		Short: "Stop a worktree's running jobs",
		Long:  "Stop the jobs running in [worktree] — the current one when omitted, picked interactively when there is a terminal.\nWith --profile, stops only that profile's jobs.\nJobs running in other worktrees are never touched.",
		Args:  cobra.ArbitraryArgs,
		RunE:  runDown,
	}
	shared.AddProfileFlag(cmd, "Stop only this profile's jobs")
	shared.AddYesFlag(cmd, "Skip all prompts; stops what the worktree has running")
	shared.AddOutputFlag(cmd)
	cmd.Flags().Bool(domain.FlagAll, false, "Stop jobs across every worktree (bypasses per-worktree scoping)")
	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	all, _ := cmd.Flags().GetBool(domain.FlagAll)
	profile, _ := cmd.Flags().GetString(domain.FlagProfile)

	// --all is a different question, not a wider answer to this one: it takes
	// neither a worktree nor a profile.
	if all && (len(args) > 0 || profile != "") {
		return fmt.Errorf("--%s cannot be combined with a worktree or --%s", domain.FlagAll, domain.FlagProfile)
	}

	ctx, err := runctx.Open(runctx.OpenParams{Cmd: cmd})
	if err != nil {
		return err
	}

	outcome, err := downflow.Run(downflow.Params{
		Context: ctx.FlowContext(),
		Request: downflow.Request{
			Worktrees: args,
			Cwd:       ctx.Dir,
			Profile:   profile,
			All:       all,
			Config:    ctx.Run,
		},
		Prompter:  ctx.Prompter(!all && ctx.Interactive),
		Presenter: downPresenter{CLIPresenter: shared.NewPresenter(cmd, format)},
	})
	if err != nil {
		return err
	}
	if outcome.Aborted || outcome.Failed() {
		return domain.ErrAborted
	}
	return nil
}

// downPresenter reports what the worktree had running. A job left standing is
// named on stderr and turned into a non-zero exit by the runner; both surfaces
// have already listed the jobs, so the error carries nothing more (LUC-198).
type downPresenter struct {
	shared.CLIPresenter
}

func (p downPresenter) Downed(outcome downflow.Outcome) error {
	if p.Format == domain.OutputJSON {
		return output.WriteWorktreeJobResultsJSON(p.Cmd.OutOrStdout(), outcome.Results)
	}

	out, errOut := p.Cmd.OutOrStdout(), p.Cmd.ErrOrStderr()
	if outcome.NoDaemon || len(outcome.Stopped()) == 0 {
		output.Frame(out, func(w io.Writer) { output.Message(w, p.nothingRunning(outcome)) })
		return nil
	}

	// A job left standing is named on stderr as it is refused, so the reason
	// reaches a reader piping stdout; the recap then accounts for it alongside
	// what did go down.
	if rules.WorktreeJobsHaveErrors(outcome.Results) {
		output.FrameStart(errOut)
		barred := output.Barred(errOut)
		for _, worktree := range outcome.Results {
			for _, result := range worktree.Jobs {
				if result.Status != domain.JobActionError {
					continue
				}
				output.Error(barred, p.qualify(fmt.Sprintf("%s: %s", result.Name, result.Message), outcome, worktree))
			}
		}
	}

	output.Frame(out, func(w io.Writer) {
		fmt.Fprint(w, output.FormatRunDownRecap(output.RunDownRecapParams{
			Profile: outcome.Profile,
			Results: outcome.Results,
		}))
	})
	return nil
}

// qualify names the worktree at the end of the line, never inside it: `migrate
// stopped · main` is the sentence `run up` writes, `migrate · main stopped` is
// the same words in the wrong order.
func (p downPresenter) qualify(line string, outcome downflow.Outcome, worktree domain.WorktreeJobResults) string {
	if len(outcome.Results) <= 1 || worktree.Worktree == "" {
		return line
	}
	return fmt.Sprintf(domain.RunStreamWorktreeFmt, line, worktree.Worktree)
}

func (p downPresenter) nothingRunning(outcome downflow.Outcome) string {
	if outcome.All {
		return domain.RunNoJobsRunning
	}
	return domain.RunNoJobsHere
}
