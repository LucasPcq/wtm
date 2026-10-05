package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ListChangesParams struct {
	WorktreePath string
}

// ListChanges returns the uncommitted files of a worktree classified for
// extraction.
func ListChanges(ctx context.Context, params ListChangesParams) ([]domain.ExtractFile, error) {
	modified, err := infra.ListModifiedFiles(ctx, infra.ListModifiedFilesParams{WorktreePath: params.WorktreePath})
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

// Extract moves the selected uncommitted files from the source worktree to the
// target worktree. It is transactional: the changes are applied to the target
// first and the source is cleaned only on success. Any conflict aborts the
// operation with the source left untouched. With Keep set, the source is not
// cleaned (copy instead of move).
func Extract(ctx context.Context, params domain.ExtractParams) (domain.ExtractResult, error) {
	if params.SourcePath == params.TargetPath {
		return domain.ExtractResult{}, domain.ErrSameWorktree
	}
	if len(params.Files) == 0 {
		return domain.ExtractResult{}, domain.ErrNoFilesSelected
	}

	tracked, untracked := splitByStatus(params.Files)

	trackedConflicts := conflictingFiles(ctx, conflictScanParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Tracked:    tracked,
	})
	collisions := collidingUntracked(untrackedCollisionParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Untracked:  untracked,
	})

	// Binary collisions cannot carry conflict markers, so resolve mode has nothing
	// to offer them. They are reported first, whatever the mode, so the message
	// never points at a --on-conflict resolve that would not help.
	if len(collisions.Unmergeable) > 0 {
		return domain.ExtractResult{}, binaryCollisionError(conflictErrorParams{
			Untracked:    collisions.Unmergeable,
			TargetBranch: params.TargetBranch,
		})
	}

	if len(trackedConflicts)+len(collisions.Conflicting) > 0 &&
		params.ConflictMode != domain.OnConflictResolve {
		return domain.ExtractResult{}, conflictError(conflictErrorParams{
			Tracked:      trackedConflicts,
			Untracked:    collisions.Conflicting,
			TargetBranch: params.TargetBranch,
		})
	}

	plan := extractPlan{
		Params:             params,
		Tracked:            tracked,
		Untracked:          untracked,
		Conflicts:          trackedConflicts,
		UntrackedConflicts: collisions.Conflicting,
	}
	if len(trackedConflicts)+len(collisions.Conflicting) > 0 {
		return resolveExtract(ctx, plan)
	}

	return cleanExtract(ctx, plan)
}

// extractPlan is the resolved extraction work: the request plus the selected
// files split into tracked/untracked and the subsets that conflict.
type extractPlan struct {
	Params    domain.ExtractParams
	Tracked   []string
	Untracked []string
	// Conflicts holds the tracked files whose patch does not apply onto the target.
	Conflicts []string
	// UntrackedConflicts holds the untracked files that already exist in the target
	// with different content.
	UntrackedConflicts []string
}

// cleanExtract handles the no-conflict path: apply the whole selection to the
// target and, unless Keep is set, remove it from the source (full move).
func cleanExtract(ctx context.Context, plan extractPlan) (domain.ExtractResult, error) {
	params := plan.Params
	patch, err := infra.DiffFiles(ctx, infra.DiffFilesParams{
		WorktreePath: params.SourcePath,
		Files:        plan.Tracked,
	})
	if err != nil {
		return domain.ExtractResult{}, err
	}

	if err := infra.ApplyPatch(ctx, infra.ApplyPatchParams{
		WorktreePath: params.TargetPath,
		Patch:        patch,
		ThreeWay:     true,
	}); err != nil {
		rollbackTarget(ctx, rollbackTargetParams{TargetPath: params.TargetPath, Patch: patch, Tracked: plan.Tracked})
		return domain.ExtractResult{}, conflictError(conflictErrorParams{Tracked: plan.Tracked, TargetBranch: params.TargetBranch})
	}

	if err := copyUntracked(copyUntrackedParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Files:      plan.Untracked,
	}); err != nil {
		rollbackTarget(ctx, rollbackTargetParams{TargetPath: params.TargetPath, Patch: patch, Tracked: plan.Tracked})
		removeTargetFiles(removeFilesParams{Dir: params.TargetPath, Files: plan.Untracked})
		return domain.ExtractResult{}, err
	}

	if !params.Keep {
		if err := cleanSource(ctx, cleanSourceParams{
			SourcePath: params.SourcePath,
			Patch:      patch,
			Tracked:    plan.Tracked,
			Untracked:  plan.Untracked,
		}); err != nil {
			return domain.ExtractResult{}, fmt.Errorf("clean source: %w", err)
		}
	}

	return domain.ExtractResult{
		Files:        params.Files,
		TargetPath:   params.TargetPath,
		TargetBranch: params.TargetBranch,
		SourceBranch: params.SourceBranch,
		Kept:         params.Keep,
	}, nil
}

