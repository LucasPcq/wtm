package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
)

const (
	headsPrefix   = "refs/heads/"
	remotesPrefix = "refs/remotes/"
	remoteHead    = "HEAD"
)

var errNoRef = errors.New("no ref given")

func (r Repo) refName(ref branch.Ref) (string, error) {
	switch b := ref.(type) {
	case branch.LocalBranch:
		return headsPrefix + b.Name, nil
	case branch.RemoteBranch:
		return r.remotePrefix() + b.Name, nil
	}
	return "", errNoRef
}

func (r Repo) remotePrefix() string { return remotesPrefix + r.Remote + "/" }

func (r Repo) Branches(ctx context.Context) (branch.Branches, error) {
	remotePrefix := r.remotePrefix()
	out, err := r.git(ctx, "for-each-ref", "--format=%(objectname) %(refname)", strings.TrimSuffix(headsPrefix, "/"), strings.TrimSuffix(remotePrefix, "/"))
	if err != nil {
		return branch.Branches{}, err
	}
	var branches branch.Branches
	for line := range strings.Lines(string(out)) {
		tip, refname, _ := strings.Cut(strings.TrimSpace(line), " ")
		if local, ok := strings.CutPrefix(refname, headsPrefix); ok {
			branches.Local = append(branches.Local, branch.Branch{Name: local, Tip: tip})
			continue
		}
		if remote, ok := strings.CutPrefix(refname, remotePrefix); ok && remote != remoteHead {
			branches.Remote = append(branches.Remote, branch.Branch{Name: remote, Tip: tip})
		}
	}
	return branches, nil
}

func (r Repo) AheadBehind(ctx context.Context, params branch.AheadBehindParams) (branch.AheadBehind, error) {
	of, err := r.refName(params.Of)
	if err != nil {
		return branch.AheadBehind{}, err
	}
	against, err := r.refName(params.Against)
	if err != nil {
		return branch.AheadBehind{}, err
	}
	out, err := r.git(ctx, "rev-list", "--left-right", "--count", of+"..."+against)
	if err != nil {
		return branch.AheadBehind{}, err
	}
	return parseAheadBehind(string(out))
}

func parseAheadBehind(out string) (branch.AheadBehind, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return branch.AheadBehind{}, fmt.Errorf("git rev-list --left-right --count: unexpected output %q", out)
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil {
		return branch.AheadBehind{}, err
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil {
		return branch.AheadBehind{}, err
	}
	return branch.AheadBehind{Ahead: ahead, Behind: behind}, nil
}

func (r Repo) DeleteBranch(ctx context.Context, name string) error {
	_, err := r.git(ctx, "branch", "-D", "--", name)
	return err
}

func (r Repo) refExists(ctx context.Context, ref branch.Ref) (bool, error) {
	name, err := r.refName(ref)
	if err != nil {
		return false, err
	}
	_, err = r.git(ctx, "show-ref", "--verify", "--quiet", name)
	var gitErr *Error
	if errors.As(err, &gitErr) && gitErr.Exit == 1 {
		return false, nil
	}
	return err == nil, err
}

func (r Repo) tip(ctx context.Context, ref branch.Ref) (string, error) {
	name, err := r.refName(ref)
	if err != nil {
		return "", err
	}
	out, err := r.git(ctx, "rev-parse", "--verify", name)
	return strings.TrimSpace(string(out)), err
}
