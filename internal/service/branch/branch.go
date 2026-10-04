// Package branch orchestrates branch-candidate listing for the pickers: it lists
// local and origin branches, computes each local branch's divergence from its
// origin counterpart, and merges them into a single ordered candidate set.
package branch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ListParams holds inputs for listing branch candidates.
type ListParams struct {
	ProjectDir string
}

// Candidates returns the local and origin remote-tracking branches as a single
// ordered candidate set, with each local branch tagged with its ahead/behind
// divergence from its same-name origin counterpart. Errors are swallowed (best
// effort): a repo with no branches yet simply yields an empty list.
func Candidates(params ListParams) []domain.BranchCandidate {
	local, err := infra.ListLocalBranches(infra.ListBranchesParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil
	}
	remote, _ := infra.ListRemoteBranches(infra.ListBranchesParams{ProjectDir: params.ProjectDir})

	return rules.MergeBranchCandidates(rules.MergeBranchCandidatesParams{
		Local:      local,
		Remote:     remote,
		Divergence: divergence(divergenceParams{ProjectDir: params.ProjectDir, Local: local, Remote: remote}),
	})
}

// Refresh fetches origin then recomputes the candidate set so the divergence
// badges reflect the latest remote state. A failed fetch is ignored so the
// picker still refreshes from whatever remote-tracking refs are present.
func Refresh(params ListParams) []domain.BranchCandidate {
	_ = infra.Fetch(infra.FetchParams{ProjectDir: params.ProjectDir})
	return Candidates(params)
}

// BranchParams identifies a single local branch to inspect or update.
type BranchParams struct {
	ProjectDir string
	Branch     string
}

// Divergence classifies how a local branch relates to its origin counterpart,
// using the current remote-tracking refs without fetching. Returns
// DivergenceUnknown when the branch has no origin counterpart.
func Divergence(params BranchParams) (domain.DivergenceState, domain.AheadBehind) {
	ab, err := infra.AheadBehind(infra.AheadBehindParams{
		ProjectDir: params.ProjectDir,
		Local:      params.Branch,
		Remote:     domain.RemoteBranchPrefix + params.Branch,
	})
	if err != nil {
		return domain.DivergenceUnknown, domain.AheadBehind{}
	}
	return rules.ClassifyDivergence(ab.Ahead, ab.Behind), ab
}

// FastForwardIfBehind is the non-interactive counterpart of the create wizard's
// fast-forward prompt: it fast-forwards a behind-only source branch to origin so
// a new worktree starts up to date. A remote start-point, an up-to-date branch,
// or one that cannot be cleanly fast-forwarded (diverged, dirty, or fetch/FF
// failure) is left untouched — creation then proceeds from the local branch
// as-is, mirroring the "keep the local version" choice offered interactively.
func FastForwardIfBehind(params BranchParams) error {
	if rules.IsRemoteBranch(params.Branch) {
		return nil
	}
	if err := FastForwardToOrigin(params); err != nil {
		return err
	}
	return nil
}

// FastForwardToOrigin advances a local branch to its origin counterpart. It
// fetches first, then refuses (without modifying anything) when the branch has
// diverged, when its worktree has uncommitted changes, or when the fast-forward
// otherwise fails. A branch that is already up to date is a no-op.
func FastForwardToOrigin(params BranchParams) error {
	check, err := Check(params)
	if err != nil {
		return err
	}
	if check.State == domain.DivergenceDiverged {
		return fmt.Errorf("%s has diverged from origin (%d ahead, %d behind)", params.Branch, check.Ahead, check.Behind)
	}
	result := FastForward(FastForwardParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Check:      &check,
	})
	if result.Status == domain.FFFailed {
		return errors.New(result.Detail)
	}
	return nil
}

// FetchFromOrigin updates the branch's remote-tracking ref, so what is checked out
// next is what origin holds now.
func FetchFromOrigin(params BranchParams) error {
	return infra.FetchBranch(infra.FetchBranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
}

// Check gathers a branch's state against origin in one network round trip, so a
// recap and the run that follows it read the same facts and the branch is
// fetched once rather than twice.
func Check(params BranchParams) (domain.FastForwardCheck, error) {
	check := domain.FastForwardCheck{Branch: params.Branch, State: domain.DivergenceUnknown}

	// A branch origin does not carry fails here; the ahead/behind below settles it.
	_ = infra.FetchBranch(infra.FetchBranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch})

	ab, err := infra.AheadBehind(infra.AheadBehindParams{
		ProjectDir: params.ProjectDir,
		Local:      params.Branch,
		Remote:     domain.RemoteBranchPrefix + params.Branch,
	})
	if err != nil {
		return check, nil
	}
	check.HasUpstream = true
	check.Ahead, check.Behind = ab.Ahead, ab.Behind
	check.State = rules.ClassifyDivergence(ab.Ahead, ab.Behind)

	wt, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		if errors.Is(err, domain.ErrWorktreeNotFound) {
			return check, nil
		}
		return check, err
	}
	check.WorktreePath = wt.Path

	dirty, err := infra.IsDirty(infra.IsDirtyParams{WorktreePath: wt.Path})
	if err != nil {
		return check, err
	}
	check.IsDirty = dirty
	return check, nil
}