// resolveExtract handles the conflict path in resolve mode: clean files are
// applied normally, conflicting files are written with merge markers, untracked
// files are copied, and the source is left fully intact (recoverable).
func resolveExtract(ctx context.Context, plan extractPlan) (domain.ExtractResult, error) {
	params := plan.Params
	clean := subtract(plan.Tracked, plan.Conflicts)
	// An untracked file that already exists in the target is merged like a tracked
	// one: MergeFile builds its base from `git show HEAD:<path>`, which is empty
	// for a file absent from HEAD, so it degenerates into a two-way merge.
	merged := append(append([]string{}, plan.Conflicts...), plan.UntrackedConflicts...)

	cleanPatch, err := infra.DiffFiles(ctx, infra.DiffFilesParams{
		WorktreePath: params.SourcePath,
		Files:        clean,
	})
	if err != nil {
		return domain.ExtractResult{}, err
	}
	if err := infra.ApplyPatch(ctx, infra.ApplyPatchParams{
		WorktreePath: params.TargetPath,
		Patch:        cleanPatch,
		ThreeWay:     true,
	}); err != nil {
		rollbackTarget(ctx, rollbackTargetParams{TargetPath: params.TargetPath, Patch: cleanPatch, Tracked: clean})
		return domain.ExtractResult{}, conflictError(conflictErrorParams{Tracked: clean, TargetBranch: params.TargetBranch})
	}

	for _, f := range merged {
		if _, err := infra.MergeFile(ctx, infra.MergeFileParams{
			SourceWorktree: params.SourcePath,
			TargetWorktree: params.TargetPath,
			RelPath:        f,
			SourceBranch:   params.SourceBranch,
			TargetBranch:   params.TargetBranch,
		}); err != nil {
			return domain.ExtractResult{}, fmt.Errorf("merge %s: %w", f, err)
		}
	}

	if err := copyUntracked(copyUntrackedParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Files:      subtract(plan.Untracked, plan.UntrackedConflicts),
	}); err != nil {
		return domain.ExtractResult{}, err
	}

	return domain.ExtractResult{
		Files:        params.Files,
		TargetPath:   params.TargetPath,
		TargetBranch: params.TargetBranch,
		SourceBranch: params.SourceBranch,
		Kept:         true,
		Conflicts:    merged,
	}, nil
}

// ConflictingFiles returns the selected files that clash with the target worktree
// and could be resolved with conflict markers: tracked files whose patch does not
// apply, and untracked files that already exist there with different content.
// Binary collisions are left out — they abort whatever the user answers, so
// offering to resolve them would be a dead end.
func ConflictingFiles(ctx context.Context, params domain.ConflictCheckParams) []string {
	tracked, untracked := splitByStatus(params.Files)
	conflicts := conflictingFiles(ctx, conflictScanParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Tracked:    tracked,
	})
	collisions := collidingUntracked(untrackedCollisionParams{
		SourcePath: params.SourcePath,
		TargetPath: params.TargetPath,
		Untracked:  untracked,
	})
	return append(conflicts, collisions.Conflicting...)
}

