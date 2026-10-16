package gittest

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

// Fake keeps a branch as its history, the commits from the root, so a count and
// a fast-forward follow what the writes did.
type Fake struct {
	mu        sync.Mutex
	dir       string
	worktrees []git.Worktree
	local     map[string]history
	remote    map[string]history
	origin    map[string]history
	folders   []string
	dirty     []string
	fail      map[string]error
	calls     []string
}

type FakeParams struct {
	Main      string
	Dir       string
	Branch    string
	Local     []string
	Remote    []string
	Worktrees []git.Worktree
	Folders   []string
	Dirty     []string
	Commits   map[git.Ref][]string
	Origin    map[string][]string
	Fail      map[string]error
}

const Root = "root"

type history []string

var (
	_ git.Reader = (*Fake)(nil)
	_ git.Writer = (*Fake)(nil)
)

func NewFake(params FakeParams) *Fake {
	main := realPath(params.Main)
	dir := main
	if params.Dir != "" {
		dir = realPath(params.Dir)
	}
	f := &Fake{
		dir:       dir,
		worktrees: []git.Worktree{{Path: main, Main: true, State: git.OnBranch{Branch: params.Branch}}},
		local:     map[string]history{},
		remote:    map[string]history{},
		origin:    map[string]history{},
		fail:      params.Fail,
	}
	for _, branch := range append([]string{params.Branch}, params.Local...) {
		f.local[branch] = historyOf(params.Commits, git.LocalBranch{Name: branch})
	}
	for _, worktree := range params.Worktrees {
		worktree.Path = realPath(worktree.Path)
		f.worktrees = append(f.worktrees, worktree)
		if branch := branchOf(worktree); branch != "" {
			f.local[branch] = historyOf(params.Commits, git.LocalBranch{Name: branch})
		}
	}
	for _, branch := range params.Remote {
		f.remote[branch] = historyOf(params.Commits, git.RemoteBranch{Name: branch})
		f.origin[branch] = f.remote[branch]
	}
	for branch, commits := range params.Origin {
		f.origin[branch] = append(history{Root}, commits...)
	}
	for _, folder := range params.Folders {
		f.folders = append(f.folders, realPath(folder))
	}
	for _, dirty := range params.Dirty {
		f.dirty = append(f.dirty, realPath(dirty))
	}
	return f
}

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
	return f.listed(), nil
}

func (f *Fake) MainWorktree(context.Context) (git.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MainWorktree", ""); err != nil {
		return git.Worktree{}, err
	}
	return f.listed()[0], nil
}

func (f *Fake) Toplevel(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Toplevel", ""); err != nil {
		return "", err
	}
	index := f.worktreeOf(f.dir)
	if index < 0 {
		return "", fmt.Errorf("gittest: %s is not in a worktree", f.dir)
	}
	return f.worktrees[index].Path, nil
}

func (f *Fake) GitCommonDir(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("GitCommonDir", ""); err != nil {
		return "", err
	}
	return filepath.Join(f.worktrees[0].Path, ".git"), nil
}

func (f *Fake) Locate(_ context.Context, path string) (git.Location, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Locate", path); err != nil {
		return git.Location{}, err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.dir, path)
	}
	resolved := realPath(path)
	return git.Location{Path: resolved, Exists: f.taken(resolved)}, nil
}

func (f *Fake) Branches(context.Context) (git.Branches, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Branches", ""); err != nil {
		return git.Branches{}, err
	}
	return git.Branches{Local: branchesOf(f.local), Remote: branchesOf(f.remote)}, nil
}

func branchesOf(refs map[string]history) []git.Branch {
	branches := make([]git.Branch, 0, len(refs))
	for _, name := range slices.Sorted(maps.Keys(refs)) {
		branches = append(branches, git.Branch{Name: name, Tip: refs[name].tip()})
	}
	return branches
}

func (f *Fake) RefExists(_ context.Context, ref git.Ref) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RefExists", fmt.Sprint(ref)); err != nil {
		return false, err
	}
	_, ok := f.historyOf(ref)
	return ok, nil
}

func (f *Fake) Tip(_ context.Context, ref git.Ref) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Tip", fmt.Sprint(ref)); err != nil {
		return "", err
	}
	commits, ok := f.historyOf(ref)
	if !ok {
		return "", fmt.Errorf("gittest: unknown ref %v", ref)
	}
	return commits.tip(), nil
}

func (f *Fake) AheadBehind(_ context.Context, params git.AheadBehindParams) (git.AheadBehind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AheadBehind", fmt.Sprint(params.Of, params.Against)); err != nil {
		return git.AheadBehind{}, err
	}
	of, ofOK := f.historyOf(params.Of)
	against, againstOK := f.historyOf(params.Against)
	if !ofOK || !againstOK {
		return git.AheadBehind{}, fmt.Errorf("gittest: unknown ref in %v", params)
	}
	shared := sharedLength(of, against)
	return git.AheadBehind{Ahead: len(of) - shared, Behind: len(against) - shared}, nil
}

