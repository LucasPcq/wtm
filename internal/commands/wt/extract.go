package wt

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/tui/components"
	extracttui "github.com/LucasPcq/wtm/internal/tui/extract"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newExtractCmd creates the wtm extract subcommand.
func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdExtract + " [source]",
		Short: "Move uncommitted changes to another worktree",
		Long: "Move a subset of a worktree's uncommitted changes to another worktree\n" +
			"(new or existing) to split an oversized PR or isolate unrelated work.\n\n" +
			"The source worktree is the first thing chosen: pass its branch as [source],\n" +
			"or omit it to pick interactively from the worktrees that have changes. A source\n" +
			"is required when there is no terminal or with --output json.\n\n" +
			"A --to branch that already exists locally is checked out as-is, keeping its\n" +
			"commits. Its parent can't be inferred, so --from then names the branch recorded\n" +
			"for `wtm sync` — asked in the wizard, required without it.\n\n" +
			"Untracked files are listed one by one, including inside brand-new directories,\n" +
			"so you can take part of a new folder; gitignored files are never listed.\n\n" +
			"On conflict it aborts by default, leaving the source intact; --on-conflict resolve\n" +
			"applies conflict markers in the target so you can resolve them like a rebase.\n" +
			"A file that merely already exists in the target counts as a conflict too.",
		Args: cobra.MaximumNArgs(1),
		RunE: runExtract,
	}

	cmd.Flags().StringSlice(domain.FlagFiles, nil, "Files to extract, or a directory to take everything below it (skips interactive selection)")
	cmd.Flags().String(domain.FlagTo, "", "Target worktree branch; created if it does not exist")
	cmd.Flags().String(domain.FlagFrom, "", "Parent branch when creating the target worktree")
	cmd.Flags().Bool(domain.FlagFF, false, "Fast-forward the parent branch to origin before creating the target (non-interactive; skipped when it has diverged)")
	cmd.Flags().Bool(domain.FlagKeep, false, "Copy instead of move (keep the changes in the source)")
	cmd.Flags().String(domain.FlagOnConflict, "", "On conflict: abort (default) or resolve (write conflict markers in the target)")
	cmd.Flags().BoolP(domain.FlagYes, "y", false, "Skip all prompts; resolve every decision from flags and safe defaults (requires a source arg, --files and --to; --from is also required when --to already exists locally; errors if a selection is missing)")
	shared.AddOutputFlag(cmd)

	return cmd
}

