package gittest

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

// Fake is git in memory: what its writes change, its reads give back, and it
// refuses what git refuses, so an undo is tested against the state it restores.
type Fake struct {
	mu        sync.Mutex
	worktrees []git.Worktree
	local     []string
	remote    []string
	counts    map[git.AheadBehindParams]git.AheadBehind
	fail      map[string]error
	calls     []string
}

// FakeParams starts the fake: the main worktree on Branch (which is a local
// branch) at Main, more Local and Remote branches, the AheadBehind of a pair,
// and the error a method returns instead of running, keyed by its name.
type FakeParams struct {
	Main        string
	Branch      string
	Local       []string
	Remote      []string
	AheadBehind map[git.AheadBehindParams]git.AheadBehind
	Fail        map[string]error
}

var (
	_ git.Reader = (*Fake)(nil)
	_ git.Writer = (*Fake)(nil)
)

func NewFake(params FakeParams) *Fake {
	return &Fake{
		worktrees: []git.Worktree{{Path: params.Main, Main: true, State: git.OnBranch{Branch: params.Branch}}},
		local:     append([]string{params.Branch}, params.Local...),
		remote:    slices.Clone(params.Remote),
		counts:    params.AheadBehind,
		fail:      params.Fail,
	}
}

// Calls are the methods run, writes and reads, each with its main argument.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *Fake) Worktrees(context.Context) ([]git.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Worktrees", ""); err != nil {
		return nil, err
	}
	return slices.Clone(f.worktrees), nil
}

func (f *Fake) MainWorktree(context.Context) (git.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MainWorktree", ""); err != nil {
		return git.Worktree{}, err
	}
	return f.worktrees[0], nil
}

func (f *Fake) Branches(context.Context) (git.Branches, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Branches", ""); err != nil {
		return git.Branches{}, err
	}
	return git.Branches{Local: slices.Sorted(slices.Values(f.local)), Remote: slices.Sorted(slices.Values(f.remote))}, nil
}

func (f *Fake) RefExists(_ context.Context, ref git.Ref) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RefExists", fmt.Sprint(ref)); err != nil {
		return false, err
	}
	return f.exists(ref), nil
}

func (f *Fake) AheadBehind(_ context.Context, params git.AheadBehindParams) (git.AheadBehind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AheadBehind", fmt.Sprint(params.Of, params.Against)); err != nil {
		return git.AheadBehind{}, err
	}
	if !f.exists(params.Of) || !f.exists(params.Against) {
		return git.AheadBehind{}, fmt.Errorf("gittest: unknown ref in %v", params)
	}
	return f.counts[params], nil
}

func (f *Fake) AddWorktree(_ context.Context, params git.AddWorktreeParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AddWorktree", params.Path); err != nil {
		return err
	}
	if f.worktreeAt(params.Path) >= 0 {
		return fmt.Errorf("gittest: %s is already a worktree", params.Path)
	}
	switch checkout := params.Checkout.(type) {
	case git.NewBranch:
		if slices.Contains(f.local, checkout.Name) {
			return fmt.Errorf("gittest: branch %s already exists", checkout.Name)
		}
		if !f.exists(checkout.From) {
			return fmt.Errorf("gittest: invalid start point %v", checkout.From)
		}
		f.local = append(f.local, checkout.Name)
		return f.add(params.Path, checkout.Name)
	case git.ExistingBranch:
		if !slices.Contains(f.local, checkout.Name) {
			return fmt.Errorf("gittest: no branch %s", checkout.Name)
		}
		return f.add(params.Path, checkout.Name)
	}
	return fmt.Errorf("gittest: no checkout given")
}

func (f *Fake) RemoveWorktree(_ context.Context, params git.RemoveWorktreeParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RemoveWorktree", params.Path); err != nil {
		return err
	}
	index := f.worktreeAt(params.Path)
	if index < 0 {
		return fmt.Errorf("gittest: %s is not a worktree", params.Path)
	}
	if f.worktrees[index].Main {
		return fmt.Errorf("gittest: %s is the main worktree", params.Path)
	}
	f.worktrees = slices.Delete(f.worktrees, index, index+1)
	return nil
}

func (f *Fake) DeleteBranch(_ context.Context, params git.DeleteBranchParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("DeleteBranch", params.Name); err != nil {
		return err
	}
	index := slices.Index(f.local, params.Name)
	if index < 0 {
		return fmt.Errorf("gittest: no branch %s", params.Name)
	}
	if f.checkedOut(params.Name) {
		return fmt.Errorf("gittest: branch %s is checked out", params.Name)
	}
	f.local = slices.Delete(f.local, index, index+1)
	return nil
}

func (f *Fake) Fetch(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enter("Fetch", "")
}

func (f *Fake) FetchRef(_ context.Context, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FetchRef", branch); err != nil {
		return err
	}
	if !slices.Contains(f.remote, branch) {
		return fmt.Errorf("gittest: no remote branch %s", branch)
	}
	return nil
}

func (f *Fake) FastForwardRef(_ context.Context, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FastForwardRef", branch); err != nil {
		return err
	}
	if !slices.Contains(f.remote, branch) {
		return fmt.Errorf("gittest: no remote branch %s", branch)
	}
	if f.checkedOut(branch) {
		return fmt.Errorf("gittest: branch %s is checked out", branch)
	}
	return nil
}

func (f *Fake) MergeFFOnly(_ context.Context, params git.MergeFFOnlyParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MergeFFOnly", params.Worktree); err != nil {
		return err
	}
	if f.worktreeAt(params.Worktree) < 0 {
		return fmt.Errorf("gittest: %s is not a worktree", params.Worktree)
	}
	if !f.exists(params.Onto) {
		return fmt.Errorf("gittest: unknown ref %v", params.Onto)
	}
	return nil
}

func (f *Fake) enter(method, argument string) error {
	f.calls = append(f.calls, method+" "+argument)
	return f.fail[method]
}

func (f *Fake) add(path, branch string) error {
	if f.checkedOut(branch) {
		return fmt.Errorf("gittest: branch %s is checked out", branch)
	}
	f.worktrees = append(f.worktrees, git.Worktree{Path: path, State: git.OnBranch{Branch: branch}})
	return nil
}

func (f *Fake) exists(ref git.Ref) bool {
	switch branch := ref.(type) {
	case git.LocalBranch:
		return slices.Contains(f.local, branch.Name)
	case git.RemoteBranch:
		return slices.Contains(f.remote, branch.Name)
	}
	return false
}

func (f *Fake) worktreeAt(path string) int {
	return slices.IndexFunc(f.worktrees, func(worktree git.Worktree) bool { return worktree.Path == path })
}

func (f *Fake) checkedOut(branch string) bool {
	return slices.ContainsFunc(f.worktrees, func(worktree git.Worktree) bool {
		state, ok := worktree.State.(git.OnBranch)
		return ok && state.Branch == branch
	})
}