func subtract(all, remove []string) []string {
	set := make(map[string]struct{}, len(remove))
	for _, r := range remove {
		set[r] = struct{}{}
	}
	out := make([]string, 0, len(all))
	for _, a := range all {
		if _, ok := set[a]; !ok {
			out = append(out, a)
		}
	}
	return out
}

// splitByStatus separates the selection into paths carried by the patch and paths
// copied verbatim. A rename contributes both of its paths to the patch: diffing
// only the new path would produce the addition without the matching deletion.
func splitByStatus(files []domain.ExtractFile) (tracked, untracked []string) {
	for _, f := range files {
		if f.Status == domain.ExtractStatusUntracked {
			untracked = append(untracked, f.Path)
			continue
		}
		tracked = append(tracked, f.Path)
		if f.OrigPath != "" {
			tracked = append(tracked, f.OrigPath)
		}
	}
	return tracked, untracked
}

type untrackedCollisionParams struct {
	SourcePath string
	TargetPath string
	Untracked  []string
}

// untrackedCollisions splits the untracked files that already exist in the target
// by what can be done about them.
type untrackedCollisions struct {
	// Conflicting can be merged with conflict markers in resolve mode.
	Conflicting []string
	// Unmergeable is binary on one side, where markers would corrupt the file.
	Unmergeable []string
}

// collidingUntracked reports the untracked files that already exist in the target
// with different content. Identical content is not a collision: copying is then a
// no-op, and naming such a file as a conflict would promise markers that never
// get written.
func collidingUntracked(params untrackedCollisionParams) untrackedCollisions {
	var collisions untrackedCollisions
	for _, f := range params.Untracked {
		targetPath := filepath.Join(params.TargetPath, f)
		if !infra.FileExists(targetPath) {
			continue
		}
		sourcePath := filepath.Join(params.SourcePath, f)
		if infra.SameContent(infra.SameContentParams{A: sourcePath, B: targetPath}) {
			continue
		}
		if infra.IsBinary(infra.IsBinaryParams{Path: sourcePath}) ||
			infra.IsBinary(infra.IsBinaryParams{Path: targetPath}) {
			collisions.Unmergeable = append(collisions.Unmergeable, f)
			continue
		}
		collisions.Conflicting = append(collisions.Conflicting, f)
	}
	return collisions
}

// conflictScanParams holds inputs for the per-file conflict scan.
type conflictScanParams struct {
	SourcePath string
	TargetPath string
	Tracked    []string
}

// conflictingFiles returns the tracked files whose changes do not apply cleanly
// onto the target, checked one by one so the message can name them.
func conflictingFiles(ctx context.Context, params conflictScanParams) []string {
	var conflicts []string
	for _, f := range params.Tracked {
		patch, err := infra.DiffFiles(ctx, infra.DiffFilesParams{WorktreePath: params.SourcePath, Files: []string{f}})
		if err != nil {
			conflicts = append(conflicts, f)
			continue
		}
		if err := infra.ApplyPatch(ctx, infra.ApplyPatchParams{
			WorktreePath: params.TargetPath,
			Patch:        patch,
			ThreeWay:     true,
			Check:        true,
		}); err != nil {
			conflicts = append(conflicts, f)
		}
	}
	return conflicts
}

type conflictErrorParams struct {
	// Tracked holds files whose patch does not apply onto the target.
	Tracked []string
	// Untracked holds files that already exist in the target with other content.
	Untracked    []string
	TargetBranch string
}

// conflictError builds a readable, actionable conflict message naming the files
// that clash with the target worktree. The two kinds of clash are worded apart:
// a tracked file was modified on both sides, an untracked file simply exists
// already.
func conflictError(params conflictErrorParams) error {
	var clauses []string
	if len(params.Tracked) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s %s", strings.Join(params.Tracked, ", "),
			pick(len(params.Tracked), "was also modified", "were also modified")))
	}
	if len(params.Untracked) > 0 {
		clauses = append(clauses, fmt.Sprintf("%s %s", strings.Join(params.Untracked, ", "),
			pick(len(params.Untracked), "already exists", "already exist")))
	}

	total := len(params.Tracked) + len(params.Untracked)
	return fmt.Errorf("%w: %s in %q — resolve %s there, extract to another worktree, or re-run with --%s %s",
		domain.ErrExtractConflict, strings.Join(clauses, " and "), params.TargetBranch,
		them(total), domain.FlagOnConflict, domain.OnConflictResolve)
}