func runExtract(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	if err := validateOnConflict(cmd); err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)

	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--output json requires --%s (prompts cannot run in JSON mode)", domain.FlagYes)
	}

	// --yes runs fully unattended: it forces the non-interactive path so a missing
	// required selection (source / --files / --to) errors naming the flag instead of
	// opening a picker. Decisions with a safe default (mode, on-conflict) still default.
	interactive := isInteractive() && rules.IsHumanFormat(format) && !yes

	sourceArg := ""
	if len(args) == 1 {
		sourceArg = args[0]
	}
	// The source is picked interactively only when it was not given as an argument
	// and there is a human terminal to pick on. Otherwise it must be explicit —
	// aligning extract with sync/prune (no cwd default).
	needSource := sourceArg == "" && interactive
	if sourceArg == "" && !interactive {
		return domain.ErrExtractSourceRequired
	}

	// worktree.List runs a git status per worktree, so on a large repo this is
	// the wait the command used to spend saying nothing.
	var statuses []domain.WorktreeStatus
	if err := components.RunLoading(components.LoadingParams{
		Message: domain.ExtractScanLoading,
		Animate: shared.Animate(cmd, interactive),
		Work: func() error {
			var listErr error
			statuses, listErr = worktree.List(domain.ListParams{
				ProjectDir: cfg.ProjectDir,
				StateDir:   cfg.StateDir,
				Config:     cfg.Config,
			})
			if listErr != nil {
				return fmt.Errorf("list worktrees: %w", listErr)
			}
			return nil
		},
	}); err != nil {
		return err
	}

	source, err := resolveSource(resolveSourceParams{
		sourceArg:  sourceArg,
		needSource: needSource,
		cfg:        cfg,
		statuses:   statuses,
	})
	if errors.Is(err, domain.ErrNoDirtyWorktrees) {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.Message(w, domain.ErrNoDirtyWorktrees.Error())
		})
		return nil
	}
	if err != nil {
		return err
	}

	// Fixed source (argument): its changes are known up front, so the empty-changes
	// case short-circuits here. A picked source loads its changes lazily instead.
	if !needSource {
		if err := components.RunLoading(components.LoadingParams{
			Message: domain.ExtractScanLoading,
			Animate: shared.Animate(cmd, interactive),
			Work: func() error {
				var filesErr error
				source.available, filesErr = listExtractFiles(source.path)
				return filesErr
			},
		}); err != nil {
			return err
		}
		if len(source.available) == 0 {
			if format == domain.OutputJSON {
				return output.WriteExtractJSON(cmd.OutOrStdout(), domain.ExtractResult{Files: []domain.ExtractFile{}})
			}
			output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
				output.Message(w, domain.ErrNoChangesToExtract.Error())
			})
			return nil
		}
	}

	sel, err := resolveSelectionAndTarget(resolveParams{
		cmd:         cmd,
		cfg:         cfg,
		statuses:    statuses,
		source:      source,
		needSource:  needSource,
		interactive: interactive,
		human:       rules.IsHumanFormat(format),
		yes:         yes,
		loadFiles:   extractFilesLoader(statuses),
	})
	if errors.Is(err, domain.ErrUserAborted) {
		if rules.IsHumanFormat(format) {
			output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
				output.Unchanged(w, domain.AbortedMessage)
			})
		}
		return nil
	}
	if err != nil {
		return err
	}

	conflictMode, err := resolveConflictMode(resolveConflictModeParams{
		cmd:        cmd,
		sourcePath: sel.sourcePath,
		target:     sel.target,
		selected:   sel.selected,
		yes:        yes,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		if rules.IsHumanFormat(format) {
			output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
				output.Unchanged(w, domain.AbortedMessage)
			})
		}
		return nil
	}
	if err != nil {
		return err
	}

	result, err := worktree.Extract(domain.ExtractParams{
		SourcePath:   sel.sourcePath,
		SourceBranch: sel.sourceBranch,
		TargetPath:   sel.target.path,
		TargetBranch: sel.target.branch,
		Files:        sel.selected,
		Keep:         sel.keep,
		ConflictMode: conflictMode,
	})
	if err != nil {
		return err
	}

	if format == domain.OutputJSON {
		return output.WriteExtractJSON(cmd.OutOrStdout(), result)
	}
	if len(result.Conflicts) > 0 {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.PrintExtractConflicts(w, result)
		})
		return nil
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.PrintExtractResult(w, output.ExtractResultParams{
			Result:  result,
			EnvNote: rules.EnvPortSettlementNote(sel.target.envPorts),
		})
	})
	return nil
}

// extractSource holds the resolved source worktree: a fixed path/branch when an
// argument was given, or the candidate worktrees to pick from otherwise.
type extractSource struct {
	path      string
	branch    string
	available []domain.ExtractFile
	// dirty are the candidate source worktrees (those with changes) for the picker,
	// set only when the source is picked interactively.
	dirty []domain.WorktreeStatus
}

type resolveSourceParams struct {
	sourceArg  string
	needSource bool
	cfg        shared.ConfigResult
	statuses   []domain.WorktreeStatus
}

