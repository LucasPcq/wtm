package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Reader is what a command observes of git.
type Reader interface {
	Worktrees(ctx context.Context) ([]Worktree, error)
	MainWorktree(ctx context.Context) (Worktree, error)
	Branches(ctx context.Context) (Branches, error)
	RefExists(ctx context.Context, ref Ref) (bool, error)
	AheadBehind(ctx context.Context, params AheadBehindParams) (AheadBehind, error)
}

// Branches are named without their prefix: Remote holds `feature`, not
// `origin/feature`.
type Branches struct {
	Local  []string
	Remote []string
}

type AheadBehindParams struct {
	Of      Ref
	Against Ref
}

// AheadBehind counts the commits Of has that Against lacks, and the reverse.
type AheadBehind struct {
	Ahead  int
	Behind int
}

const remoteHead = "HEAD"

func (r Repo) Branches(ctx context.Context) (Branches, error) {
	remotePrefix := r.remotePrefix()
	out, err := r.git(ctx, "for-each-ref", "--format=%(refname)", strings.TrimSuffix(headsPrefix, "/"), strings.TrimSuffix(remotePrefix, "/"))
	if err != nil {
		return Branches{}, err
	}
	var branches Branches
	for line := range strings.Lines(string(out)) {
		refname := strings.TrimSpace(line)
		if local, ok := strings.CutPrefix(refname, headsPrefix); ok {
			branches.Local = append(branches.Local, local)
			continue
		}
		if remote, ok := strings.CutPrefix(refname, remotePrefix); ok && remote != remoteHead {
			branches.Remote = append(branches.Remote, remote)
		}
	}
	return branches, nil
}

// RefExists reads git's exit status 1 as an absent ref; any other failure is
// an error, never an absence.
func (r Repo) RefExists(ctx context.Context, ref Ref) (bool, error) {
	name, err := r.refName(ref)
	if err != nil {
		return false, err
	}
	_, err = r.git(ctx, "show-ref", "--verify", "--quiet", name)
	var gitErr *Error
	if errors.As(err, &gitErr) && gitErr.Exit == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r Repo) AheadBehind(ctx context.Context, params AheadBehindParams) (AheadBehind, error) {
	of, err := r.refName(params.Of)
	if err != nil {
		return AheadBehind{}, err
	}
	against, err := r.refName(params.Against)
	if err != nil {
		return AheadBehind{}, err
	}
	out, err := r.git(ctx, "rev-list", "--left-right", "--count", of+"..."+against)
	if err != nil {
		return AheadBehind{}, err
	}
	return parseAheadBehind(string(out))
}

func parseAheadBehind(out string) (AheadBehind, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return AheadBehind{}, fmt.Errorf("git rev-list --left-right --count: unexpected output %q", out)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return AheadBehind{}, err
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return AheadBehind{}, err
	}
	return AheadBehind{Ahead: ahead, Behind: behind}, nil
}
