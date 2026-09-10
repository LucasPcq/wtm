package wt

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/envwizard"
)

// newEnvCmd creates the wtm env subcommand.
func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdEnv + " [worktree]",
		Short: "Reconcile a worktree's .env against its template and value sources",
		Long: "Detect and fix .env drift in a worktree: add expected-but-missing keys and\n" +
			"(with --mode refresh) settle values that diverge from the source.\n\n" +
			"Values come from the strategy the worktree was created with (example → template\n" +
			"placeholders, main → the main worktree, parent → the parent worktree then main),\n" +
			"shown in the report; override it per run with --from.\n\n" +
			"Pass a worktree branch, or omit it to pick interactively. --check prints a\n" +
			"read-only drift report. Non-interactively (--yes / --output json) it applies only\n" +
			"safe additions; conflicts need --on-conflict and orphans need --prune.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEnv,
	}

	cmd.Flags().String(domain.FlagMode, string(domain.EnvModeAdd), "Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts)")
	cmd.Flags().Bool(domain.FlagCheck, false, "Read-only drift report; write nothing")
	cmd.Flags().Bool(domain.FlagPrune, false, "Remove orphan keys (present in the .env but in no source)")
	cmd.Flags().String(domain.FlagFrom, "", "Override the value source strategy (example, main, parent)")
	cmd.Flags().String(domain.FlagOnConflict, "", "Non-interactive conflict resolution: keep (default) or overwrite")
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

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	check, _ := cmd.Flags().GetBool(domain.FlagCheck)
	prune, _ := cmd.Flags().GetBool(domain.FlagPrune)

	// A read-only --check never prompts, so it needs no --yes even in JSON.
	if format == domain.OutputJSON && !yes && !check {
		return domain.ErrEnvJSONNeedsYes
	}

	if len(cfg.Config.Project.Env.Files) == 0 {
		return domain.ErrEnvNoFiles
	}

	flags := envFlags{mode: mode, from: from, onConflict: onConflict, prune: prune, check: check, format: format}

	// The unified wizard runs only fully interactively (human output, a TTY, not
	// --yes) and never for --check (read-only). Everything else is the
	// report/flag-driven path.
	if isInteractive() && rules.IsHumanFormat(format) && !yes && !check {
		return runEnvInteractive(cmd, cfg, firstArg(args), flags)
	}
	return runEnvNonInteractive(cmd, cfg, firstArg(args), flags)
}

// envFlags carries the validated flag values through the two run paths.
type envFlags struct {
	mode       domain.EnvMode
	from       string
	onConflict domain.EnvConflictDecision
	prune      bool
	check      bool
	format     string
}

// runEnvNonInteractive reconciles a single named worktree without prompting
// (report / JSON / --yes / --check). The worktree arg is required.
func runEnvNonInteractive(cmd *cobra.Command, cfg shared.ConfigResult, arg string, f envFlags) error {
	if arg == "" {
		return domain.ErrEnvWorktreeRequired
	}
	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: cfg.ProjectDir,
		Branch:     arg,
	})
	if err != nil {
		return fmt.Errorf("worktree %q: %w", arg, err)
	}

	ctx := resolveEnvStrategyAndParent(cfg, wt.Branch, f.from)
	ports, err := resolveEnvPorts(cfg, wt.Branch, wt.Path)
	if err != nil {
		return err
	}
	result, err := envsvc.SyncEnv(envsvc.SyncEnvParams{
		Branch:             wt.Branch,
		MainPath:           cfg.ProjectDir,
		WorktreePath:       wt.Path,
		ParentWorktreePath: ctx.ParentPath,
		ParentBranch:       ctx.ParentBranch,
		Files:              cfg.Config.Project.Env.Files,
		Strategy:           ctx.Strategy,
		Mode:               f.mode,
		Prune:              f.prune,
		Check:              f.check,
		OnConflict:         f.onConflict,
		Ports:              ports,
	})
	if err != nil {
		return err
	}
	return writeEnvResult(cmd, result, f.format)
}