// resolveSource resolves the source worktree from the argument (exact branch
// match, like --to), or gathers the worktrees with changes for the interactive
// picker. Returns ErrNoDirtyWorktrees when the picker would be empty.
func resolveSource(p resolveSourceParams) (extractSource, error) {
	if !p.needSource {
		wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: p.cfg.ProjectDir,
			Branch:     p.sourceArg,
		})
		if err != nil {
			return extractSource{}, fmt.Errorf("source worktree %q: %w", p.sourceArg, err)
		}
		return extractSource{path: wt.Path, branch: wt.Branch}, nil
	}

	dirty := dirtyWorktrees(p.statuses)
	if len(dirty) == 0 {
		return extractSource{}, domain.ErrNoDirtyWorktrees
	}
	return extractSource{dirty: dirty}, nil
}

// extractFilesLoader builds the closure the wizard uses to lazily list a picked
// source worktree's changes, keeping the TUI free of git/service imports.
func extractFilesLoader(statuses []domain.WorktreeStatus) func(branch string) []domain.ExtractFile {
	return func(branch string) []domain.ExtractFile {
		path := worktreePathForBranch(statuses, branch)
		if path == "" {
			return nil
		}
		files, err := listExtractFiles(path)
		if err != nil {
			return nil
		}
		return files
	}
}

// dirtyWorktrees keeps the worktrees with uncommitted changes — the only valid
// extract sources, so the picker never dead-ends on an empty selection.
func dirtyWorktrees(statuses []domain.WorktreeStatus) []domain.WorktreeStatus {
	dirty := make([]domain.WorktreeStatus, 0, len(statuses))
	for _, s := range statuses {
		if s.IsDirty {
			dirty = append(dirty, s)
		}
	}
	return dirty
}

// worktreePathForBranch returns the worktree path backing a branch, or "".
func worktreePathForBranch(statuses []domain.WorktreeStatus, branch string) string {
	for _, s := range statuses {
		if s.Branch == branch {
			return s.Path
		}
	}
	return ""
}

type resolveConflictModeParams struct {
	cmd        *cobra.Command
	sourcePath string
	target     extractTarget
	selected   []domain.ExtractFile
	// yes is --yes: the on-conflict decision is a prompt like any other, so under
	// --yes it resolves to the safe default (abort) unless --on-conflict was given.
	yes bool
}