// AddWorktree creates a new branch before it looks at the path, as git does:
// an add refused for its path leaves the branch.
func (f *Fake) AddWorktree(_ context.Context, params git.AddWorktreeParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AddWorktree", params.Path); err != nil {
		return err
	}
	path := realPath(params.Path)
	switch checkout := params.Checkout.(type) {
	case git.NewBranch:
		if _, ok := f.local[checkout.Name]; ok {
			return fmt.Errorf("gittest: branch %s already exists", checkout.Name)
		}
		from, ok := f.historyOf(checkout.From)
		if !ok {
			return fmt.Errorf("gittest: invalid start point %v", checkout.From)
		}
		f.local[checkout.Name] = slices.Clone(from)
		return f.add(path, checkout.Name)
	case git.ExistingBranch:
		if _, ok := f.local[checkout.Name]; !ok {
			return fmt.Errorf("%w: %s", git.ErrNoLocalBranch, checkout.Name)
		}
		if f.checkedOut(checkout.Name) {
			return fmt.Errorf("gittest: branch %s is checked out", checkout.Name)
		}
		return f.add(path, checkout.Name)
	}
	return fmt.Errorf("gittest: no checkout given")
}

// RemoveWorktree keeps a locked worktree even under Force: git asks for it
// twice. A worktree whose directory is gone needs no Force.
func (f *Fake) RemoveWorktree(_ context.Context, params git.RemoveWorktreeParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RemoveWorktree", params.Path); err != nil {
		return err
	}
	path := realPath(params.Path)
	index := f.worktreeAt(path)
	if index < 0 {
		return fmt.Errorf("gittest: %s is not a worktree", path)
	}
	worktree := f.worktrees[index]
	switch {
	case worktree.Main:
		return fmt.Errorf("gittest: %s is the main worktree", path)
	case worktree.Locked:
		return fmt.Errorf("gittest: %s is locked", path)
	case !worktree.Prunable && !params.Force && slices.Contains(f.dirty, path):
		return fmt.Errorf("gittest: %s has changes", path)
	}
	f.worktrees = slices.Delete(f.worktrees, index, index+1)
	f.dirty = slices.DeleteFunc(f.dirty, func(dirty string) bool { return dirty == path })
	return nil
}

// DeleteBranch without Force keeps a branch the HEAD of Dir lacks commits of.
func (f *Fake) DeleteBranch(_ context.Context, params git.DeleteBranchParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("DeleteBranch", params.Name); err != nil {
		return err
	}
	commits, ok := f.local[params.Name]
	if !ok {
		return fmt.Errorf("gittest: no branch %s", params.Name)
	}
	if f.checkedOut(params.Name) {
		return fmt.Errorf("gittest: branch %s is checked out", params.Name)
	}
	if !params.Force && !commits.within(f.head()) {
		return fmt.Errorf("gittest: branch %s is not fully merged", params.Name)
	}
	delete(f.local, params.Name)
	return nil
}

func (f *Fake) Fetch(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Fetch", ""); err != nil {
		return err
	}
	maps.Copy(f.remote, f.origin)
	return nil
}

func (f *Fake) FetchRef(_ context.Context, branch string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FetchRef", branch); err != nil {
		return err
	}
	commits, ok := f.origin[branch]
	if !ok {
		return fmt.Errorf("gittest: no remote branch %s", branch)
	}
	f.remote[branch] = commits
	return nil
}

// FastForwardRef brings the remote branch even when it refuses to move the
// local one, as `git fetch <remote> b:b` does.
func (f *Fake) FastForwardRef(_ context.Context, branch string) (git.Move, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FastForwardRef", branch); err != nil {
		return git.Move{}, err
	}
	commits, ok := f.origin[branch]
	if !ok {
		return git.Move{}, fmt.Errorf("gittest: no remote branch %s", branch)
	}
	if f.checkedOut(branch) {
		return git.Move{}, fmt.Errorf("gittest: branch %s is checked out", branch)
	}
	f.remote[branch] = commits
	local, existed := f.local[branch]
	if existed && !local.within(commits) {
		return git.Move{}, fmt.Errorf("gittest: %s is not a fast-forward", branch)
	}
	f.local[branch] = slices.Clone(commits)
	if !existed {
		return git.Move{To: commits.tip()}, nil
	}
	return git.Move{From: local.tip(), To: commits.tip()}, nil
}

