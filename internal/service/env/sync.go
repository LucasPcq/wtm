package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// envPaths are the resolved locations + strategy shared by the compute and apply
// passes. Paths are resolved by the caller (command layer) so this service stays
// free of git / worktree imports. Strategy is already resolved (memorized strategy
// with any --from override applied).
type envPaths struct {
	MainPath           string
	WorktreePath       string
	ParentWorktreePath string // "" when the parent has no local worktree
	ParentBranch       string // recorded parent branch (for display), "" if none
	Strategy           domain.EnvStrategy
	Mode               domain.EnvMode
	// Ports are the [[env_port]] links of the project, empty when it declares
	// none. They are never a value source: they normalize the comparison so a
	// value differing only by the worktree's offset is not a conflict, and the
	// rewrite itself happens after every file is reconciled.
	Ports EnvPortsParams
}

// EnvResolution is the decision set for one file: how to settle each conflict, the
// values supplied for missing/edited keys, and whether to prune orphans. It is
// produced non-interactively from flags (SyncEnv) or interactively by the wizard
// (envwizard) and consumed by ApplyEnvSync.
type EnvResolution struct {
	Decisions    map[string]domain.EnvConflictDecision
	FilledValues map[string]string
	// Prune drops every orphan (the non-interactive --prune); PruneKeys drops only
	// the named orphans; SkipKeys omits an addition the user chose not to add
	// (per-key choices from the interactive wizard).
	Prune     bool
	PruneKeys map[string]bool
	SkipKeys  map[string]bool
}

// computedFile is the per-file working state between diff computation and apply.
type computedFile struct {
	file           domain.EnvFile
	source         string
	parentFallback bool
	child          []domain.EnvLine
	diff           domain.EnvDiff
	// unresolvable marks a configured file that exists nowhere: not in the
	// worktree, in no value source, and with no template to scaffold from. A
	// fresh project has no source either, but it does have a template — this is
	// a config.toml entry pointing at nothing.
	unresolvable bool
}

// ComputeEnvParams holds the inputs to compute a worktree's env drift, without any
// write. Used by the interactive wizard (to render the drift) and the drift badges.
type ComputeEnvParams struct {
	Branch             string
	MainPath           string
	WorktreePath       string
	ParentWorktreePath string
	ParentBranch       string
	Files              []domain.EnvFile
	Strategy           domain.EnvStrategy
	Mode               domain.EnvMode
	Ports              EnvPortsParams
}

// ComputeEnvDiff reconciles every configured env file against its template and
// value sources and returns the per-file drift, writing nothing. Applied is always
// false.
func ComputeEnvDiff(params ComputeEnvParams) ([]domain.EnvFileResult, error) {
	if len(params.Files) == 0 {
		return nil, domain.ErrEnvNoFiles
	}
	paths := envPaths{
		MainPath:           params.MainPath,
		WorktreePath:       params.WorktreePath,
		ParentWorktreePath: params.ParentWorktreePath,
		ParentBranch:       params.ParentBranch,
		Strategy:           params.Strategy,
		Mode:               params.Mode,
		Ports:              params.Ports,
	}

	out := make([]domain.EnvFileResult, 0, len(params.Files))
	for _, f := range params.Files {
		c, err := computeFile(paths, f)
		if err != nil {
			return nil, err
		}
		out = append(out, fileResult(paths, c, false))
	}
	return out, nil
}

// ApplyEnvSyncParams holds the inputs to apply a set of resolutions to a worktree.
// Resolutions is keyed by file target; a target with no entry gets the zero
// resolution (safe additions only — no conflict overwrite, no prune).
type ApplyEnvSyncParams struct {
	Branch             string
	MainPath           string
	WorktreePath       string
	ParentWorktreePath string
	ParentBranch       string
	Files              []domain.EnvFile
	Strategy           domain.EnvStrategy
	Mode               domain.EnvMode
	Ports              EnvPortsParams
	Resolutions        map[string]EnvResolution
	// SkipPortRewrite is the user declining the port pass. The links stay in
	// Ports: they still normalize the diff, they just do not get written.
	SkipPortRewrite bool
}