// resolveConflictMode decides what Extract does on conflict. No conflict → abort
// (unused). On conflict: honor --on-conflict, else prompt on a TTY, else default
// to abort. Returns ErrUserAborted when the user declines the prompt.
func resolveConflictMode(p resolveConflictModeParams) (string, error) {
	conflicts := worktree.ConflictingFiles(domain.ConflictCheckParams{
		SourcePath: p.sourcePath,
		TargetPath: p.target.path,
		Files:      p.selected,
	})
	if len(conflicts) == 0 {
		return domain.OnConflictAbort, nil
	}

	// The flag value is validated at command entry, so it is safe to trust here.
	if p.cmd.Flags().Changed(domain.FlagOnConflict) {
		mode, _ := p.cmd.Flags().GetString(domain.FlagOnConflict)
		return mode, nil
	}

	// --yes runs without decision prompts: fall back to the safe default (abort).
	if p.yes || !isInteractive() {
		return domain.OnConflictAbort, nil
	}

	resolve, err := extracttui.ConfirmResolve(conflicts, p.target.branch)
	if err != nil {
		return "", err
	}
	if !resolve {
		return "", domain.ErrUserAborted
	}
	return domain.OnConflictResolve, nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// validateOnConflict rejects an invalid --on-conflict value at the command
// boundary, regardless of whether the extraction ends up conflicting.
func validateOnConflict(cmd *cobra.Command) error {
	if !cmd.Flags().Changed(domain.FlagOnConflict) {
		return nil
	}
	mode, _ := cmd.Flags().GetString(domain.FlagOnConflict)
	if mode != domain.OnConflictAbort && mode != domain.OnConflictResolve {
		return fmt.Errorf("invalid --%s value %q: use %s or %s",
			domain.FlagOnConflict, mode, domain.OnConflictAbort, domain.OnConflictResolve)
	}
	return nil
}

// listExtractFiles returns the uncommitted files of a worktree classified for
// extraction.
func listExtractFiles(sourcePath string) ([]domain.ExtractFile, error) {
	modified, err := infra.ListModifiedFiles(infra.ListModifiedFilesParams{WorktreePath: sourcePath})
	if err != nil {
		return nil, fmt.Errorf("list modified files: %w", err)
	}

	files := make([]domain.ExtractFile, 0, len(modified))
	for _, m := range modified {
		files = append(files, domain.ExtractFile{
			Path:     m.Path,
			OrigPath: m.OrigPath,
			Status:   rules.ClassifyExtractStatus(m.Status),
		})
	}
	return files, nil
}

type resolveParams struct {
	cmd        *cobra.Command
	cfg        shared.ConfigResult
	statuses   []domain.WorktreeStatus
	source     extractSource
	needSource bool
	// interactive is false under --yes, no TTY, or --output json: a required
	// selection with no flag then errors instead of opening a picker.
	interactive bool
	// human gates the on_create hooks section header (text format, incl. --yes),
	// mirroring create/clean; it is broader than interactive.
	human     bool
	yes       bool
	loadFiles func(branch string) []domain.ExtractFile
}

// extractSelection is the fully-resolved plan: which source, its path, the files,
// the target, and move-vs-copy.
type extractSelection struct {
	sourceBranch string
	sourcePath   string
	selected     []domain.ExtractFile
	target       extractTarget
	keep         bool
}

// resolveSelectionAndTarget resolves the source (when picked), the files to
// extract, and the destination worktree, from flags or the interactive wizard.
// The "create new worktree" sub-flow is embedded in the wizard (see extracttui),
// so one combined recap covers both create and extract and there is no re-entry loop.
func resolveSelectionAndTarget(p resolveParams) (extractSelection, error) {
	filesFlag, _ := p.cmd.Flags().GetStringSlice(domain.FlagFiles)
	toFlag, _ := p.cmd.Flags().GetString(domain.FlagTo)
	keepFlag, _ := p.cmd.Flags().GetBool(domain.FlagKeep)
	needFiles := len(filesFlag) == 0
	needTarget := toFlag == ""
	needMode := !p.cmd.Flags().Changed(domain.FlagKeep) && p.interactive

	// Non-interactive (--yes / no TTY / --output json): a required selection with no
	// flag is an error naming the flag, never a picker. Mode has a safe default
	// (move), so it is not required here.
	if !p.interactive {
		if needFiles {
			return extractSelection{}, domain.ErrExtractFilesRequired
		}
		if needTarget {
			return extractSelection{}, domain.ErrExtractTargetRequired
		}
	}

	wizard, err := runWizard(runWizardParams{
		cmd:        p.cmd,
		cfg:        p.cfg,
		statuses:   p.statuses,
		source:     p.source,
		loadFiles:  p.loadFiles,
		needSource: p.needSource,
		needFiles:  needFiles,
		needTarget: needTarget,
		needMode:   needMode,
		filesFlag:  filesFlag,
		toFlag:     toFlag,
		keepFlag:   keepFlag,
	})
	if err != nil {
		return extractSelection{}, err
	}

	// The source is fixed by the argument, or picked on the wizard's first step.
	sourceBranch := p.source.branch
	sourcePath := p.source.path
	available := p.source.available
	if p.needSource {
		sourceBranch = wizard.SourceBranch
		sourcePath = worktreePathForBranch(p.statuses, sourceBranch)
		available, err = listExtractFiles(sourcePath)
		if err != nil {
			return extractSelection{}, err
		}
	}

	selectedPaths := filesFlag
	if needFiles {
		selectedPaths = wizard.Files
	}
	selected, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{
		Available: available,
		Paths:     selectedPaths,
	})
	if err != nil {
		return extractSelection{}, err
	}

	keep := keepFlag
	if needMode {
		keep = wizard.Keep
	}

	target, err := resolveTarget(resolveTargetParams{
		cmd:          p.cmd,
		cfg:          p.cfg,
		sourceBranch: sourceBranch,
		choice:       wizard.Target,
		create:       wizard.Create,
		interactive:  p.interactive,
		human:        p.human,
	})
	if err != nil {
		return extractSelection{}, err
	}
	return extractSelection{
		sourceBranch: sourceBranch,
		sourcePath:   sourcePath,
		selected:     selected,
		target:       target,
		keep:         keep,
	}, nil
}

