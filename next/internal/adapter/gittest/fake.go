package gittest

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

type Fake struct {
	mu        sync.Mutex
	worktrees []worktree.Worktree
	gone      []string
	local     map[string]history
	remote    map[string]history
	origin    map[string]history
	folders   []string
	empty     []string
	dir       string
	dirty     []string
	fail      map[string]error
	calls     []string
}

type FakeParams struct {
	Main      string
	Branch    string
	Local     []string
	Remote    []string
	Worktrees []worktree.Worktree
	Gone      []string
	Folders   []string
	Empty     []string
	Dir       string
	Dirty     []string
	Commits   map[branch.Ref][]string
	Origin    map[string][]string
	Fail      map[string]error
}

type history []string

func NewFake(params FakeParams) *Fake {
	f := &Fake{
		worktrees: []worktree.Worktree{{Path: realPath(params.Main), Main: true, State: worktree.OnBranch{Branch: params.Branch}}},
		local:     map[string]history{},
		remote:    map[string]history{},
		origin:    map[string]history{},
		gone:      realPaths(params.Gone),
		folders:   realPaths(params.Folders),
		empty:     realPaths(params.Empty),
		dir:       realPath(cmp.Or(params.Dir, params.Main)),
		dirty:     realPaths(params.Dirty),
		fail:      params.Fail,
	}
	for _, name := range append([]string{params.Branch}, params.Local...) {
		f.local[name] = historyOf(params.Commits, branch.LocalBranch{Name: name})
	}
	for _, w := range params.Worktrees {
		w.Path = realPath(w.Path)
		f.worktrees = append(f.worktrees, w)
		if name := w.Branch(); name != "" {
			f.local[name] = historyOf(params.Commits, branch.LocalBranch{Name: name})
		}
	}
	for _, name := range params.Remote {
		f.remote[name] = historyOf(params.Commits, branch.RemoteBranch{Name: name})
		f.origin[name] = f.remote[name]
	}
	for name, commits := range params.Origin {
		f.origin[name] = append(history{Root}, commits...)
	}
	return f
}

func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *Fake) Worktrees(context.Context) ([]worktree.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Worktrees", ""); err != nil {
		return nil, err
	}
	return f.listed(), nil
}

func (f *Fake) MainWorktree(context.Context) (worktree.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MainWorktree", ""); err != nil {
		return worktree.Worktree{}, err
	}
	return f.worktrees[0], nil
}

func (f *Fake) GitCommonDir(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("GitCommonDir", ""); err != nil {
		return "", err
	}
	return filepath.Join(f.worktrees[0].Path, ".git"), nil
}

func (f *Fake) Locate(_ context.Context, path string) (worktree.Location, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Locate", path); err != nil {
		return worktree.Location{}, err
	}
	absolute := path
	if !filepath.IsAbs(path) {
		absolute = filepath.Join(f.dir, path)
	}
	resolved := realPath(absolute)
	return worktree.Location{Path: resolved, Exists: f.taken(resolved) || slices.Contains(f.empty, resolved)}, nil
}

func (f *Fake) Branches(context.Context) (branch.Branches, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Branches", ""); err != nil {
		return branch.Branches{}, err
	}
	return branch.Branches{Local: branchesOf(f.local), Remote: branchesOf(f.remote)}, nil
}

func (f *Fake) AheadBehind(_ context.Context, name string) (branch.AheadBehind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AheadBehind", name); err != nil {
		return branch.AheadBehind{}, err
	}
	of, ofOK := f.historyOf(branch.LocalBranch{Name: name})
	against, againstOK := f.historyOf(branch.RemoteBranch{Name: name})
	if !ofOK || !againstOK {
		return branch.AheadBehind{}, fmt.Errorf("gittest: unknown ref %s or its remote", name)
	}
	shared := sharedLength(of, against)
	return branch.AheadBehind{Ahead: len(of) - shared, Behind: len(against) - shared}, nil
}

