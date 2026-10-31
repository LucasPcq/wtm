package git

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ForkPoint returns where span.Branch forked from span.Base, with the branch's own commits after it: git's fork point, read from the base's reflog, so it is found even after the base was rewritten; their merge base when the reflog no longer knows it. Both are local branches.
func (r Repo) ForkPoint(ctx context.Context, span worktree.Span) (worktree.Fork, error) {
	base, branch := headsPrefix+span.Base, headsPrefix+span.Branch
	out, err := r.git(ctx, "merge-base", "--fork-point", base, branch)
	// git exits 1 when the reflog holds no fork point
	if failed, ok := errors.AsType[*Error](err); ok && failed.Exit == 1 {
		out, err = r.git(ctx, "merge-base", base, branch)
	}
	if err != nil {
		return worktree.Fork{}, err
	}

	fork := worktree.Fork{Commit: strings.TrimSpace(string(out))}
	count, err := r.git(ctx, "rev-list", "--count", fork.Commit+".."+branch, "--")
	if err != nil {
		return worktree.Fork{}, err
	}
	fork.Own, err = strconv.Atoi(strings.TrimSpace(string(count)))
	return fork, err
}

// Rebase replays, in the worktree at rebase.Path, its branch's commits after rebase.Upstream onto rebase.Onto. A rebase that stops on a conflict reports the files it stopped in and is aborted, the worktree back as it was, or left in progress with rebase.Keep; any other failure aborts it and is returned. With rebase.Autostash, a stash that no longer applies is kept in git's stash, which Stashed says.
func (r Repo) Rebase(ctx context.Context, rebase worktree.Rebase) (worktree.Rebased, error) {
	onto, err := r.refName(rebase.Onto)
	if err != nil {
		return worktree.Rebased{}, err
	}

	at := r.at(rebase.Path)
	stash := "--no-autostash"
	if rebase.Autostash {
		stash = "--autostash"
	}
	stashed := at.stashTip(ctx)
	// updateRefs off: a branch pointing inside the range replayed is the cascade's to move, never git's
	_, err = at.git(ctx, "-c", "rebase.updateRefs=false", "rebase", "--quiet", stash, "--onto", onto, rebase.Upstream)
	if err == nil {
		return worktree.Rebased{Stashed: rebase.Autostash && at.stashTip(ctx) != stashed}, nil
	}

	conflicts, listed := at.unmerged(ctx)
	if listed != nil || len(conflicts) == 0 {
		// git may have refused before it started: an abort with nothing to abort fails, and says nothing more
		_, _ = at.git(ctx, "rebase", "--abort")
		return worktree.Rebased{}, err
	}
	if rebase.Keep {
		return worktree.Rebased{Conflicts: conflicts, Kept: true}, nil
	}
	if _, err := at.git(ctx, "rebase", "--abort"); err != nil {
		return worktree.Rebased{Conflicts: conflicts, Kept: true}, err
	}
	return worktree.Rebased{Conflicts: conflicts}, nil
}

// stashTip is the commit git's stash stands at, "" when it holds nothing.
func (r Repo) stashTip(ctx context.Context) string {
	out, err := r.git(ctx, "rev-parse", "--quiet", "--verify", "refs/stash")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// unmerged are the files of the worktree a merge or a rebase stopped on a conflict in.
func (r Repo) unmerged(ctx context.Context) ([]string, error) {
	out, err := r.git(ctx, "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(string(out), func(c rune) bool { return c == 0 }), nil
}

// Push force-pushes the local branch push.Branch to origin's branch of the same name, refused unless origin's stands at push.Expect, "" meaning origin has none.
func (r Repo) Push(ctx context.Context, push worktree.Push) error {
	ref := headsPrefix + push.Branch
	_, err := r.git(ctx, "push", "--quiet", "--force-with-lease="+ref+":"+push.Expect, origin, ref+":"+ref)
	return err
}

// NeverHeld counts the commits of origin's branch name, as last fetched, that the local branch name never held: neither in its history nor in any commit its reflog went through. A branch rebased since it was pushed held every commit origin has; one a teammate pushed to has some it never held.
func (r Repo) NeverHeld(ctx context.Context, name string) (int, error) {
	logged, err := r.git(ctx, "reflog", "show", "--format=%H", headsPrefix+name, "--")
	if err != nil {
		return 0, err
	}

	var held bytes.Buffer
	held.WriteString("^" + headsPrefix + name + "\n")
	for _, commit := range strings.Fields(string(logged)) {
		held.WriteString("^" + commit + "\n")
	}
	out, err := r.gitWith(ctx, held.Bytes(), "rev-list", "--count", remotePrefix+name, "--stdin")
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(strings.TrimSpace(string(out)))
}