// ApplyEnvSync recomputes each file's drift and writes the reconciled content,
// applying the matching resolution. Files whose reconciled content is unchanged are
// left untouched. Returns the full result.
func ApplyEnvSync(params ApplyEnvSyncParams) (domain.EnvSyncResult, error) {
	if len(params.Files) == 0 {
		return domain.EnvSyncResult{}, domain.ErrEnvNoFiles
	}
	paths := envPaths{
		MainPath:           params.MainPath,
		WorktreePath:       params.WorktreePath,
		ParentWorktreePath: params.ParentWorktreePath,
		ParentBranch:       params.ParentBranch,
		Strategy:           params.Strategy,
		Mode:               params.Mode,
		Ports:              params.Ports,
	}

	files := make([]domain.EnvFileResult, 0, len(params.Files))
	for _, f := range params.Files {
		c, err := computeFile(paths, f)
		if err != nil {
			return domain.EnvSyncResult{}, err
		}
		applied, err := applyFile(paths, c, params.Resolutions[f.Target])
		if err != nil {
			return domain.EnvSyncResult{}, err
		}
		files = append(files, fileResult(paths, c, applied))
	}

	ports, err := settleEnvPorts(settleEnvPortsParams{Ports: params.Ports, Write: !params.SkipPortRewrite, Owned: true})
	if err != nil {
		return domain.EnvSyncResult{}, err
	}

	return domain.EnvSyncResult{
		Branch: params.Branch,
		Mode:   params.Mode,
		Check:  false,
		Files:  files,
		Ports:  ports,
	}, nil
}

// SyncEnvParams holds the inputs for the non-interactive reconciliation path
// (report / JSON / --yes / --check). Interactive resolution goes through
// ComputeEnvDiff + ApplyEnvSync instead.
type SyncEnvParams struct {
	Branch             string
	MainPath           string
	WorktreePath       string
	ParentWorktreePath string
	ParentBranch       string
	Files              []domain.EnvFile
	Strategy           domain.EnvStrategy
	Mode               domain.EnvMode
	Ports              EnvPortsParams
	Prune              bool
	Check              bool
	// OnConflict is the conflict decision applied to every conflict (keep — the safe
	// default — or overwrite).
	OnConflict domain.EnvConflictDecision
}

// SyncEnv reconciles a worktree's env files non-interactively: it applies safe
// additions plus any flag-driven decisions (OnConflict, Prune), writing changed
// files. With Check set it computes and returns the drift without writing anything.
func SyncEnv(params SyncEnvParams) (domain.EnvSyncResult, error) {
	if len(params.Files) == 0 {
		return domain.EnvSyncResult{}, domain.ErrEnvNoFiles
	}
	paths := envPaths{
		MainPath:           params.MainPath,
		WorktreePath:       params.WorktreePath,
		ParentWorktreePath: params.ParentWorktreePath,
		ParentBranch:       params.ParentBranch,
		Strategy:           params.Strategy,
		Mode:               params.Mode,
		Ports:              params.Ports,
	}

	files := make([]domain.EnvFileResult, 0, len(params.Files))
	for _, f := range params.Files {
		c, err := computeFile(paths, f)
		if err != nil {
			return domain.EnvSyncResult{}, err
		}
		applied := false
		if !params.Check {
			applied, err = applyFile(paths, c, flagResolution(params, c.diff))
			if err != nil {
				return domain.EnvSyncResult{}, err
			}
		}
		files = append(files, fileResult(paths, c, applied))
	}

	// The ports come last, on files that are now reconciled: the value sources
	// carry another worktree's port, so applying the offset before the merge
	// would only see it overwritten.
	ports, err := settleEnvPorts(settleEnvPortsParams{Ports: params.Ports, Write: !params.Check, Owned: !params.Check})
	if err != nil {
		return domain.EnvSyncResult{}, err
	}

	return domain.EnvSyncResult{
		Branch: params.Branch,
		Mode:   params.Mode,
		Check:  params.Check,
		Files:  files,
		Ports:  ports,
	}, nil
}

type settleEnvPortsParams struct {
	Ports EnvPortsParams
	// Write is the port pass itself. Owned says the worktree identity may still
	// be written when that pass is not: declining the port rewrite is an answer
	// about ports, and which worktree this is was never one of the questions.
	// Both are false on a --check run, which writes nothing at all.
	Write bool
	Owned bool
}

