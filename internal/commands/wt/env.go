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
	"github.com/LucasPcq/wtm/internal/service/runconfig"
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
			"safe additions; conflicts need --on-conflict and orphans need --prune.\n\n" +
			"A worktree created before the isolation choice existed (no isolation in its\n" +
			"meta.json) keeps its source's ports and COMPOSE_PROJECT_NAME: non-interactively\n" +
			"only its keys are reconciled, and the report says so. The wizard offers to adopt\n" +
			"isolation — a new compose project, so its current volumes are no longer used —\n" +
			"and --isolation isolated adopts it explicitly.",
		Args: cobra.MaximumNArgs(1),
		RunE: runEnv,
	}

	cmd.Flags().String(domain.FlagMode, string(domain.EnvModeAdd), "Reconciliation mode: add (fill gaps) or refresh (also settle value conflicts)")
	cmd.Flags().Bool(domain.FlagCheck, false, "Read-only drift report; write nothing")
	cmd.Flags().Bool(domain.FlagPrune, false, "Remove orphan keys (present in the .env but in no source)")
	cmd.Flags().String(domain.FlagFrom, "", "Override the value source strategy (example, main, parent)")
	cmd.Flags().String(domain.FlagOnConflict, "", "Non-interactive conflict resolution: keep (default) or overwrite")
	cmd.Flags().String(domain.FlagIsolation, "", "Switch the worktree's isolation before reconciling: isolated (wtm moves its ports, compose project and service slices, in the .env and at run time) or verbatim (wtm writes none of them and runs it on the ports its .env keeps)")
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

	flags := envFlags{mode: mode, from: from, onConflict: onConflict, prune: prune, check: check, format: format, isolation: isolation}

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
	// isolation is --isolation, recorded before anything is read so the whole
	// run — drift, port pass, report — sees the worktree as it now stands.
	isolation domain.Isolation
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
	adoption, err := isolationAdoption(cfg, envTarget{branch: wt.Branch, path: wt.Path})
	if err != nil {
		return err
	}
	if err := recordIsolation(cfg, wt.Branch, f.isolation); err != nil {
		return err
	}

	ctx := resolveEnvStrategyAndParent(cfg, wt.Branch, f.from)
	pass := runPass(runPassParams{cfg: cfg, branch: wt.Branch, worktreePath: wt.Path, adoption: adoption, adopt: f.isolation != ""})
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
		Ports:              pass.ports,
		Reserved:           pass.reserved,
	})
	if err != nil {
		return err
	}
	return writeEnvResult(cmd, pass.decorate(cfg, result), f.format)
}