// runEnvInteractive drives the unified wizard (worktree selection → single-screen
// resolution → recap), then applies the collected decisions. When a worktree arg is
// given, the selection step is preset.
func runEnvInteractive(cmd *cobra.Command, cfg shared.ConfigResult, arg string, f envFlags) error {
	var (
		statuses      []domain.WorktreeStatus
		diffByBranch  map[string][]domain.EnvFileResult
		portsByBranch map[string]domain.EnvPortPlan
		preset        string
	)

	// One box over the whole pre-scan rather than one per phase: it is a single
	// wait as far as the reader is concerned, and two boxes in a row flicker.
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.EnvScanLoading,
		Animate: shared.Animate(cmd, true),
		Work: func() error {
			var err error
			statuses, err = worktree.List(domain.ListParams{
				ProjectDir: cfg.ProjectDir,
				StateDir:   cfg.StateDir,
				Config:     cfg.Config,
			})
			if err != nil {
				return fmt.Errorf("list worktrees: %w", err)
			}

			if arg != "" {
				if worktreePathForBranch(statuses, arg) == "" {
					return fmt.Errorf("worktree %q: %w", arg, domain.ErrWorktreeNotFound)
				}
				preset = arg
			}

			// The drift is precomputed for every branch the wizard will surface, so
			// the selection list can badge each one with it.
			branches := []string{preset}
			if preset == "" {
				branches = branchNames(statuses)
			}
			diffByBranch = make(map[string][]domain.EnvFileResult, len(branches))
			portsByBranch = make(map[string]domain.EnvPortPlan, len(branches))
			for _, b := range branches {
				files, err := computeBranchDiff(cfg, statuses, b, f)
				if err != nil {
					return err
				}
				diffByBranch[b] = files

				plan, err := computeBranchPorts(cfg, statuses, b)
				if err != nil {
					return err
				}
				portsByBranch[b] = plan
			}
			return nil
		},
	}); err != nil {
		return err
	}

	res, err := envwizard.Run(envwizard.RunParams{
		Candidates:    statuses,
		PresetBranch:  preset,
		DiffByBranch:  diffByBranch,
		PortsByBranch: portsByBranch,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return abortedEnv(cmd, f.format)
	}
	if err != nil {
		return err
	}

	ctx := resolveEnvStrategyAndParent(cfg, res.Branch, f.from)
	worktreePath := worktreePathForBranch(statuses, res.Branch)
	ports, err := resolveEnvPorts(cfg, res.Branch, worktreePath)
	if err != nil {
		return err
	}
	result, err := envsvc.ApplyEnvSync(envsvc.ApplyEnvSyncParams{
		Branch:             res.Branch,
		MainPath:           cfg.ProjectDir,
		WorktreePath:       worktreePath,
		ParentWorktreePath: ctx.ParentPath,
		ParentBranch:       ctx.ParentBranch,
		Files:              cfg.Config.Project.Env.Files,
		Strategy:           ctx.Strategy,
		Mode:               f.mode,
		Resolutions:        mapDecisions(res.Decisions),
		Ports:              ports,
		SkipPortRewrite:    res.SkipPorts,
	})
	if err != nil {
		return err
	}
	return writeEnvResult(cmd, result, f.format)
}

// computeBranchDiff computes one worktree's drift for the wizard (no write).
func computeBranchDiff(cfg shared.ConfigResult, statuses []domain.WorktreeStatus, branch string, f envFlags) ([]domain.EnvFileResult, error) {
	ctx := resolveEnvStrategyAndParent(cfg, branch, f.from)
	worktreePath := worktreePathForBranch(statuses, branch)
	ports, err := resolveEnvPorts(cfg, branch, worktreePath)
	if err != nil {
		return nil, err
	}
	return envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:             branch,
		MainPath:           cfg.ProjectDir,
		WorktreePath:       worktreePath,
		ParentWorktreePath: ctx.ParentPath,
		ParentBranch:       ctx.ParentBranch,
		Files:              cfg.Config.Project.Env.Files,
		Strategy:           ctx.Strategy,
		Mode:               f.mode,
		Ports:              ports,
	})
}