func (f *Fake) Dirty(_ context.Context, path string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Dirty", path); err != nil {
		return false, err
	}
	resolved := realPath(path)
	if f.worktreeAt(resolved) < 0 || slices.Contains(f.gone, resolved) {
		return false, fmt.Errorf("gittest: %s is not a worktree", resolved)
	}
	return slices.Contains(f.dirty, resolved), nil
}

func (f *Fake) AddWorktree(_ context.Context, addition worktree.Addition) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AddWorktree", addition.Path); err != nil {
		return err
	}
	created, creates := worktree.CreatedBranch(addition.Checkout)
	_, existed := f.local[created]
	err := f.add(addition)
	if err == nil || !creates || existed {
		return err
	}
	if _, left := f.local[created]; !left {
		return err
	}
	return errors.Join(err, f.deleteBranch(created))
}

func (f *Fake) add(addition worktree.Addition) error {
	path := realPath(addition.Path)
	switch checkout := addition.Checkout.(type) {
	case worktree.NewBranch:
		if _, ok := f.local[checkout.Name]; ok {
			return fmt.Errorf("gittest: branch %s already exists", checkout.Name)
		}
		from, ok := f.historyOf(checkout.From)
		if !ok {
			return fmt.Errorf("gittest: invalid start point %v", checkout.From)
		}
		f.local[checkout.Name] = slices.Clone(from)
		return f.place(path, checkout.Name)
	case worktree.ExistingBranch:
		if _, ok := f.local[checkout.Name]; !ok {
			return fmt.Errorf("%w: %s", git.ErrNoLocalBranch, checkout.Name)
		}
		if f.checkedOut(checkout.Name) {
			return fmt.Errorf("gittest: branch %s is checked out", checkout.Name)
		}
		return f.place(path, checkout.Name)
	case worktree.TrackingBranch:
		if _, ok := f.local[checkout.Name]; ok {
			return fmt.Errorf("gittest: branch %s already exists", checkout.Name)
		}
		from, ok := f.remote[checkout.Name]
		if !ok {
			return fmt.Errorf("gittest: invalid start point %s/%s", origin, checkout.Name)
		}
		f.local[checkout.Name] = slices.Clone(from)
		return f.place(path, checkout.Name)
	}
	return errors.New("gittest: no checkout given")
}

func (f *Fake) RemoveWorktree(_ context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RemoveWorktree", path); err != nil {
		return err
	}
	resolved := realPath(path)
	index := f.worktreeAt(resolved)
	switch {
	case index < 0:
		return fmt.Errorf("gittest: %s is not a worktree", resolved)
	case f.worktrees[index].Main:
		return fmt.Errorf("gittest: %s is the main worktree", resolved)
	}
	f.worktrees = slices.Delete(f.worktrees, index, index+1)
	f.dirty = slices.DeleteFunc(f.dirty, func(dirty string) bool { return dirty == resolved })
	f.gone = slices.DeleteFunc(f.gone, func(gone string) bool { return gone == resolved })
	return nil
}

func (f *Fake) DeleteBranch(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleteBranch(name)
}

func (f *Fake) deleteBranch(name string) error {
	if err := f.enter("DeleteBranch", name); err != nil {
		return err
	}
	if _, ok := f.local[name]; !ok {
		return fmt.Errorf("gittest: no branch %s", name)
	}
	if f.checkedOut(name) {
		return fmt.Errorf("gittest: branch %s is checked out", name)
	}
	delete(f.local, name)
	return nil
}

func (f *Fake) FetchRef(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FetchRef", name); err != nil {
		return err
	}
	commits, ok := f.origin[name]
	if !ok {
		return fmt.Errorf("gittest: no remote branch %s", name)
	}
	f.remote[name] = commits
	return nil
}

