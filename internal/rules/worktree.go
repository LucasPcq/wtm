package rules

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// SanitizeBranchName replaces slashes with dashes for use as a directory name.
func SanitizeBranchName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}

// EncodeBranchSegment percent-encodes a branch name for use as a single
// path segment under <state-dir>/worktrees/. Slashes become %2F so that
// `feat/x` doesn't create a nested directory.
func EncodeBranchSegment(branch string) string {
	return url.PathEscape(branch)
}

// WorktreeMetaDir returns <state-dir>/worktrees/<encoded-branch>/, the
// directory holding meta.json for one worktree.
func WorktreeMetaDir(stateDir, branch string) string {
	return filepath.Join(stateDir, domain.WorktreesSubdir, EncodeBranchSegment(branch))
}

type PurgeableMetaDirParams struct {
	StateDir string
	Branch   string
}

// PurgeableMetaDir returns the same directory as WorktreeMetaDir, but refuses a
// branch that would not survive as a single path segment. It is what a removal
// resolves: WorktreeMetaDir only ever fed writes, while this path feeds a
// recursive delete, where refusing beats guessing.
func PurgeableMetaDir(params PurgeableMetaDirParams) string {
	if params.StateDir == "" || params.Branch == "" {
		return ""
	}
	if !isSinglePathSegment(EncodeBranchSegment(params.Branch)) {
		return ""
	}
	return WorktreeMetaDir(params.StateDir, params.Branch)
}

// WorktreeSlug renders a branch as a name Docker and friends accept for a
// project, a network or a volume: lowercase, [a-z0-9_-] only, starting on an
// alphanumeric. SanitizeBranchName is not a substitute — it keeps the uppercase
// letters that `docker compose` rejects in a project name.
func WorktreeSlug(branch string) string {
	lowered := strings.ToLower(branch)
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, lowered)

	slug := strings.TrimLeft(mapped, "-_")
	if slug == "" {
		return domain.ComposeProjectFallback
	}
	return slug
}

// ResolveEnvStrategy returns override cast to EnvStrategy if non-empty, otherwise strategy.
// The caller is responsible for ensuring override is a valid EnvStrategy value;
// validation occurs at the config boundary before this function is called.
func ResolveEnvStrategy(strategy domain.EnvStrategy, override string) domain.EnvStrategy {
	if override != "" {
		return domain.EnvStrategy(override)
	}
	return strategy
}

// ParentEnvFallbackParams holds inputs for ParentEnvFallsBackToMain.
type ParentEnvFallbackParams struct {
	Strategy          domain.EnvStrategy
	HasCopyFiles      bool
	SourceHasWorktree bool
}

// ParentEnvFallsBackToMain reports whether provisioning .env with the "parent"
// strategy will silently fall back to the main worktree because the source branch
// has no local worktree to copy from. Only meaningful when files are configured to
// be copied.
func ParentEnvFallsBackToMain(params ParentEnvFallbackParams) bool {
	return params.HasCopyFiles &&
		params.Strategy == domain.EnvStrategyParent &&
		!params.SourceHasWorktree
}

// SortStatuses sorts worktree statuses: parent first, then children by creation date (oldest first).
func SortStatuses(statuses []domain.WorktreeStatus) {
	sort.SliceStable(statuses, func(i, j int) bool {
		if statuses[i].IsParent != statuses[j].IsParent {
			return statuses[i].IsParent
		}
		return statuses[i].CreatedAt.Before(statuses[j].CreatedAt)
	})
}

// IsPathWithin reports whether target is dir itself or nested under it.
func IsPathWithin(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// HasWarnings reports whether any condition warrants showing the force option.
func HasWarnings(result domain.CleanCheckResult) bool {
	return len(CleanBlockers(result)) > 0
}

// CleanBlockers names every refusal standing between a worktree and its removal,
// in the order CleanUnsafeReason ranks them. A surface that can only print folds
// them into one recap; one that can ask lifts them one by one.
//
// The parent worktree never gets a blocker: it is not "blocked from
// deletion", it is never deletable at all — a different category the caller
// must not render as a lifted safety refusal.
func CleanBlockers(result domain.CleanCheckResult) []domain.CleanBlocker {
	if result.IsParent {
		return nil
	}
	var blockers []domain.CleanBlocker
	if result.IsDirty {
		blockers = append(blockers, domain.CleanBlocker{
			Key:   domain.CleanBlockerDirty,
			Label: domain.CleanWarnDirty,
		})
	}
	if result.UnpushedCommits > 0 {
		blockers = append(blockers, domain.CleanBlocker{
			Key:   domain.CleanBlockerUnpushed,
			Label: fmt.Sprintf(domain.CleanWarnUnpushedFmt, result.UnpushedCommits),
		})
	}
	if result.HasOpenPR {
		blockers = append(blockers, domain.CleanBlocker{
			Key:   domain.CleanBlockerOpenPR,
			Label: domain.CleanWarnOpenPR + result.PRUrl,
		})
	}
	return blockers
}

// FilterStatusesByMatches returns the subset of statuses whose branch appears in matches.
// Returns statuses unchanged when matches is empty.
func FilterStatusesByMatches(statuses []domain.WorktreeStatus, matches []domain.GitWorktree) []domain.WorktreeStatus {
	if len(matches) == 0 {
		return statuses
	}
	allowed := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		allowed[m.Branch] = struct{}{}
	}
	out := make([]domain.WorktreeStatus, 0, len(matches))
	for _, s := range statuses {
		if _, ok := allowed[s.Branch]; ok {
			out = append(out, s)
		}
	}
	return out
}

// CleanUnsafeReason words the refusal, in the order a user acts on: uncommitted
// work, then unpushed commits, then an open pull request.
func CleanUnsafeReason(check domain.CleanCheckResult) (string, bool) {
	if check.IsDirty {
		return domain.CleanUnsafeDirty, true
	}
	if check.UnpushedCommits > 0 {
		return fmt.Sprintf(domain.CleanUnsafeUnpushedFmt, check.UnpushedCommits), true
	}
	if check.HasOpenPR {
		return domain.CleanUnsafeOpenPR, true
	}
	return "", false
}