// runEnvInteractive drives the unified wizard (worktree selection → single-screen
// resolution → recap), then applies the collected decisions. When a worktree arg is
// given, the selection step is preset.
func runEnvInteractive(cmd *cobra.Command, cfg shared.ConfigResult, arg string, f envFlags) error {
	var (
		statuses         []domain.WorktreeStatus
		diffByBranch     map[string][]domain.EnvFileResult
		portsByBranch    map[string]domain.EnvPortPlan
		adoptionByBranch map[string]domain.IsolationAdoptionPlan
		preset           string
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

			if arg != "" && worktreePathForBranch(statuses, arg) == "" {
				return fmt.Errorf("worktree %q: %w", arg, domain.ErrWorktreeNotFound)
			}
			preset = arg

			// The drift is precomputed for every branch the wizard will surface, so
			// the selection list can badge each one with it.
			branches := []string{preset}
			if preset == "" {
				branches = branchNames(statuses)
			}
			diffByBranch = make(map[string][]domain.EnvFileResult, len(branches))
			portsByBranch = make(map[string]domain.EnvPortPlan, len(branches))
			adoptionByBranch = make(map[string]domain.IsolationAdoptionPlan, len(branches))
			for _, b := range branches {
				adoption, err := isolationAdoption(cfg, envTarget{branch: b, path: worktreePathForBranch(statuses, b)})
				if err != nil {
					return err
				}
				adoptionByBranch[b] = adoption
			}
			if preset != "" {
				if err := recordIsolation(cfg, preset, f.isolation); err != nil {
					return err
				}
			}

			for _, b := range branches {
				// A worktree still to adopt its isolation is scanned without its
				// port pass: resolving one allocates an ordinal, and whether it
				// gets one is the question the wizard is about to ask.
				pending := adoptionByBranch[b].Pending
				adoptedByFlag := b == preset && f.isolation != ""
				scan := branchScan{cfg: cfg, statuses: statuses, branch: b, flags: f, pending: pending, skipRun: pending && !adoptedByFlag}
				files, err := computeBranchDiff(scan)
				if err != nil {
					return err
				}
				diffByBranch[b] = files

				plan, err := computeBranchPorts(scan)
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

	pending := map[string]domain.IsolationAdoptionPlan{}
	if f.isolation == "" {
		for branch, adoption := range adoptionByBranch {
			if adoption.Pending {
				pending[branch] = adoption
			}
		}
	}
	res, err := envwizard.Run(envwizard.RunParams{
		Candidates:       statuses,
		PresetBranch:     preset,
		DiffByBranch:     diffByBranch,
		PortsByBranch:    portsByBranch,
		AdoptionByBranch: pending,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return abortedEnv(cmd, f.format)
	}
	if err != nil {
		return err
	}

	isolation := f.isolation
	if res.Verbatim {
		isolation = domain.IsolationVerbatim
	}
	if res.Adopt {
		isolation = domain.IsolationIsolated
	}
	if err := recordIsolation(cfg, res.Branch, isolation); err != nil {
		return err
	}

	ctx := resolveEnvStrategyAndParent(cfg, res.Branch, f.from)
	worktreePath := worktreePathForBranch(statuses, res.Branch)
	pass := runPass(runPassParams{cfg: cfg, branch: res.Branch, worktreePath: worktreePath, adoption: adoptionByBranch[res.Branch], adopt: isolation != ""})
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
		Ports:              pass.ports,
		Reserved:           pass.reserved,
	})
	if err != nil {
		return err
	}
	return writeEnvResult(cmd, pass.decorate(cfg, result), f.format)
}

type branchScan struct {
	cfg      shared.ConfigResult
	statuses []domain.WorktreeStatus
	branch   string
	flags    envFlags
	// pending is a worktree still to adopt its isolation: scanned without its
	// port pass and without its identity keys, whatever the wizard answers.
	pending bool
	skipRun bool
}

func (s branchScan) reserved() []string {
	if !s.pending {
		return nil
	}
	return domain.WtmOwnedEnvKeys
}

func (s branchScan) ports() envsvc.EnvPortsParams {
	if s.skipRun {
		return envsvc.EnvPortsParams{}
	}
	ports, _ := resolveEnvPorts(s.cfg, s.branch, worktreePathForBranch(s.statuses, s.branch))
	return ports
}

// computeBranchDiff computes one worktree's drift for the wizard (no write).
func computeBranchDiff(scan branchScan) ([]domain.EnvFileResult, error) {
	ctx := resolveEnvStrategyAndParent(scan.cfg, scan.branch, scan.flags.from)
	return envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:             scan.branch,
		MainPath:           scan.cfg.ProjectDir,
		WorktreePath:       worktreePathForBranch(scan.statuses, scan.branch),
		ParentWorktreePath: ctx.ParentPath,
		ParentBranch:       ctx.ParentBranch,
		Files:              scan.cfg.Config.Project.Env.Files,
		Strategy:           ctx.Strategy,
		Mode:               scan.flags.mode,
		Ports:              scan.ports(),
		Reserved:           scan.reserved(),
	})
}

// computeBranchPorts resolves one worktree's port pass for the wizard recap, so
// the apply is announced before it happens rather than discovered after.
func computeBranchPorts(scan branchScan) (domain.EnvPortPlan, error) {
	ports := scan.ports()
	if ports.Empty() {
		return domain.EnvPortPlan{}, nil
	}
	return envsvc.ComputeEnvPorts(ports)
}

type runPassParams struct {
	cfg          shared.ConfigResult
	branch       string
	worktreePath string
	adoption     domain.IsolationAdoptionPlan
	// adopt says the worktree now has an isolation recorded — by --isolation or
	// by the wizard — so a pending adoption is settled.
	adopt bool
}

