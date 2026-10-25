package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	headsPrefix  = "refs/heads/"
	remotePrefix = "refs/remotes/" + origin + "/" // the ref prefix of origin's branches
	remoteHead   = "HEAD"
)

var errNoRef = errors.New("no ref given")

// refName returns the full ref name of ref, or errNoRef for a nil ref.
func (r Repo) refName(ref worktree.Ref) (string, error) {
	switch b := ref.(type) {
	case worktree.LocalBranch:
		return headsPrefix + b.Name, nil
	case worktree.RemoteBranch:
		return remotePrefix + b.Name, nil
	}
	return "", errNoRef
}

// Branches returns the local branches and the remote's branches with their
// tips, the remote's HEAD left out.
func (r Repo) Branches(ctx context.Context) (worktree.Branches, error) {
	out, err := r.git(ctx, "for-each-ref", "--format=%(objectname) %(refname)", strings.TrimSuffix(headsPrefix, "/"), strings.TrimSuffix(remotePrefix, "/"))
	if err != nil {
		return worktree.Branches{}, err
	}

	var branches worktree.Branches
	for line := range strings.Lines(string(out)) {
		tip, refname, _ := strings.Cut(strings.TrimSpace(line), " ")
		if local, ok := strings.CutPrefix(refname, headsPrefix); ok {
			branches.Local = append(branches.Local, worktree.Branch{Name: local, Tip: tip})
			continue
		}
		if remote, ok := strings.CutPrefix(refname, remotePrefix); ok && remote != remoteHead {
			branches.Remote = append(branches.Remote, worktree.Branch{Name: remote, Tip: tip})
		}
	}

	return branches, nil
}

// CommitDates returns, by local branch, when the commit at its tip was made, in UTC as every date wtm writes.
func (r Repo) CommitDates(ctx context.Context) (map[string]time.Time, error) {
	out, err := r.git(ctx, "for-each-ref", "--format=%(committerdate:iso-strict) %(refname)", strings.TrimSuffix(headsPrefix, "/"))
	if err != nil {
		return nil, err
	}

	dates := map[string]time.Time{}
	for line := range strings.Lines(string(out)) {
		date, refname, _ := strings.Cut(strings.TrimSpace(line), " ")
		at, err := time.Parse(time.RFC3339, date)
		if err != nil {
			return nil, fmt.Errorf("read the date of %s: %w", refname, err)
		}
		dates[strings.TrimPrefix(refname, headsPrefix)] = at.UTC()
	}

	return dates, nil
}

// AheadBehind counts the commits the local branch name has that its remote
// branch lacks, and the reverse. It fails when either branch is missing.
func (r Repo) AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error) {
	of, err := r.refName(worktree.LocalBranch{Name: name})
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	against, err := r.refName(worktree.RemoteBranch{Name: name})
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	out, err := r.git(ctx, "rev-list", "--left-right", "--count", of+"..."+against)
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	return parseAheadBehind(string(out))
}

// AheadBehindOf counts, in one git process, the commits span's branch has that
// its base lacks, and the reverse, both local branches. It fails when either
// branch is missing.
func (r Repo) AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error) {
	out, err := r.git(ctx, "rev-list", "--left-right", "--count", headsPrefix+span.Branch+"..."+headsPrefix+span.Base, "--")
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	return parseAheadBehind(string(out))
}

// IsAncestor reports whether the commit ancestry.Ancestor is ancestry.Descendant or one of its ancestors; it fails when either is no commit of the repository, one never fetched included.
func (r Repo) IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error) {
	_, err := r.git(ctx, "merge-base", "--is-ancestor", "--end-of-options", ancestry.Ancestor, ancestry.Descendant)
	// git exits 1 for a commit that is not an ancestor, 128 for one it does not know.
	if failed, ok := errors.AsType[*Error](err); ok && failed.Exit == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// parseAheadBehind reads the two counts of git rev-list --left-right --count.
func parseAheadBehind(out string) (worktree.AheadBehind, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return worktree.AheadBehind{}, fmt.Errorf("git rev-list --left-right --count: unexpected output %q", out)
	}

	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return worktree.AheadBehind{}, err
	}

	return worktree.AheadBehind{Ahead: ahead, Behind: behind}, nil
}

// Unpushed counts the commits of the local branch name that no other local
// branch but those going with it, nor any remote-tracking branch, holds: what
// deleting them would lose. It fails when the branch is missing.
func (r Repo) Unpushed(ctx context.Context, name string, going []string) (int, error) {
	of, err := r.refName(worktree.LocalBranch{Name: name})
	if err != nil {
		return 0, err
	}

	args := []string{"rev-list", "--count", of, "--not", "--exclude=" + name}
	for _, other := range going {
		args = append(args, "--exclude="+other)
	}
	out, err := r.git(ctx, append(args, "--branches", "--remotes", "--")...)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// DeleteBranch deletes the local branch name, merged or not.
func (r Repo) DeleteBranch(ctx context.Context, name string) error {
	_, err := r.git(ctx, "branch", "-D", "--", name)
	return err
}

// refExists reports whether ref exists; git's exit 1 means it does not, any
// other failure is an error.
func (r Repo) refExists(ctx context.Context, ref worktree.Ref) (bool, error) {
	name, err := r.refName(ref)
	if err != nil {
		return false, err
	}

	_, err = r.git(ctx, "show-ref", "--verify", "--quiet", name)
	if gitErr, ok := errors.AsType[*Error](err); ok && gitErr.Exit == 1 {
		return false, nil
	}
	return err == nil, err
}
