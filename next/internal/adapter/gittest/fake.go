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
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Fake simulates a git repository in memory, with the methods of git.Repo the
// ports call. A commit is its message and a branch the list of commits from
// Root to its tip, so ahead, behind and fast-forwards follow from the lists.
// A test seeds it with NewFake, makes any method fail through FakeParams.Fail,
// and reads back what was called with Calls and what was moved with Moves.
// It is safe for concurrent use.
type Fake struct {
	mu        sync.Mutex
	worktrees []worktree.Worktree
	gone      []string
	local     map[string]history
	remote    map[string]history
	origin    map[string]history
	tracking  []string
	folders   []string
	empty     []string
	dir       string
	dirty     []string
	fail      map[string]error
	calls     []string
	changes   map[string][]worktree.Change
	diffstats map[string]worktree.Diffstat
	conflicts map[string]worktree.ConflictKind
	moves     []worktree.Move
	dates     map[string]time.Time
}

// FakeParams describes a repository, for NewFake and Build alike. Every branch
// starts at Root; Commits adds commits on top of it.
type FakeParams struct {
	Main      string                           // the main worktree
	Branch    string                           // the branch checked out in the main worktree
	Local     []string                         // more local branches, besides those the worktrees are on
	Remote    []string                         // branches on origin and already fetched
	Tracking  []string                         // local branches whose upstream is origin's branch of the same name
	Deleted   []string                         // branches of Remote deleted on origin since they were fetched
	Worktrees []worktree.Worktree              // linked worktrees
	Gone      []string                         // worktrees whose directory was deleted behind git's back
	Folders   []string                         // directories holding a file, where no worktree can go
	Empty     []string                         // directories that exist and are empty
	Dir       string                           // where relative paths start from; Main when empty (fake only)
	Dirty     []string                         // worktrees with uncommitted changes
	Commits   map[worktree.Ref][]string        // a branch's commits after Root
	Origin    map[string][]string              // commits after Root on origin's branch, not fetched yet
	Fail      map[string]error                 // the error a method returns, by method name (fake only)
	Changes   map[string][]worktree.Change     // the uncommitted changes Changes lists, by worktree (fake only)
	Diffstats map[string]worktree.Diffstat     // what Diffstat measures, by worktree (fake only)
	Conflicts map[string]worktree.ConflictKind // the paths a move conflicts on, by path (fake only)
	Dates     map[string]time.Time             // when a local branch's tip was committed, Committed when not given (fake only)
}

// Committed is when the tip of a local branch was committed, unless FakeParams.Dates says otherwise.
var Committed = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// history is a branch's commits, from Root to its tip.
type history []string

// NewFake returns a Fake holding the repository params describes, every path
// resolved as git would.
func NewFake(params FakeParams) *Fake {
	f := &Fake{
		worktrees: []worktree.Worktree{{Path: realPath(params.Main), Main: true, State: worktree.OnBranch{Branch: params.Branch}}},
		local:     map[string]history{},
		remote:    map[string]history{},
		origin:    map[string]history{},
		tracking:  params.Tracking,
		gone:      realPaths(params.Gone),
		folders:   realPaths(params.Folders),
		empty:     realPaths(params.Empty),
		dir:       realPath(cmp.Or(params.Dir, params.Main)),
		dirty:     realPaths(params.Dirty),
		fail:      params.Fail,
		changes:   map[string][]worktree.Change{},
		diffstats: map[string]worktree.Diffstat{},
		conflicts: params.Conflicts,
		dates:     params.Dates,
	}

	for dir, stat := range params.Diffstats {
		f.diffstats[realPath(dir)] = stat
	}

	for dir, changes := range params.Changes {
		f.changes[realPath(dir)] = changes
	}

	for _, name := range append([]string{params.Branch}, params.Local...) {
		f.local[name] = historyOf(params.Commits, worktree.LocalBranch{Name: name})
	}

	for _, w := range params.Worktrees {
		w.Path = realPath(w.Path)
		f.worktrees = append(f.worktrees, w)
		if name := w.Branch(); name != "" {
			f.local[name] = historyOf(params.Commits, worktree.LocalBranch{Name: name})
		}
	}

	for _, name := range params.Remote {
		f.remote[name] = historyOf(params.Commits, worktree.RemoteBranch{Name: name})
		f.origin[name] = f.remote[name]
	}

	for name, commits := range params.Origin {
		f.origin[name] = append(history{Root}, commits...)
	}

	for _, name := range params.Deleted {
		delete(f.origin, name)
	}

	return f
}