// envRunPass is the run half of a `wtm env`: the port and owned-value pass, and
// what it says about the worktree's isolation.
type envRunPass struct {
	ports    envsvc.EnvPortsParams
	reserved []string
	warnings []string
	adoption domain.IsolationAdoption
	branch   string
}

// runPass settles the run values of a worktree that chose its isolation, and
// leaves a worktree that never did exactly as it is: no port moved, no compose
// project written, no ordinal allocated — only the keys are reconciled.
func runPass(params runPassParams) envRunPass {
	pass := envRunPass{branch: params.branch}
	if params.adoption.Pending {
		pass.reserved = domain.WtmOwnedEnvKeys
	}
	if params.adoption.Pending && !params.adopt {
		pass.warnings = []string{rules.IsolationNotAdoptedWarning(params.branch)}
		pass.adoption = domain.IsolationNotAdopted
		return pass
	}
	if params.adoption.Pending {
		pass.adoption = domain.IsolationAdopted
	}
	pass.ports, pass.warnings = resolveEnvPorts(params.cfg, params.branch, params.worktreePath)
	return pass
}

// decorate reports the pass on the result. A worktree left on its source's
// values has no isolation to report: calling it isolated would be the one
// thing it is not.
func (p envRunPass) decorate(cfg shared.ConfigResult, result domain.EnvSyncResult) domain.EnvSyncResult {
	result.Warnings = p.warnings
	result.IsolationAdoption = p.adoption
	if p.adoption != domain.IsolationNotAdopted {
		result.Isolation = isolationOf(cfg, p.branch)
	}
	return result
}

type envTarget struct {
	branch string
	path   string
}

func isolationAdoption(cfg shared.ConfigResult, target envTarget) (domain.IsolationAdoptionPlan, error) {
	return worktree.IsolationAdoptionFor(worktree.IsolationAdoptionParams{
		Ref:          worktree.WorktreeRef{ProjectDir: cfg.ProjectDir, StateDir: cfg.StateDir, Branch: target.branch},
		WorktreePath: target.path,
	})
}

// resolveEnvPorts gathers the [[env_port]] links and the offset this worktree
// binds on. A project with no run.toml, or none declared, resolves to nothing and
// the reconciliation runs exactly as it did before — and so does one whose
// run.toml cannot be used: the keys never depend on it, only the port pass is
// skipped, and the warning says why.
func resolveEnvPorts(cfg shared.ConfigResult, branch string, worktreePath string) (envsvc.EnvPortsParams, []string) {
	if err := runconfig.Check(runconfig.CheckParams{StateDir: cfg.StateDir, EnvFiles: cfg.Config.Project.Env.Files}); err != nil {
		return envsvc.EnvPortsParams{}, []string{rules.PortsNotSettledWarning(rules.PortsNotSettledWarningParams{
			Cause:                 err.Error(),
			PortsNotSettledParams: rules.PortsNotSettledParams{Branch: branch, RunConfig: true},
		})}
	}
	ports, err := worktree.ResolveEnvPorts(worktree.ResolveEnvPortsParams{
		ProjectDir:   cfg.ProjectDir,
		StateDir:     cfg.StateDir,
		Branch:       branch,
		WorktreePath: worktreePath,
		EnvFiles:     cfg.Config.Project.Env.Files,
		Global:       cfg.Config.Global,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, []string{rules.PortsNotSettledWarning(rules.PortsNotSettledWarningParams{
			Cause:                 err.Error(),
			PortsNotSettledParams: rules.PortsNotSettledParams{Branch: branch},
		})}
	}
	return ports, nil
}

// recordIsolation writes --isolation, or the recap's verbatim answer, to the
// worktree's record. Nothing given records nothing.
func recordIsolation(cfg shared.ConfigResult, branch string, isolation domain.Isolation) error {
	if isolation == "" {
		return nil
	}
	return worktree.SetIsolation(worktree.SetIsolationParams{
		Ref:       worktree.WorktreeRef{ProjectDir: cfg.ProjectDir, StateDir: cfg.StateDir, Branch: branch},
		Isolation: isolation,
	})
}

func isolationOf(cfg shared.ConfigResult, branch string) domain.Isolation {
	return worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: cfg.ProjectDir, StateDir: cfg.StateDir, Branch: branch})
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