// settleEnvPorts applies the worktree's offset to the linked values, or merely
// resolves what it would do when the caller is reporting rather than writing —
// a --check run, or one where the user declined the pass. Either way the links
// still feed the diff's comparison, which is why they are never simply dropped.
func settleEnvPorts(params settleEnvPortsParams) (domain.EnvPortPlan, error) {
	if params.Ports.Empty() {
		return domain.EnvPortPlan{}, nil
	}
	if !params.Write {
		plan, err := ComputeEnvPorts(params.Ports)
		if err != nil || !params.Owned {
			return plan, err
		}
		return plan, ApplyOwnedEnv(params.Ports)
	}
	plan, err := ApplyEnvPorts(params.Ports)
	plan.Applied = err == nil
	return plan, err
}

// fileResult projects a computed file into its result form.
func fileResult(paths envPaths, c computedFile, applied bool) domain.EnvFileResult {
	parentBranch := ""
	if paths.Strategy == domain.EnvStrategyParent {
		parentBranch = paths.ParentBranch
	}
	return domain.EnvFileResult{
		Target:         c.file.Target,
		Strategy:       paths.Strategy,
		Source:         c.source,
		Diff:           c.diff,
		Applied:        applied,
		ParentBranch:   parentBranch,
		ParentFallback: c.parentFallback,
		Unresolvable:   c.unresolvable,
	}
}

// computeFile reads the four documents for one env file according to the strategy
// and computes its diff.
func computeFile(paths envPaths, f domain.EnvFile) (computedFile, error) {
	child, err := readEnvFile(filepath.Join(paths.WorktreePath, f.Target))
	if err != nil {
		return computedFile{}, err
	}
	template, err := templateLines(paths.WorktreePath, f)
	if err != nil {
		return computedFile{}, err
	}

	parent, main, source, fallback, err := valueSources(paths, f)
	if err != nil {
		return computedFile{}, err
	}

	diff := rules.DiffEnv(rules.EnvDiffParams{
		Template:   template,
		Parent:     parent,
		Main:       main,
		Child:      child,
		Mode:       paths.Mode,
		PortValues: EnvValueRefsFor(paths.Ports, f.Target),
		PortBlock:  paths.Ports.Block,
		Owned:      rules.EnvValueOwnedKeys(paths.Ports.ValueLinks, f.Target),
	})

	return computedFile{
		file:           f,
		source:         source,
		parentFallback: fallback,
		child:          child,
		diff:           diff,
		unresolvable:   child == nil && template == nil && parent == nil && main == nil,
	}, nil
}

// valueSources feeds the value document per strategy — one source only, never a
// silent mix. The strategy names the source: `example` reads no real values
// (template placeholders only); `main` reads the main worktree; `parent` reads the
// parent worktree's file **and only that** (a key the parent lacks stays unresolved,
// it is not pulled from main). The single exception, mirroring `wtm create`: when
// there is no readable parent file at all — the parent worktree is absent, or that
// file does not exist in it — it falls back to main (LUC-62), flagged. `--from` is
// the explicit way to pick a different source.
func valueSources(paths envPaths, f domain.EnvFile) (parent, main []domain.EnvLine, source string, fallback bool, err error) {
	switch paths.Strategy {
	case domain.EnvStrategyMain:
		main, err = readEnvFile(filepath.Join(paths.MainPath, f.Target))
		if err != nil {
			return nil, nil, "", false, err
		}
		if main == nil {
			return nil, nil, domain.EnvSourceLabelNone, false, nil
		}
		return nil, main, domain.EnvSourceLabelMain, false, nil

	case domain.EnvStrategyParent:
		if paths.ParentWorktreePath == "" {
			return parentFallback(paths, f, worktreeAbsentLabel(paths.ParentBranch))
		}
		parent, err = readEnvFile(filepath.Join(paths.ParentWorktreePath, f.Target))
		if err != nil {
			return nil, nil, "", false, err
		}
		if parent == nil {
			// Parent worktree present but this file is not in it → fall back to main,
			// like create's per-file fileExists fallback.
			return parentFallback(paths, f, fileAbsentLabel(paths.ParentBranch, f.Target))
		}
		// Parent present: strict — values come from the parent worktree only.
		return parent, nil, parentSourceLabel(paths.ParentBranch), false, nil

	default: // EnvStrategyExample
		return nil, nil, domain.EnvSourceLabelTemplate, false, nil
	}
}