// Calls returns every call made so far, in order, each as its method name, a
// space and its main argument.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// ----------------------------------------------------------------------------
// Reading the repository

// Worktrees returns the worktrees, the main one first and the others by path,
// as git lists them.
func (f *Fake) Worktrees(context.Context) ([]worktree.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Worktrees", ""); err != nil {
		return nil, err
	}

	return f.listed(), nil
}

// MainWorktree returns the main worktree.
func (f *Fake) MainWorktree(context.Context) (worktree.Worktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("MainWorktree", ""); err != nil {
		return worktree.Worktree{}, err
	}

	return f.worktrees[0], nil
}

// CommonDir returns the main worktree's .git directory.
func (f *Fake) CommonDir(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("CommonDir", ""); err != nil {
		return "", err
	}

	return filepath.Join(f.worktrees[0].Path, ".git"), nil
}

// Locate resolves path, relative to the fake's Dir, and reports it existing
// when a live worktree or a seeded folder lies at or under it, or it is a
// seeded empty directory.
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

// Branches returns the local and the fetched remote branches, sorted by name,
// each with its tip.
func (f *Fake) Branches(context.Context) (worktree.Branches, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Branches", ""); err != nil {
		return worktree.Branches{}, err
	}

	return worktree.Branches{Local: branchesOf(f.local), Remote: branchesOf(f.remote)}, nil
}

// CommitDates returns when the tip of each local branch was committed: as
// seeded, else Committed.
func (f *Fake) CommitDates(context.Context) (map[string]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("CommitDates", ""); err != nil {
		return nil, err
	}

	dates := map[string]time.Time{}
	for name := range f.local {
		dates[name] = cmp.Or(f.dates[name], Committed)
	}
	return dates, nil
}

// AheadBehind counts the commits of the local branch name past the history it
// shares with its fetched remote branch, and the reverse; it fails when either
// is missing.
func (f *Fake) AheadBehind(_ context.Context, name string) (worktree.AheadBehind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AheadBehind", name); err != nil {
		return worktree.AheadBehind{}, err
	}

	of, ofOK := f.historyOf(worktree.LocalBranch{Name: name})
	against, againstOK := f.historyOf(worktree.RemoteBranch{Name: name})
	if !ofOK || !againstOK {
		return worktree.AheadBehind{}, fmt.Errorf("gittest: unknown ref %s or its remote", name)
	}

	shared := sharedLength(of, against)
	return worktree.AheadBehind{Ahead: len(of) - shared, Behind: len(against) - shared}, nil
}

// Commits returns the last count commits of the branch of the worktree at dir,
// the newest first, each named by its message: its subject and its sha.
func (f *Fake) Commits(_ context.Context, dir string, count int) ([]worktree.Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Commits", dir); err != nil {
		return nil, err
	}

	at := f.worktreeAt(realPath(dir))
	if at < 0 {
		return nil, fmt.Errorf("gittest: %s is not a worktree", dir)
	}
	commits, ok := f.historyOf(worktree.LocalBranch{Name: f.worktrees[at].Branch()})
	if !ok {
		commits = history{Root}
	}

	var log []worktree.Commit
	for i := len(commits) - 1; i >= 0 && len(log) < count; i-- {
		log = append(log, worktree.Commit{SHA: commits[i], Subject: commits[i]})
	}
	return log, nil
}

// Diffstat returns what the test seeded for the worktree at dir.
func (f *Fake) Diffstat(_ context.Context, dir string) (worktree.Diffstat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Diffstat", dir); err != nil {
		return worktree.Diffstat{}, err
	}

	return f.diffstats[realPath(dir)], nil
}

