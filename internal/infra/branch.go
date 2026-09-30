// Package infra provides I/O, git command execution, and filesystem wrappers.
package infra

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ListBranchesParams holds inputs for listing local branches.
type ListBranchesParams struct {
	ProjectDir string
}

// ListLocalBranches returns all local branch names sorted alphabetically.
func ListLocalBranches(params ListBranchesParams) ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git branch: %s: %w", strings.TrimSpace(string(out)), err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			branches = append(branches, trimmed)
		}
	}

	return branches, nil
}

// ListRemoteBranches returns the short names of origin's remote-tracking
// branches (e.g. "origin/feature"), sorted alphabetically. The symbolic
// "origin/HEAD" pointer is excluded since it is not a real branch: git shortens
// "refs/remotes/origin/HEAD" to the bare remote name "origin" (never to
// "origin/HEAD"), and no real remote branch shortens to that, so it is dropped.
func ListRemoteBranches(params ListBranchesParams) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %s: %w", strings.TrimSpace(string(out)), err)
	}

	remoteName := strings.TrimSuffix(domain.RemoteBranchPrefix, "/")

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == remoteName {
			continue
		}
		branches = append(branches, trimmed)
	}

	return branches, nil
}

// DeleteLocalBranchParams holds inputs for deleting a local branch.
type DeleteLocalBranchParams struct {
	ProjectDir string
	Branch     string
	Force      bool
}

// DeleteLocalBranch deletes a local git branch.
func DeleteLocalBranch(params DeleteLocalBranchParams) error {
	flag := "-d"
	if params.Force {
		flag = "-D"
	}

	cmd := exec.Command("git", "branch", flag, params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch %s: %s: %w", flag, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func branchExists(projectDir string, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = projectDir
	return cmd.Run() == nil
}

// LocalBranchExistsParams holds inputs for checking local branch existence.
type LocalBranchExistsParams struct {
	ProjectDir string
	Branch     string
}

// LocalBranchExists reports whether a local branch with the given name exists.
func LocalBranchExists(params LocalBranchExistsParams) bool {
	return branchExists(params.ProjectDir, params.Branch)
}

// BranchOrRemoteExistsParams holds inputs for checking a worktree parent ref.
type BranchOrRemoteExistsParams struct {
	ProjectDir string
	Ref        string
}

// BranchOrRemoteExists reports whether ref resolves to a local branch
// (refs/heads/<ref>) or an origin remote-tracking branch (refs/remotes/<ref>,
// e.g. "origin/feature"). Used to validate a worktree parent that may be remote.
func BranchOrRemoteExists(params BranchOrRemoteExistsParams) bool {
	if branchExists(params.ProjectDir, params.Ref) {
		return true
	}
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/remotes/"+params.Ref)
	cmd.Dir = params.ProjectDir
	return cmd.Run() == nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(projectDir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("detached HEAD, no branch checked out")
	}
	return branch, nil
}

// BranchExistsOnRemoteParams holds inputs for checking remote branch existence.
type BranchExistsOnRemoteParams struct {
	ProjectDir string
	Branch     string
}

// FetchBranchParams holds inputs for fetching a specific branch from origin.
type FetchBranchParams struct {
	ProjectDir string
	Branch     string
}

// FetchBranch runs `git fetch origin <branch>` to update the remote-tracking ref.
func FetchBranch(params FetchBranchParams) error {
	cmd := exec.Command("git", "fetch", "origin", params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch origin %s: %s", params.Branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// FastForwardRefParams holds inputs for advancing a branch ref that is not
// checked out anywhere.
type FastForwardRefParams struct {
	ProjectDir string
	Branch     string
}

// FastForwardRef advances a local branch to its origin counterpart by fetching
// straight into the ref. The refspec carries no leading '+', so git itself
// refuses a rewrite: the safety net does not depend on the caller checking first.
// git 2.36+ also refuses a branch checked out in a worktree — advance those with
// FastForwardBranch inside their own worktree. Prefer this to
// UpdateLocalBranchToRemote, which force-moves and can lose commits.
func FastForwardRef(params FastForwardRefParams) error {
	refspec := params.Branch + ":" + params.Branch
	remote := strings.TrimSuffix(domain.RemoteBranchPrefix, "/")
	cmd := exec.Command("git", "fetch", remote, refspec)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch %s %s: %s", remote, refspec, strings.TrimSpace(string(out)))
	}
	return nil
}

// FetchPruneParams holds inputs for a pruning fetch.
type FetchPruneParams struct {
	ProjectDir string
	Remote     string
}

// FetchPrune runs `git fetch --prune <remote>` so branches deleted on the remote
// drop their local remote-tracking refs, making UpstreamGone accurate. Remote
// defaults to origin.
func FetchPrune(params FetchPruneParams) error {
	remote := params.Remote
	if remote == "" {
		remote = strings.TrimSuffix(domain.RemoteBranchPrefix, "/")
	}
	cmd := exec.Command("git", "fetch", "--prune", remote)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch --prune %s: %s", remote, strings.TrimSpace(string(out)))
	}
	return nil
}

// UpstreamGoneParams holds inputs for UpstreamGone.
type UpstreamGoneParams struct {
	ProjectDir string
	Branch     string
}

// UpstreamGone reports whether Branch has a configured upstream whose remote-
// tracking ref no longer exists — git's "[gone]" marker, set once the remote
// branch was deleted and a pruning fetch ran. A branch with no upstream, or one
// whose upstream still exists, returns false.
func UpstreamGone(params UpstreamGoneParams) bool {
	cmd := exec.Command("git", "for-each-ref", "--format=%(upstream:track)", "refs/heads/"+params.Branch)
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "[gone]"
}

// AheadBehindParams holds inputs for computing a local branch's divergence.
type AheadBehindParams struct {
	ProjectDir string
	Local      string
	Remote     string
}

// AheadBehind returns how many commits Local is ahead of and behind Remote,
// computed in a single pass via `git rev-list --left-right --count
// <local>...<remote>`. The left count is ahead (local-only commits), the right
// count is behind (remote-only commits).
func AheadBehind(params AheadBehindParams) (domain.AheadBehind, error) {
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", params.Local+"..."+params.Remote)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return domain.AheadBehind{}, fmt.Errorf("git rev-list: %s: %w", strings.TrimSpace(string(out)), err)
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return domain.AheadBehind{}, fmt.Errorf("git rev-list: unexpected output %q", strings.TrimSpace(string(out)))
	}

	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return domain.AheadBehind{}, fmt.Errorf("git rev-list: parse ahead %q: %w", fields[0], err)
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return domain.AheadBehind{}, fmt.Errorf("git rev-list: parse behind %q: %w", fields[1], err)
	}

	return domain.AheadBehind{Ahead: ahead, Behind: behind}, nil
}

// FetchParams holds inputs for fetching all branches from origin.
type FetchParams struct {
	ProjectDir string
}

// Fetch runs `git fetch origin` to refresh every origin remote-tracking ref.
func Fetch(params FetchParams) error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch origin: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PushBranchParams holds inputs for pushing a branch.
type PushBranchParams struct {
	ProjectDir string
	Branch     string
}