type runWizardParams struct {
	cmd        *cobra.Command
	cfg        shared.ConfigResult
	statuses   []domain.WorktreeStatus
	source     extractSource
	loadFiles  func(branch string) []domain.ExtractFile
	needSource bool
	needFiles  bool
	needTarget bool
	needMode   bool
	// filesFlag/toFlag/keepFlag are the flag values, forwarded to the wizard so the
	// recap names every part of the plan even for the steps a flag skipped.
	filesFlag []string
	toFlag    string
	keepFlag  bool
}

// runWizard runs the unified source+file+target+create+mode selection wizard,
// skipping any step already resolved from a flag or argument. Returns a zero
// result when nothing is needed. When the target is picked it also supplies the
// embedded create sub-flow.
func runWizard(params runWizardParams) (extracttui.RunResult, error) {
	if !params.needSource && !params.needFiles && !params.needTarget && !params.needMode {
		return extracttui.RunResult{}, nil
	}

	var create newpicker.WizardParams
	if params.needTarget {
		create = extractCreateParams(params.cfg, params.source.branch)
	}

	return extracttui.Run(extracttui.RunParams{
		Files:        params.source.available,
		Worktrees:    params.statuses,
		SourceBranch: params.source.branch,
		Sources:      params.source.dirty,
		LoadFiles:    params.loadFiles,
		NeedSource:   params.needSource,
		NeedFiles:    params.needFiles,
		NeedTarget:   params.needTarget,
		NeedMode:     params.needMode,
		FixedFiles:   params.filesFlag,
		FixedTarget:  params.toFlag,
		FixedKeep:    params.keepFlag,
		Create:       create,
	})
}

// extractCreateParams builds the create sub-flow config embedded in the extract
// wizard: the source is picked (not fixed), the branch is asked, and env is left
// at the config default. The deciders guard against an empty source — the create
// steps are auto-skipped (and their source is "") when the target is an existing
// worktree.
func extractCreateParams(cfg shared.ConfigResult, sourceBranch string) newpicker.WizardParams {
	// Cached per branch name for the run, like create's own wizard: the recap and
	// the source-update step both classify the same branch repeatedly.
	cachedTarget := memoizedTarget(cfg.ProjectDir)
	target := func(b string) domain.BranchTarget {
		if b == "" {
			return domain.BranchTarget{}
		}
		return cachedTarget(b)
	}
	return newpicker.WizardParams{
		ProjectDir:     cfg.ProjectDir,
		Branches:       branchCandidates(cfg.ProjectDir),
		DefaultBranch:  defaultParent(defaultParentParams{cfg: cfg, sourceBranch: sourceBranch}),
		ConfigStrategy: cfg.Config.Project.Env.Strategy,
		IncludeBranch:  true,
		// Asked only where something follows a port; the TUI is handed the answer,
		// never the config it comes from.
		IncludeEnvPorts: envports.Linked(shared.FlowContext(cfg)),
		Target:          target,
		SourceUpdate: func(up newpicker.SourceUpdateParams) newpicker.SourceUpdatePrompt {
			if up.Source == "" {
				return newpicker.SourceUpdatePrompt{}
			}
			return sourceUpdatePrompt(sourceUpdatePromptParams{ProjectDir: cfg.ProjectDir, Target: target, Update: up})
		},
		EnvFallback: func(source, _ string) (bool, components.NewConfirmParams) {
			if source == "" {
				return false, components.NewConfirmParams{}
			}
			return envFallbackPrompt(cfg.ProjectDir, cfg.Config, source, "")
		},
	}
}