// AheadBehindOf counts the commits of the local branch span.Branch past the
// history it shares with the local branch span.Base, and the reverse; it fails
// when either is missing.
func (f *Fake) AheadBehindOf(_ context.Context, span worktree.Span) (worktree.AheadBehind, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("AheadBehindOf", span.Branch); err != nil {
		return worktree.AheadBehind{}, err
	}

	of, ofOK := f.historyOf(worktree.LocalBranch{Name: span.Branch})
	base, baseOK := f.historyOf(worktree.LocalBranch{Name: span.Base})
	if !ofOK || !baseOK {
		return worktree.AheadBehind{}, fmt.Errorf("gittest: unknown branch %s or %s", span.Branch, span.Base)
	}

	shared := sharedLength(of, base)
	return worktree.AheadBehind{Ahead: len(of) - shared, Behind: len(base) - shared}, nil
}

// IsAncestor reports whether the commit ancestry.Ancestor is in the history of
// ancestry.Descendant, as git.Repo.IsAncestor does over the local and fetched
// branches; it fails when either commit is on none of them.
func (f *Fake) IsAncestor(_ context.Context, ancestry worktree.Ancestry) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("IsAncestor", ancestry.Descendant); err != nil {
		return false, err
	}

	var hasAncestor, hasDescendant, behind bool
	for _, h := range slices.Concat(slices.Collect(maps.Values(f.local)), slices.Collect(maps.Values(f.remote))) {
		at := slices.Index(h, ancestry.Descendant)
		hasAncestor = hasAncestor || slices.Contains(h, ancestry.Ancestor)
		hasDescendant = hasDescendant || at >= 0
		behind = behind || at >= 0 && slices.Contains(h[:at+1], ancestry.Ancestor)
	}
	if !hasAncestor || !hasDescendant {
		return false, fmt.Errorf("gittest: unknown commit %s or %s", ancestry.Ancestor, ancestry.Descendant)
	}

	return behind, nil
}

// Snapshot returns the worktrees, the branches and the divergence of each
// local branch whose remote tip differs, as git.Repo.Snapshot does.
func (f *Fake) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	return git.ReadSnapshot(ctx, f)
}

// Dirty reports whether the worktree at path was seeded dirty; it fails for a
// path that is not a live worktree.
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

// ----------------------------------------------------------------------------
// Changing worktrees and branches

// AddWorktree adds a worktree on its checkout's branch, refusing as git does a
// branch that exists or is checked out, a missing start point and a taken
// path. Like git.Repo, it deletes a branch it created when the add fails.
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

// add creates the checkout's branch, if it creates one, and places the
// worktree on it.
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

// RemoveWorktree forgets the linked worktree at path, dirty, gone or locked; it fails
// for the main worktree or a path that is none.
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

// Same reports whether a and b resolve to the same worktree or folder the fake
// holds; the fake's disk tells case apart.
func (f *Fake) Same(_ context.Context, a, b string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Same", a); err != nil {
		return false, err
	}

	resolved := realPath(a)
	return resolved == realPath(b) && (f.worktreeAt(resolved) >= 0 || slices.Contains(f.folders, resolved) || slices.Contains(f.empty, resolved)), nil
}

// Unpushed counts the commits of the local branch losing.Branch, and of the
// commit losing.Head, that no other local branch but those going with it, nor
// any fetched remote branch, holds, a commit both hold once; a tip that names
// no commit holds none.
func (f *Fake) Unpushed(_ context.Context, losing worktree.Losing) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Unpushed", losing.Branch); err != nil {
		return 0, err
	}

	of := slices.Clone(f.local[losing.Branch])
	if losing.Head != "" {
		of = append(of, f.upTo(losing.Head)...)
	}

	held := map[string]bool{}
	for other, commits := range f.local {
		if other != losing.Branch && !slices.Contains(losing.Going, other) {
			for _, commit := range commits {
				held[commit] = true
			}
		}
	}
	for _, commits := range f.remote {
		for _, commit := range commits {
			held[commit] = true
		}
	}

	count := 0
	for _, commit := range of {
		if !held[commit] {
			held[commit] = true
			count++
		}
	}
	return count, nil
}

// head is the commit the worktree w stands on: its branch's tip, else Root, the
// fake committing on no branch; none for the bare entry.
func (f *Fake) head(w worktree.Worktree) string {
	if _, bare := w.State.(worktree.Bare); bare {
		return ""
	}
	if commits := f.local[w.Branch()]; len(commits) > 0 {
		return commits[len(commits)-1]
	}
	return Root
}