// MergeFFOnly moves a worktree's branch; a detached HEAD has no history the
// fake keeps.
func (f *Fake) MergeFFOnly(_ context.Context, params git.MergeFFOnlyParams) (git.Move, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MergeFFOnly", params.Worktree); err != nil {
		return git.Move{}, err
	}
	index := f.worktreeAt(realPath(params.Worktree))
	if index < 0 || f.worktrees[index].Prunable {
		return git.Move{}, fmt.Errorf("gittest: %s is not a worktree", params.Worktree)
	}
	onto, ok := f.historyOf(params.Onto)
	if !ok {
		return git.Move{}, fmt.Errorf("gittest: unknown ref %v", params.Onto)
	}
	state, ok := f.worktrees[index].State.(git.OnBranch)
	if !ok {
		return git.Move{}, fmt.Errorf("gittest: %s is on no branch", params.Worktree)
	}
	local := f.local[state.Branch]
	switch {
	case onto.within(local):
		return git.Move{From: local.tip(), To: local.tip()}, nil
	case !local.within(onto):
		return git.Move{}, fmt.Errorf("gittest: %v is not a fast-forward of %s", params.Onto, state.Branch)
	}
	f.local[state.Branch] = slices.Clone(onto)
	return git.Move{From: local.tip(), To: onto.tip()}, nil
}

func (f *Fake) enter(method, argument string) error {
	f.calls = append(f.calls, method+" "+argument)
	return f.fail[method]
}

func (f *Fake) add(path, branch string) error {
	if f.worktreeAt(path) >= 0 || f.taken(path) {
		return fmt.Errorf("gittest: %s already exists", path)
	}
	f.worktrees = append(f.worktrees, git.Worktree{Path: path, State: git.OnBranch{Branch: branch}})
	return nil
}

// listed sorts the linked worktrees by path after the main one, as git lists
// them.
func (f *Fake) listed() []git.Worktree {
	worktrees := slices.Clone(f.worktrees)
	slices.SortStableFunc(worktrees[1:], func(a, b git.Worktree) int { return strings.Compare(a.Path, b.Path) })
	for i, worktree := range worktrees {
		worktrees[i].Head = Root
		if state, ok := worktree.State.(git.OnBranch); ok {
			worktrees[i].Head = f.local[state.Branch].tip()
		}
	}
	return worktrees
}

func (f *Fake) head() history {
	index := f.worktreeOf(f.dir)
	if index < 0 {
		return history{Root}
	}
	state, ok := f.worktrees[index].State.(git.OnBranch)
	if !ok {
		return history{Root}
	}
	return f.local[state.Branch]
}

func (f *Fake) worktreeOf(path string) int {
	found := -1
	for i, worktree := range f.worktrees {
		if !worktree.Prunable && within(path, worktree.Path) && (found < 0 || len(worktree.Path) > len(f.worktrees[found].Path)) {
			found = i
		}
	}
	return found
}

func (f *Fake) historyOf(ref git.Ref) (history, bool) {
	switch branch := ref.(type) {
	case git.LocalBranch:
		commits, ok := f.local[branch.Name]
		return commits, ok
	case git.RemoteBranch:
		commits, ok := f.remote[branch.Name]
		return commits, ok
	}
	return nil, false
}

// taken is anything on disk at path or below it: a folder, or the directory of
// a worktree that still has one.
func (f *Fake) taken(path string) bool {
	for _, worktree := range f.worktrees {
		if !worktree.Prunable && within(worktree.Path, path) {
			return true
		}
	}
	return slices.ContainsFunc(f.folders, func(folder string) bool { return within(folder, path) })
}

func (f *Fake) worktreeAt(path string) int {
	return slices.IndexFunc(f.worktrees, func(worktree git.Worktree) bool { return worktree.Path == path })
}

func (f *Fake) checkedOut(branch string) bool {
	return slices.ContainsFunc(f.worktrees, func(worktree git.Worktree) bool { return branchOf(worktree) == branch })
}

func historyOf(commits map[git.Ref][]string, ref git.Ref) history {
	return append(history{Root}, commits[ref]...)
}

// branchOf counts a branch a rebase will move as checked out: git refuses to
// check it out elsewhere or delete it.
func branchOf(worktree git.Worktree) string {
	switch state := worktree.State.(type) {
	case git.OnBranch:
		return state.Branch
	case git.Rebasing:
		return state.Branch
	case git.Detached, git.Bare:
	}
	return ""
}

func (h history) tip() string { return h[len(h)-1] }

// within is a history that other contains whole: merged into it.
func (h history) within(other history) bool {
	return sharedLength(h, other) == len(h)
}

func sharedLength(a, b history) int {
	shared := 0
	for shared < len(a) && shared < len(b) && a[shared] == b[shared] {
		shared++
	}
	return shared
}

func within(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// realPath resolves a path as git does; an error leaves it as written, which
// is what a path the fake invented needs.
func realPath(path string) string {
	resolved, err := git.RealPath(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}