// FastForwardParams holds inputs for FastForward.
type FastForwardParams struct {
	ProjectDir string
	Branch     string
	// Force lifts the dirty refusal, and only that one: a diverged branch is
	// refused whatever it is set to.
	Force bool
	// Check is a state Check already gathered, so an interactive run that built a
	// recap does not fetch the same branch a second time.
	Check *domain.FastForwardCheck
}

// FastForward advances a branch to its origin counterpart, reporting what became
// of it rather than returning an error: a run over several branches keeps going
// past the one it could not move.
func FastForward(params FastForwardParams) domain.FastForwardResult {
	check, err := resolveCheck(params)
	if err != nil {
		return failed(params.Branch, err)
	}

	result := domain.FastForwardResult{Branch: params.Branch, Behind: check.Behind}
	result.OldTip, _ = infra.Tip(infra.TipParams{WorktreePath: params.ProjectDir, Ref: params.Branch})
	result.NewTip = result.OldTip

	if !check.HasUpstream {
		return labelled(result, domain.FFNoUpstream)
	}
	if check.State == domain.DivergenceDiverged {
		return labelled(result, domain.FFDiverged)
	}
	if check.Behind == 0 {
		return labelled(result, domain.FFUpToDate)
	}
	if check.IsDirty && !params.Force {
		result.Detail = domain.FastForwardWarnDirty
		return labelled(result, domain.FFFailed)
	}

	if ffErr := advance(check, params.ProjectDir); ffErr != nil {
		result.Detail = ffErr.Error()
		return labelled(result, domain.FFFailed)
	}
	if tip, tipErr := infra.Tip(infra.TipParams{WorktreePath: params.ProjectDir, Ref: params.Branch}); tipErr == nil {
		result.NewTip = tip
	}
	return labelled(result, domain.FFAdvanced)
}

func resolveCheck(params FastForwardParams) (domain.FastForwardCheck, error) {
	if params.Check != nil {
		return *params.Check, nil
	}
	return Check(BranchParams{ProjectDir: params.ProjectDir, Branch: params.Branch})
}

// advance moves the ref where it must be moved: a branch checked out nowhere is
// advanced by fetching straight into it, one that is checked out must go through
// its own worktree.
func advance(check domain.FastForwardCheck, projectDir string) error {
	if check.WorktreePath == "" {
		return infra.FastForwardRef(infra.FastForwardRefParams{
			ProjectDir: projectDir,
			Branch:     check.Branch,
		})
	}
	return infra.FastForwardBranch(infra.FastForwardParams{
		WorktreePath: check.WorktreePath,
		Onto:         domain.RemoteBranchPrefix + check.Branch,
	})
}

func labelled(result domain.FastForwardResult, status domain.FastForwardStatus) domain.FastForwardResult {
	result.Status = status
	result.Label = rules.FastForwardStatusLabel(status)
	return result
}

func failed(branchName string, err error) domain.FastForwardResult {
	return labelled(domain.FastForwardResult{Branch: branchName, Detail: err.Error()}, domain.FFFailed)
}

// divergenceParams holds inputs for divergence.
type divergenceParams struct {
	ProjectDir string
	Local      []string
	Remote     []string
}

// divergence computes ahead/behind only for local branches that have a same-name
// origin counterpart, bounding the number of git calls to the overlap.
func divergence(params divergenceParams) map[string]domain.AheadBehind {
	remoteSet := make(map[string]struct{}, len(params.Remote))
	for _, r := range params.Remote {
		remoteSet[strings.TrimPrefix(r, domain.RemoteBranchPrefix)] = struct{}{}
	}

	result := make(map[string]domain.AheadBehind, len(params.Local))
	for _, b := range params.Local {
		if _, ok := remoteSet[b]; !ok {
			continue
		}
		ab, err := infra.AheadBehind(infra.AheadBehindParams{
			ProjectDir: params.ProjectDir,
			Local:      b,
			Remote:     domain.RemoteBranchPrefix + b,
		})
		if err != nil {
			continue
		}
		result[b] = ab
	}
	return result
}