// parentFallback reads main as the fallback value source for a "parent" strategy with
// no readable parent file. When main also has no .env there is nothing to sync from,
// so the source degrades to the template (no fallback claim).
func parentFallback(paths envPaths, f domain.EnvFile, fallbackLabel string) (parent, main []domain.EnvLine, source string, fallback bool, err error) {
	main, err = readEnvFile(filepath.Join(paths.MainPath, f.Target))
	if err != nil {
		return nil, nil, "", false, err
	}
	if main == nil {
		return nil, nil, domain.EnvSourceLabelNone, false, nil
	}
	return nil, main, fallbackLabel, true, nil
}

// parentSourceLabel names the parent worktree the values come from (strict — no main
// mixing when the parent has this file).
func parentSourceLabel(branch string) string {
	if branch == "" {
		return domain.EnvSourceLabelParent
	}
	return branch
}

// worktreeAbsentLabel explains a fallback to main because the parent branch has no
// local worktree at all, e.g. "main (parent 'legacy' has no worktree)".
func worktreeAbsentLabel(branch string) string {
	if branch == "" {
		return domain.EnvSourceLabelMain
	}
	return fmt.Sprintf("main (parent '%s' has no worktree)", branch)
}

// fileAbsentLabel explains a fallback to main because the parent worktree exists but
// does not contain this file, e.g. "main (parent 'feature' has no apps/api/.env)".
func fileAbsentLabel(branch, target string) string {
	if branch == "" {
		return domain.EnvSourceLabelMain
	}
	return fmt.Sprintf("main (parent '%s' has no %s)", branch, target)
}

// flagResolution settles conflicts to the flag decision (empty → keep) and prunes
// orphans only when --prune was passed. Missing keys are never filled without a
// prompt, so they stay reported, not added.
func flagResolution(params SyncEnvParams, diff domain.EnvDiff) EnvResolution {
	decisions := make(map[string]domain.EnvConflictDecision)
	if params.OnConflict == domain.EnvDecisionOverwrite {
		for _, e := range rules.EnvKeysWithStatus(rules.EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
			decisions[e.Key] = domain.EnvDecisionOverwrite
		}
	}
	return EnvResolution{Decisions: decisions, Prune: params.Prune}
}

// applyFile reconciles one file and writes it back when the result differs from the
// current content. Returns whether it wrote.
func applyFile(paths envPaths, c computedFile, res EnvResolution) (bool, error) {
	reconciled := rules.ApplyEnvDiff(rules.ApplyEnvDiffParams{
		Child:        c.child,
		Diff:         c.diff,
		Decisions:    res.Decisions,
		FilledValues: res.FilledValues,
		Prune:        res.Prune,
		PruneKeys:    res.PruneKeys,
		SkipKeys:     res.SkipKeys,
	})

	rendered := rules.RenderEnv(reconciled)
	if rendered == rules.RenderEnv(c.child) {
		return false, nil
	}

	if err := writeEnvFile(filepath.Join(paths.WorktreePath, c.file.Target), rendered); err != nil {
		return false, err
	}
	return true, nil
}

// templateLines reads the committed template (the schema) for a file from the
// target worktree itself — each worktree's branch declares its own expected keys —
// returning nil when no template exists. resolveTemplateSrc is shared with the
// creation-time provisioning in env.go.
func templateLines(worktreePath string, f domain.EnvFile) ([]domain.EnvLine, error) {
	src := resolveTemplateSrc(worktreePath, f)
	if src == "" {
		return nil, nil
	}
	return readEnvFile(src)
}

// readEnvFile parses a .env document, returning nil (not an error) when the file
// is absent — a missing child .env is a valid "everything is drift" starting point.
func readEnvFile(path string) ([]domain.EnvLine, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return rules.ParseEnv(string(data)), nil
}

// writeEnvFile renders and writes reconciled content, creating parent dirs.
func writeEnvFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
