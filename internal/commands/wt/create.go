package wt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/rules"
)

// newCreateCmd creates the wtm create subcommand.
func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdCreate + " [branch...]",
		Short: "Create one or more worktrees",
		Long: "Create one or more git worktrees with env provisioning, metadata, and hooks.\n" +
			"Several branches are created one after the other from the same source; a failure\n" +
			"does not stop the others, and the run ends with what was created and what failed.\n" +
			"A branch that already exists locally is checked out as-is, keeping its commits.\n" +
			"Its parent can't be inferred, so --from then names the branch recorded for\n" +
			"`wtm sync` — asked in the wizard, required without it.\n" +
			"When run.toml declares jobs, a branch whose derived name a live worktree or another\n" +
			"branch of the run already carries (feat.x next to feat/x) is refused.\n" +
			"Without arguments, the wizard asks for the branches: tab adds another, enter continues.",
		Example: `  # Answer the wizard: branches, source, env strategy, isolation
  wtm create

  # Three worktrees from the base branch, no prompts
  wtm create feat/login feat/billing fix/header --yes

  # A stacked branch on top of feat/login
  wtm create feat/login-ui --from feat/login --yes

  # For a script or an agent: idempotent, with a JSON envelope
  wtm create feat/login --if-not-exists --yes --output json`,
		Args: cobra.ArbitraryArgs,
		RunE: runCreate,
	}

	// No backquotes in this usage string: cobra reads backquoted text as the flag's
	// value placeholder, which would render as "--from wtm sync" instead of string.
	cmd.Flags().String(domain.FlagFrom, "", "Source branch to start from — or, when the branch already exists locally, the parent to record for wtm sync (required there without the wizard)")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward to origin before creating — the source branch, or the branch itself when it already exists locally (non-interactive; skipped when it has diverged)")
	cmd.Flags().String(domain.FlagEnvFrom, "", "Override env strategy (example, main, parent)")
	shared.AddIsolationFlag(cmd)
	shared.AddAskFlag(cmd)
	cmd.Flags().Bool(domain.FlagIfNotExists, false, "Succeed silently if the worktree already exists (idempotent)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (branch names required; source defaults to the base branch for a new branch, and --from is required for one that already exists)")
	shared.AddOutputFlag(cmd)
	shared.RequireYesInJSON(cmd)

	return cmd
}

func runCreate(cmd *cobra.Command, args []string) error {
	fromFlag, _ := cmd.Flags().GetString(domain.FlagFrom)
	ffFlag, _ := cmd.Flags().GetBool(domain.FlagFF)
	ifNotExists, _ := cmd.Flags().GetBool(domain.FlagIfNotExists)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	ask, _ := cmd.Flags().GetBool(domain.FlagAsk)
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	isolation, err := shared.IsolationFlag(cmd)
	if err != nil {
		return err
	}
	envFromFlag, err := shared.EnvFromFlag(cmd)
	if err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	config, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	// The wizard needs a TTY and is skipped by --yes; a human-format run without a
	// terminal also takes the prompt-free path.
	interactive := rules.IsHumanFormat(format) && !yes && term.IsTerminal(int(os.Stdin.Fd()))

	_, err = createflow.Run(cmd.Context(), createflow.Params{
		Context: shared.FlowContext(config),
		Request: createflow.Request{
			Branches:    args,
			From:        fromFlag,
			EnvFrom:     envFromFlag,
			FastForward: ffFlag,
			IfNotExists: ifNotExists,
			Isolation:   isolation,
			Ask:         ask,
		},
		Prompter:  shared.FlowPrompter(cmd.Context(), shared.FlowPrompterParams{Interactive: interactive}),
		Presenter: createPresenter{CLIPresenter: shared.NewPresenter(cmd, format), config: config},
	})
	return err
}

type displayPathParams struct {
	Config     domain.Config
	ProjectDir string
	Path       string
}

// createDisplayPath renders the new worktree as base_path/<name>. One that already
// existed may sit elsewhere (adopted, or holding the branch outside base_path), and
// recomposing it there would name a directory that does not exist.
func createDisplayPath(params displayPathParams) string {
	base := params.Config.Project.Worktrees.BasePath
	expected := filepath.Join(params.ProjectDir, base, filepath.Base(params.Path))
	if filepath.Clean(expected) != filepath.Clean(params.Path) {
		return params.Path
	}
	return filepath.Join(base, filepath.Base(params.Path))
}