func (f *Fake) FastForwardRef(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("FastForwardRef", name); err != nil {
		return err
	}
	commits, ok := f.origin[name]
	if !ok {
		return fmt.Errorf("gittest: no remote branch %s", name)
	}
	if f.checkedOut(name) {
		return fmt.Errorf("gittest: branch %s is checked out", name)
	}
	f.remote[name] = commits
	if local, existed := f.local[name]; existed && !local.within(commits) {
		return fmt.Errorf("gittest: %s is not a fast-forward", name)
	}
	f.local[name] = slices.Clone(commits)
	return nil
}

func (f *Fake) MergeFFOnly(_ context.Context, merge worktree.Merge) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MergeFFOnly", merge.Path); err != nil {
		return err
	}
	resolved := realPath(merge.Path)
	index := f.worktreeAt(resolved)
	if index < 0 || slices.Contains(f.gone, resolved) {
		return fmt.Errorf("gittest: %s is not a worktree", merge.Path)
	}
	onto, ok := f.historyOf(merge.Onto)
	if !ok {
		return fmt.Errorf("gittest: unknown ref %v", merge.Onto)
	}
	state, ok := f.worktrees[index].State.(worktree.OnBranch)
	if !ok {
		return fmt.Errorf("gittest: %s is on no branch", merge.Path)
	}
	local := f.local[state.Branch]
	switch {
	case onto.within(local):
		return nil
	case !local.within(onto):
		return fmt.Errorf("gittest: %v is not a fast-forward of %s", merge.Onto, state.Branch)
	}
	f.local[state.Branch] = slices.Clone(onto)
	return nil
}

func (f *Fake) enter(method, argument string) error {
	f.calls = append(f.calls, method+" "+argument)
	return f.fail[method]
}

func (f *Fake) place(path, name string) error {
	if f.worktreeAt(path) >= 0 || f.taken(path) {
		return fmt.Errorf("gittest: %s already exists", path)
	}
	f.worktrees = append(f.worktrees, worktree.Worktree{Path: path, State: worktree.OnBranch{Branch: name}})
	return nil
}

func (f *Fake) listed() []worktree.Worktree {
	worktrees := slices.Clone(f.worktrees)
	slices.SortStableFunc(worktrees[1:], func(a, b worktree.Worktree) int { return strings.Compare(a.Path, b.Path) })
	return worktrees
}

func (f *Fake) historyOf(ref branch.Ref) (history, bool) {
	switch b := ref.(type) {
	case branch.LocalBranch:
		commits, ok := f.local[b.Name]
		return commits, ok
	case branch.RemoteBranch:
		commits, ok := f.remote[b.Name]
		return commits, ok
	}
	return nil, false
}

func (f *Fake) taken(path string) bool {
	live := func(w worktree.Worktree) bool { return !slices.Contains(f.gone, w.Path) && within(w.Path, path) }
	return slices.ContainsFunc(f.worktrees, live) || slices.ContainsFunc(f.folders, func(folder string) bool { return within(folder, path) })
}

func (f *Fake) worktreeAt(path string) int {
	return slices.IndexFunc(f.worktrees, func(w worktree.Worktree) bool { return w.Path == path })
}

func (f *Fake) checkedOut(name string) bool {
	return slices.ContainsFunc(f.worktrees, func(w worktree.Worktree) bool { return w.Branch() == name })
}

func branchesOf(refs map[string]history) []branch.Branch {
	branches := make([]branch.Branch, 0, len(refs))
	for _, name := range slices.Sorted(maps.Keys(refs)) {
		branches = append(branches, branch.Branch{Name: name, Tip: refs[name].tip()})
	}
	return branches
}

func historyOf(commits map[branch.Ref][]string, ref branch.Ref) history {
	return append(history{Root}, commits[ref]...)
}

func (h history) tip() string { return h[len(h)-1] }

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

func realPath(path string) string {
	resolved, err := git.RealPath(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

func realPaths(paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, realPath(path))
	}
	return resolved
}