type extractTarget struct {
	path   string
	branch string
	// envPorts is what the port pass did in a worktree this extraction created,
	// zero for one that already existed and was never provisioned.
	envPorts domain.EnvPortSettlement
}

type resolveTargetParams struct {
	cmd          *cobra.Command
	cfg          shared.ConfigResult
	sourceBranch string
	choice       extracttui.TargetChoice
	// create holds the new-worktree answers collected in the combined wizard, used
	// when choice.CreateNew.
	create newpicker.WizardResult
	// interactive is false under --yes / no TTY / --output json: the source
	// fast-forward is then a non-prompting decision (skip unless --ff).
	interactive bool
	// human gates the on_create hooks section header (text format, incl. --yes).
	human bool
}

// resolveTarget resolves the destination worktree from the --to flag or the
// wizard choice, creating a new worktree when requested.
func resolveTarget(params resolveTargetParams) (extractTarget, error) {
	toFlag, _ := params.cmd.Flags().GetString(domain.FlagTo)
	if toFlag != "" {
		if wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: params.cfg.ProjectDir,
			Branch:     toFlag,
		}); err == nil {
			return extractTarget{path: wt.Path, branch: wt.Branch}, nil
		}
		target := branch.Target(branch.BranchParams{ProjectDir: params.cfg.ProjectDir, Branch: toFlag})
		fromFlag, _ := params.cmd.Flags().GetString(domain.FlagFrom)
		// --to has no wizard step of its own (it fully resolves the target), so the
		// parent guard applies whether or not the run is otherwise interactive — same
		// rationale as create's non-interactive path: an existing branch was created
		// outside wtm, so nothing says what it was branched off.
		if rules.ParentMustBeExplicit(target.State) && fromFlag == "" {
			return extractTarget{}, fmt.Errorf(domain.ParentRequiredFmt, toFlag, domain.FlagFrom)
		}
		fromBranch := defaultParent(defaultParentParams{cfg: params.cfg, sourceBranch: params.sourceBranch, override: fromFlag})
		// --ff (and the interactive fast-forward offer) updates whichever branch the
		// worktree actually starts from: the parent for a branch git is about to
		// create, the target itself when it already exists locally.
		ffSubjectBranch := ffSubject(ffSubjectParams{Target: target, FromBranch: fromBranch, Branch: toFlag})
		// The fast-forward prompt is a decision like any other: only interactive runs
		// ask. Under --yes / no TTY / JSON it is non-prompting — fast-forward only when
		// --ff was passed, otherwise leave the branch as-is.
		if params.interactive {
			if !maybeFastForwardSource(fastForwardSourceParams{
				Cmd:        params.cmd,
				ProjectDir: params.cfg.ProjectDir,
				Source:     ffSubjectBranch,
			}) {
				return extractTarget{}, domain.ErrUserAborted
			}
		} else if ffFlag, _ := params.cmd.Flags().GetBool(domain.FlagFF); ffFlag {
			_ = branch.FastForwardIfBehind(branch.BranchParams{ProjectDir: params.cfg.ProjectDir, Branch: ffSubjectBranch})
		}
		// --to fully resolves the target, so it poses no wizard and no env-ports
		// step: the safe default answers for it.
		return createTarget(createTargetParams{
			cmd:            params.cmd,
			showHeader:     params.human,
			cfg:            params.cfg,
			branch:         toFlag,
			fromBranch:     fromBranch,
			adjustEnvPorts: true,
			interactive:    params.interactive,
		})
	}

	if !params.choice.CreateNew {
		wt, findErr := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
			ProjectDir: params.cfg.ProjectDir,
			Branch:     params.choice.Branch,
		})
		if findErr != nil {
			return extractTarget{}, findErr
		}
		return extractTarget{path: wt.Path, branch: wt.Branch}, nil
	}

	// "Create new" — the branch/source/fast-forward were collected in the combined
	// wizard (already confirmed on its recap). Only the accepted fast-forward is
	// executed here; its failure-recovery prompt is a legitimate post-exec standalone.
	if params.create.FastForwardBranch != "" &&
		!executeFastForwardSource(fastForwardSourceParams{
			Cmd:        params.cmd,
			ProjectDir: params.cfg.ProjectDir,
			Source:     params.create.FastForwardBranch,
		}) {
		return extractTarget{}, domain.ErrUserAborted
	}
	return createTarget(createTargetParams{
		cmd:            params.cmd,
		showHeader:     params.human,
		cfg:            params.cfg,
		branch:         params.create.BranchName,
		fromBranch:     params.create.FromBranch,
		adjustEnvPorts: params.create.AdjustEnvPorts,
		interactive:    params.interactive,
	})
}