// computeBranchPorts resolves one worktree's port pass for the wizard recap, so
// the apply is announced before it happens rather than discovered after.
func computeBranchPorts(cfg shared.ConfigResult, statuses []domain.WorktreeStatus, branch string) (domain.EnvPortPlan, error) {
	ports, err := resolveEnvPorts(cfg, branch, worktreePathForBranch(statuses, branch))
	if err != nil || ports.Empty() {
		return domain.EnvPortPlan{}, err
	}
	return envsvc.ComputeEnvPorts(ports)
}

// resolveEnvPorts gathers the [[env_port]] links and the offset this worktree
// binds on. A project with no run.toml, or none declared, resolves to nothing and
// the reconciliation runs exactly as it did before.
func resolveEnvPorts(cfg shared.ConfigResult, branch string, worktreePath string) (envsvc.EnvPortsParams, error) {
	return worktree.ResolveEnvPorts(worktree.ResolveEnvPortsParams{
		ProjectDir:   cfg.ProjectDir,
		StateDir:     cfg.StateDir,
		Branch:       branch,
		WorktreePath: worktreePath,
		EnvFiles:     cfg.Config.Project.Env.Files,
		Global:       cfg.Config.Global,
	})
}

// mapDecisions converts the wizard's per-file decisions to service resolutions.
func mapDecisions(decisions []components.EnvFileDecision) map[string]envsvc.EnvResolution {
	out := make(map[string]envsvc.EnvResolution, len(decisions))
	for _, d := range decisions {
		out[d.Target] = envsvc.EnvResolution{
			Decisions:    d.Decisions,
			FilledValues: d.FilledValues,
			PruneKeys:    toSet(d.PruneKeys),
			SkipKeys:     toSet(d.SkipKeys),
		}
	}
	return out
}

// toSet turns a key slice into a set for the resolution maps.
func toSet(keys []string) map[string]bool {
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// branchNames extracts the branch of each worktree status.
func branchNames(statuses []domain.WorktreeStatus) []string {
	out := make([]string, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, s.Branch)
	}
	return out
}

// writeEnvResult routes the reconciliation result to JSON or the framed report.
func writeEnvResult(cmd *cobra.Command, result domain.EnvSyncResult, format string) error {
	if format == domain.OutputJSON {
		return output.WriteEnvJSON(cmd.OutOrStdout(), result)
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.PrintEnvReport(w, result)
	})
	return nil
}

// envContext is the resolved value strategy plus the recorded parent branch and its
// on-disk worktree path ("" when it has no local worktree, so the "parent" strategy
// falls back to main).
type envContext struct {
	Strategy     domain.EnvStrategy
	ParentBranch string
	ParentPath   string
}

// resolveEnvStrategyAndParent resolves the strategy (memorized strategy with any
// --from override, falling back to the config default) and the parent branch + path.
func resolveEnvStrategyAndParent(cfg shared.ConfigResult, branch, from string) envContext {
	base := cfg.Config.Project.Env.Strategy
	parentBranch := ""
	if meta, ok := worktree.Metadata(worktree.ParentBranchParams{StateDir: cfg.StateDir, Branch: branch}); ok {
		if meta.EnvStrategy != "" {
			base = meta.EnvStrategy
		}
		parentBranch = meta.SourceBranch
	}

	parentPath := ""
	if parentBranch != "" {
		if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: cfg.ProjectDir,
			Branch:     parentBranch,
		}); err == nil {
			parentPath = wt.Path
		}
	}
	return envContext{
		Strategy:     rules.ResolveEnvStrategy(base, from),
		ParentBranch: parentBranch,
		ParentPath:   parentPath,
	}
}

// abortedEnv prints the framed "Aborted." line on the human path and returns nil.
func abortedEnv(cmd *cobra.Command, format string) error {
	if rules.IsHumanFormat(format) {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, domain.AbortedMessage)
		})
	}
	return nil
}

func firstArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return ""
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
