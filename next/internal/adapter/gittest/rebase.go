package gittest

import (
	"context"
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ForkPoint returns the last commit span.Branch shares with span.Base, and the branch's commits after it: the fake keeps no reflog, so a fork point is their merge base.
func (f *Fake) ForkPoint(_ context.Context, span worktree.Span) (worktree.Fork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("ForkPoint", span.Branch); err != nil {
		return worktree.Fork{}, err
	}

	base, onBase := f.local[span.Base]
	branch, onBranch := f.local[span.Branch]
	if !onBase || !onBranch {
		return worktree.Fork{}, fmt.Errorf("gittest: no branch %s or %s", span.Base, span.Branch)
	}
	shared := sharedLength(base, branch)
	return worktree.Fork{Commit: branch[shared-1], Own: len(branch) - shared}, nil
}

// Rebase replays the commits of the worktree's branch after rebase.Upstream onto rebase.Onto, those Onto holds already dropped, as git drops a commit it already has. A branch seeded with Rebases stops on a conflict in its files instead: left in progress with Keep, the worktree then rebasing, else aborted. It fails on a worktree gone, on no branch, or with uncommitted changes but with Autostash.
func (f *Fake) Rebase(_ context.Context, rebase worktree.Rebase) (worktree.Rebased, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Rebase", rebase.Path); err != nil {
		return worktree.Rebased{}, err
	}

	resolved := realPath(rebase.Path)
	index := f.worktreeAt(resolved)
	if index < 0 || slices.Contains(f.gone, resolved) {
		return worktree.Rebased{}, fmt.Errorf("gittest: %s is not a worktree", rebase.Path)
	}
	state, ok := f.worktrees[index].State.(worktree.OnBranch)
	if !ok {
		return worktree.Rebased{}, fmt.Errorf("gittest: %s is on no branch", rebase.Path)
	}
	if slices.Contains(f.dirty, resolved) && !rebase.Autostash {
		return worktree.Rebased{}, fmt.Errorf("gittest: cannot rebase %s: it has uncommitted changes", rebase.Path)
	}
	onto, ok := f.historyOf(rebase.Onto)
	local := f.local[state.Branch]
	from := slices.Index(local, rebase.Upstream)
	if !ok || from < 0 {
		return worktree.Rebased{}, fmt.Errorf("gittest: unknown ref %v or %s", rebase.Onto, rebase.Upstream)
	}

	if files := f.rebases[state.Branch]; len(files) > 0 {
		if rebase.Keep {
			f.worktrees[index].State = worktree.Rebasing(state)
		}
		return worktree.Rebased{Conflicts: slices.Clone(files), Kept: rebase.Keep}, nil
	}

	replayed := slices.Clone(onto)
	for _, commit := range local[from+1:] {
		if !slices.Contains(onto, commit) {
			replayed = append(replayed, commit)
		}
	}
	f.local[state.Branch] = replayed
	return worktree.Rebased{}, nil
}

// UpdateBranch moves the local branch update.Branch to the history ending on update.To, refused unless it stands at update.From.
func (f *Fake) UpdateBranch(_ context.Context, update worktree.BranchUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("UpdateBranch", update.Branch); err != nil {
		return err
	}

	local, ok := f.local[update.Branch]
	to := f.upTo(update.To)
	if !ok || local.tip() != update.From || to == nil {
		return fmt.Errorf("gittest: %s does not stand at %s, or %s is no commit", update.Branch, update.From, update.To)
	}
	f.local[update.Branch] = slices.Clone(to)
	return nil
}

// Push sets origin's branch push.Branch, and its remote branch, to the local one, refused when origin's does not stand at push.Expect ("" when origin has none); one origin has already is pushed, as git does, whatever was expected.
func (f *Fake) Push(_ context.Context, push worktree.Push) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Push", push.Branch); err != nil {
		return err
	}

	local, ok := f.local[push.Branch]
	if !ok {
		return fmt.Errorf("gittest: no branch %s", push.Branch)
	}
	at := ""
	if commits, on := f.origin[push.Branch]; on {
		at = commits.tip()
	}
	if at == local.tip() {
		return nil
	}
	if at != push.Expect {
		return fmt.Errorf("gittest: %s: stale info, origin stands at %q", push.Branch, at)
	}

	f.origin[push.Branch], f.remote[push.Branch] = slices.Clone(local), slices.Clone(local)
	return nil
}

// NeverHeld counts the commits of the remote branch name the local one lacks: the fake keeps no reflog, and its rebase keeps each commit's name, so a commit a branch held it still holds.
func (f *Fake) NeverHeld(_ context.Context, name string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("NeverHeld", name); err != nil {
		return 0, err
	}

	remote, onRemote := f.remote[name]
	local, onLocal := f.local[name]
	if !onRemote || !onLocal {
		return 0, fmt.Errorf("gittest: no branch %s on both sides", name)
	}
	never := 0
	for _, commit := range remote {
		if !slices.Contains(local, commit) {
			never++
		}
	}
	return never, nil
}