// upTo is the history ending on commit, as a branch holding it has it; none
// when no branch holds it.
func (f *Fake) upTo(commit string) history {
	for _, commits := range f.local {
		if at := slices.Index(commits, commit); at >= 0 {
			return commits[:at+1]
		}
	}
	for _, commits := range f.remote {
		if at := slices.Index(commits, commit); at >= 0 {
			return commits[:at+1]
		}
	}
	return nil
}

// DeleteBranch deletes the local branch name; it fails when the branch is
// missing or checked out.
func (f *Fake) DeleteBranch(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deleteBranch(name)
}

// deleteBranch is DeleteBranch with the lock already held.
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

// Fetch updates every remote branch to origin's.
func (f *Fake) Fetch(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Fetch", ""); err != nil {
		return err
	}

	maps.Copy(f.remote, f.origin)
	return nil
}

// RefreshOrigin updates the remote branch of each of branches to origin's, and drops the remote branch of each origin no longer has.
func (f *Fake) RefreshOrigin(_ context.Context, branches []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("RefreshOrigin", strings.Join(branches, " ")); err != nil {
		return err
	}

	for _, name := range branches {
		if commits, ok := f.origin[name]; ok {
			f.remote[name] = commits
			continue
		}
		delete(f.remote, name)
	}
	return nil
}

// Gone returns the tracking branches whose remote branch is no longer fetched.
func (f *Fake) Gone(context.Context) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Gone", ""); err != nil {
		return nil, err
	}

	gone := map[string]bool{}
	for _, name := range f.tracking {
		if _, fetched := f.remote[name]; !fetched {
			gone[name] = true
		}
	}
	return gone, nil
}

// FetchRef updates the remote branch name to origin's.
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

// FastForwardRef fetches origin's branch name into both the remote and the
// local branch; it fails when the branch is checked out, or when origin is not
// a fast-forward of it, after updating the remote branch as git does.
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

// MergeFFOnly fast-forwards the branch of the worktree at merge.Path onto
// merge.Onto, doing nothing when it already holds it; it fails when the
// worktree is gone or on no branch, or when Onto is not a fast-forward.
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

// ----------------------------------------------------------------------------
// Moving changes

// Moves returns every move asked of Move so far, refused ones included.
func (f *Fake) Moves() []worktree.Move {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.moves)
}

// Changes returns the changes seeded or moved into the worktree at dir.
func (f *Fake) Changes(_ context.Context, dir string) ([]worktree.Change, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Changes", dir); err != nil {
		return nil, err
	}

	return slices.Clone(f.changes[realPath(dir)]), nil
}

// Conflicts returns the seeded conflicts of check's changes, whatever the
// target.
func (f *Fake) Conflicts(_ context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Conflicts", cmp.Or(check.Target, check.TargetRef)); err != nil {
		return nil, err
	}

	return f.conflictsOf(check.Changes), nil
}

// Move records move and moves its changes from the source's list to the
// target's, refusing on the seeded conflicts as git.Repo.Move does: a binary
// one, or any the move does not resolve. Resolved conflicts leave the source
// its changes, as does a move that keeps them.
func (f *Fake) Move(_ context.Context, move worktree.Move) (worktree.Moved, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.enter("Move", move.Target); err != nil {
		return worktree.Moved{}, err
	}
	f.moves = append(f.moves, move)

	conflicts := f.conflictsOf(move.Changes)
	if refusal := move.Refusal(conflicts); refusal != nil {
		return worktree.Moved{}, refusal
	}

	target := realPath(move.Target)
	f.changes[target] = append(f.changes[target], move.Changes...)

	if len(conflicts) > 0 {
		merged := make([]string, 0, len(conflicts))
		for _, c := range conflicts {
			merged = append(merged, c.Path)
		}
		return worktree.Moved{Merged: merged, Kept: true}, nil
	}
	if move.Keep {
		return worktree.Moved{Kept: true}, nil
	}

	source := realPath(move.Source)
	f.changes[source] = slices.DeleteFunc(f.changes[source], func(c worktree.Change) bool { return slices.Contains(move.Changes, c) })
	return worktree.Moved{}, nil
}