// binaryCollisionError reports untracked binary files that already exist in the
// target. Conflict markers would corrupt them, so resolve mode cannot help.
func binaryCollisionError(params conflictErrorParams) error {
	return fmt.Errorf("%w: %s %s in %q and %s binary — conflict markers cannot be written, remove %s there or extract to another worktree",
		domain.ErrExtractConflict, strings.Join(params.Untracked, ", "),
		pick(len(params.Untracked), "already exists", "already exist"), params.TargetBranch,
		pick(len(params.Untracked), "is", "are"), them(len(params.Untracked)))
}

// pick returns the singular or plural wording matching a count.
func pick(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// them returns the object pronoun matching the file count.
func them(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

type copyUntrackedParams struct {
	SourcePath string
	TargetPath string
	Files      []string
}

func copyUntracked(params copyUntrackedParams) error {
	for _, f := range params.Files {
		if err := infra.CopyPath(infra.CopyPathParams{
			SourceDir: params.SourcePath,
			TargetDir: params.TargetPath,
			RelPath:   f,
		}); err != nil {
			return fmt.Errorf("copy %s: %w", f, err)
		}
	}
	return nil
}

type cleanSourceParams struct {
	SourcePath string
	Patch      []byte
	Tracked    []string
	Untracked  []string
}

// cleanSource reverts the extracted changes in the source worktree by
// reverse-applying the same patch and unstaging the affected paths, then deletes
// the extracted untracked files.
func cleanSource(ctx context.Context, params cleanSourceParams) error {
	if err := infra.ApplyPatch(ctx, infra.ApplyPatchParams{
		WorktreePath: params.SourcePath,
		Patch:        params.Patch,
		Reverse:      true,
	}); err != nil {
		return err
	}
	if err := infra.ResetPaths(ctx, infra.ResetPathsParams{
		WorktreePath: params.SourcePath,
		Files:        params.Tracked,
	}); err != nil {
		return err
	}
	removeTargetFiles(removeFilesParams{Dir: params.SourcePath, Files: params.Untracked})
	pruneEmptyDirs(pruneEmptyDirsParams{Root: params.SourcePath, Files: params.Untracked})
	return nil
}

type pruneEmptyDirsParams struct {
	Root  string
	Files []string
}

// pruneEmptyDirs removes the directories left behind once extracted untracked
// files are gone. Untracked entries are listed per file, so moving out every file
// of a brand-new directory would otherwise strand an empty tree in the source.
func pruneEmptyDirs(params pruneEmptyDirsParams) {
	for _, f := range params.Files {
		dir := filepath.Dir(filepath.Join(params.Root, f))
		for dir != params.Root && strings.HasPrefix(dir, params.Root+string(filepath.Separator)) {
			if err := os.Remove(dir); err != nil {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}

type rollbackTargetParams struct {
	TargetPath string
	Patch      []byte
	Tracked    []string
}

// rollbackTarget best-effort undoes a partial application on the target so a
// failed extraction leaves it as it was.
func rollbackTarget(ctx context.Context, params rollbackTargetParams) {
	_ = infra.ApplyPatch(ctx, infra.ApplyPatchParams{
		WorktreePath: params.TargetPath,
		Patch:        params.Patch,
		ThreeWay:     true,
		Reverse:      true,
	})
	_ = infra.ResetPaths(ctx, infra.ResetPathsParams{WorktreePath: params.TargetPath, Files: params.Tracked})
}

type removeFilesParams struct {
	Dir   string
	Files []string
}

func removeTargetFiles(params removeFilesParams) {
	for _, f := range params.Files {
		_ = os.RemoveAll(filepath.Join(params.Dir, f))
	}
}