type defaultParentParams struct {
	cfg          shared.ConfigResult
	sourceBranch string
	override     string
}

// defaultParent resolves the parent branch for a new target worktree: the
// explicit override, else the source worktree's recorded parent, else the
// configured base branch.
func defaultParent(params defaultParentParams) string {
	if params.override != "" {
		return params.override
	}
	if parent := worktree.ParentBranch(worktree.ParentBranchParams{
		StateDir: params.cfg.StateDir,
		Branch:   params.sourceBranch,
	}); parent != "" {
		return parent
	}
	return params.cfg.Config.Project.Worktrees.BaseBranch
}

type createTargetParams struct {
	cmd        *cobra.Command
	showHeader bool
	cfg        shared.ConfigResult
	branch     string
	fromBranch string
	// adjustEnvPorts is the wizard's answer to the env-ports step, true wherever
	// it was never posed — a .env left pointing at another worktree's services is
	// not the safer outcome.
	adjustEnvPorts bool
	interactive    bool
}

func createTarget(params createTargetParams) (extractTarget, error) {
	res, err := worktree.Create(domain.CreateParams{
		ProjectDir: params.cfg.ProjectDir,
		StateDir:   params.cfg.StateDir,
		Branch:     params.branch,
		FromBranch: params.fromBranch,
		Config:     params.cfg.Config,
		SkipHooks:  true,
	})
	if err != nil {
		return extractTarget{}, err
	}

	// Before the hooks: one of them may read the .env, and it has to read the
	// ports this worktree binds rather than the ones it was copied from.
	format, _ := params.cmd.Flags().GetString(domain.FlagOutput)
	settlement, err := envports.Settle(envports.Params{
		Context:      shared.FlowContext(params.cfg),
		Branch:       res.Branch,
		WorktreePath: res.Path,
		Rewrite:      params.adjustEnvPorts,
		Presenter:    shared.NewPresenter(params.cmd, format),
	})
	if err != nil {
		return extractTarget{}, err
	}

	// on_create hooks as a distinct, titled phase (shared with create/checkout).
	if err := shared.RunCreateHooksPhase(shared.CreateHooksPhaseParams{
		StateDir:     params.cfg.StateDir,
		Cmd:          params.cmd,
		ShowHeader:   params.showHeader,
		ProjectDir:   params.cfg.ProjectDir,
		WorktreePath: res.Path,
		Branch:       res.Branch,
		FromBranch:   params.fromBranch,
		Hooks:        params.cfg.Config.Project.Hooks.OnCreate,
	}); err != nil {
		return extractTarget{}, err
	}
	return extractTarget{path: res.Path, branch: res.Branch, envPorts: settlement}, nil
}
