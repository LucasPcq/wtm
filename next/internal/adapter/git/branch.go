package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