// conflictsOf returns the seeded conflicts on the paths of changes, a rename's
// original path included.
func (f *Fake) conflictsOf(changes []worktree.Change) []worktree.Conflict {
	var conflicts []worktree.Conflict
	for _, change := range changes {
		for _, path := range []string{change.Path, change.OrigPath} {
			if kind, ok := f.conflicts[path]; ok && path != "" {
				conflicts = append(conflicts, worktree.Conflict{Path: path, Kind: kind})
			}
		}
	}

	return conflicts
}

// ----------------------------------------------------------------------------
// Helpers

// enter records a call to method with its main argument and returns the
// failure the test seeded for method, if any. The caller holds the lock.
func (f *Fake) enter(method, argument string) error {
	f.calls = append(f.calls, method+" "+argument)
	return f.fail[method]
}

// place adds a worktree at path on the branch name, unless path is taken.
func (f *Fake) place(path, name string) error {
	if f.worktreeAt(path) >= 0 || f.taken(path) {
		return fmt.Errorf("gittest: %s already exists", path)
	}

	f.worktrees = append(f.worktrees, worktree.Worktree{Path: path, State: worktree.OnBranch{Branch: name}})
	return nil
}

// listed returns the worktrees in git's order: the main one, then by path, a
// gone one prunable as git lists it unless it is locked.
func (f *Fake) listed() []worktree.Worktree {
	worktrees := slices.Clone(f.worktrees)
	for i, w := range worktrees {
		worktrees[i].Prunable = slices.Contains(f.gone, w.Path) && !w.Locked
		worktrees[i].Head = f.head(w)
	}
	slices.SortStableFunc(worktrees[1:], func(a, b worktree.Worktree) int { return strings.Compare(a.Path, b.Path) })
	return worktrees
}

// historyOf returns the history of the local or fetched remote branch ref, and
// whether it exists.
func (f *Fake) historyOf(ref worktree.Ref) (history, bool) {
	switch b := ref.(type) {
	case worktree.LocalBranch:
		commits, ok := f.local[b.Name]
		return commits, ok
	case worktree.RemoteBranch:
		commits, ok := f.remote[b.Name]
		return commits, ok
	}
	return nil, false
}

// taken reports whether a live worktree or a seeded folder lies at or under
// path.
func (f *Fake) taken(path string) bool {
	live := func(w worktree.Worktree) bool { return !slices.Contains(f.gone, w.Path) && within(w.Path, path) }
	return slices.ContainsFunc(f.worktrees, live) || slices.ContainsFunc(f.folders, func(folder string) bool { return within(folder, path) })
}

// worktreeAt returns the index of the worktree at path, or -1.
func (f *Fake) worktreeAt(path string) int {
	return slices.IndexFunc(f.worktrees, func(w worktree.Worktree) bool { return w.Path == path })
}

// checkedOut reports whether a worktree is on the branch name.
func (f *Fake) checkedOut(name string) bool {
	return slices.ContainsFunc(f.worktrees, func(w worktree.Worktree) bool { return w.Branch() == name })
}

// branchesOf returns refs as branches sorted by name, each with its tip.
func branchesOf(refs map[string]history) []worktree.Branch {
	branches := make([]worktree.Branch, 0, len(refs))
	for _, name := range slices.Sorted(maps.Keys(refs)) {
		branches = append(branches, worktree.Branch{Name: name, Tip: refs[name].tip()})
	}
	return branches
}

// historyOf returns ref's seeded history: Root, then its commits.
func historyOf(commits map[worktree.Ref][]string, ref worktree.Ref) history {
	return append(history{Root}, commits[ref]...)
}

// tip returns the last commit of h.
func (h history) tip() string { return h[len(h)-1] }

// within reports whether other starts with all of h, so that moving h to
// other is a fast-forward.
func (h history) within(other history) bool {
	return sharedLength(h, other) == len(h)
}

// sharedLength returns how many commits a and b share from Root.
func sharedLength(a, b history) int {
	shared := 0
	for shared < len(a) && shared < len(b) && a[shared] == b[shared] {
		shared++
	}
	return shared
}

// within reports whether path is root or lies under it.
func within(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// realPath resolves path's symlinks as git.RealPath does, falling back to the
// cleaned path.
func realPath(path string) string {
	resolved, err := git.RealPath(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

// realPaths resolves each of paths.
func realPaths(paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, realPath(path))
	}
	return resolved
}
