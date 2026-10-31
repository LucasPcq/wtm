package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Fetch fetches every branch of the remote, updating its remote-tracking
// branches only.
func (r Repo) Fetch(ctx context.Context) error {
	_, err := r.git(ctx, "fetch", "--quiet", origin)
	return err
}

// RefreshOrigin brings the remote-tracking branch of each of branches up to
// date with origin: fetched when origin has it, deleted when origin no longer
// does, so a branch whose upstream was deleted reads as gone. A repository with
// no origin has nothing to refresh.
func (r Repo) RefreshOrigin(ctx context.Context, branches []string) error {
	remotes, err := r.git(ctx, "remote")
	if err != nil || len(branches) == 0 || !slices.Contains(strings.Fields(string(remotes)), origin) {
		return err
	}

	heads := make([]string, 0, len(branches))
	for _, name := range branches {
		heads = append(heads, headsPrefix+name)
	}
	listed, err := r.git(ctx, append([]string{"ls-remote", "--heads", origin}, heads...)...)
	if err != nil {
		return err
	}

	on := map[string]bool{}
	for line := range strings.Lines(string(listed)) {
		_, refname, _ := strings.Cut(strings.TrimSpace(line), "\t")
		on[refname] = true
	}
	var refspecs []string
	for _, head := range heads {
		tracking := remotePrefix + strings.TrimPrefix(head, headsPrefix)
		if on[head] {
			refspecs = append(refspecs, "+"+head+":"+tracking)
			continue
		}
		if _, err := r.git(ctx, "update-ref", "-d", tracking); err != nil {
			return err
		}
	}
	if len(refspecs) == 0 {
		return nil
	}

	_, err = r.git(ctx, append([]string{"fetch", "--quiet", origin, "--"}, refspecs...)...)
	return err
}

// Gone returns the local branches whose upstream is gone on origin: they
// track a branch of origin that is no longer fetched, deleted there since. A
// branch tracking a local branch that was deleted is not gone on origin: git
// reads it [gone] too, its remote ".".
func (r Repo) Gone(ctx context.Context) (map[string]bool, error) {
	out, err := r.git(ctx, "for-each-ref", "--format=%(upstream:remotename)%00%(upstream:track)%00%(refname)", strings.TrimSuffix(headsPrefix, "/"))
	if err != nil {
		return nil, err
	}

	gone := map[string]bool{}
	for line := range strings.Lines(string(out)) {
		fields := strings.Split(strings.TrimSpace(line), "\x00")
		if len(fields) == 3 && fields[0] == origin && fields[1] == "[gone]" {
			gone[strings.TrimPrefix(fields[2], headsPrefix)] = true
		}
	}
	return gone, nil
}

// FetchRef fetches the branch name from the remote, updating its remote-tracking
// branch only. Branches are named in full so a tag of the same name, or a
// branch named tags/..., is never fetched instead. A branch the remote no longer
// has wraps worktree.ErrNoRemoteBranch.
func (r Repo) FetchRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", origin, "--", headsPrefix+name)
	if failed, ok := errors.AsType[*Error](err); ok && strings.Contains(failed.Stderr, "couldn't find remote ref") {
		return fmt.Errorf("%w: %s: %w", worktree.ErrNoRemoteBranch, name, err)
	}

	return err
}

// FastForwardRef fetches the branch name from the remote straight into the
// local branch of that name; git refuses unless it is a fast-forward and the
// branch is checked out nowhere.
func (r Repo) FastForwardRef(ctx context.Context, name string) error {
	_, err := r.git(ctx, "fetch", origin, "--", headsPrefix+name+":"+headsPrefix+name)
	return err
}

// MergeFFOnly fast-forwards the branch checked out at merge.Path onto
// merge.Onto, failing rather than merging when it cannot.
func (r Repo) MergeFFOnly(ctx context.Context, merge worktree.Merge) error {
	onto, err := r.refName(merge.Onto)
	if err != nil {
		return err
	}

	_, err = r.at(merge.Path).git(ctx, "merge", "--ff-only", onto)
	return err
}
