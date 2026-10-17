package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	binarySniff = 8000 // bytes git itself reads to tell a binary file by a NUL byte
	mergeFatal  = 128  // git merge-file exits with its count of conflicts (at most 127); a negative status, seen as 128 or more, is an error
)

// ----------------------------------------------------------------------------
// Listing changes

// Changes returns the uncommitted changes of the worktree at dir, each
// untracked file listed on its own and ignored files left out.
func (r Repo) Changes(ctx context.Context, dir string) ([]worktree.Change, error) {
	out, err := r.at(dir).git(ctx, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}

	return parseChanges(out)
}

// parseChanges reads git status --porcelain -z, where a rename or copy record
// is followed by a field holding the original path.
func parseChanges(out []byte) ([]worktree.Change, error) {
	fields := strings.Split(string(out), "\x00")

	var changes []worktree.Change
	for i := 0; i < len(fields); i++ {
		record := fields[i]
		if record == "" {
			continue
		}
		if len(record) < 4 {
			return nil, fmt.Errorf("malformed git status record %q", record)
		}

		code, change := record[:2], worktree.Change{Path: record[3:], Status: statusOf(record[:2])}
		if strings.ContainsAny(code, "RC") {
			if i+1 >= len(fields) || fields[i+1] == "" {
				return nil, fmt.Errorf("malformed git status record %q: no original path", record)
			}
			i++
			change.OrigPath = fields[i]
		}

		if code != "!!" {
			changes = append(changes, change)
		}
	}

	return changes, nil
}

// statusOf maps a two-letter porcelain status code onto a change status.
func statusOf(code string) worktree.ChangeStatus {
	switch {
	case code == "??":
		return worktree.ChangeUntracked
	case strings.ContainsAny(code, "RC"):
		return worktree.ChangeRenamed
	case strings.Contains(code, "D"):
		return worktree.ChangeDeleted
	}
	return worktree.ChangeModified
}

// ----------------------------------------------------------------------------
// Conflicts and moves

// Conflicts returns the files among check's changes that would not land
// cleanly in the target, without writing anything.
func (r Repo) Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error) {
	files, err := r.files(ctx, check)
	if err != nil {
		return nil, err
	}

	plan, err := r.plan(ctx, files)
	return plan.conflicts, err
}

// Move writes move's changes into the target worktree, then removes them from
// the source unless move keeps them or some were merged with conflict markers.
// It refuses with a *worktree.ConflictRefusal a binary conflict, or any
// conflict the move does not resolve, before writing; a failed write is undone
// in the target; a source that could not be cleaned is a *worktree.HalfMoved.
func (r Repo) Move(ctx context.Context, move worktree.Move) (worktree.Moved, error) {
	if filepath.Clean(move.Source) == filepath.Clean(move.Target) {
		return worktree.Moved{}, domain.InvalidMessage("target worktree must differ from the source")
	}

	files, err := r.files(ctx, worktree.ConflictCheck{Source: move.Source, Target: move.Target, Changes: move.Changes})
	if err != nil {
		return worktree.Moved{}, err
	}

	plan, err := r.plan(ctx, files)
	if err != nil {
		return worktree.Moved{}, err
	}
	if refusal := plan.refusal(move); refusal != nil {
		return worktree.Moved{}, refusal
	}

	source, target := r.at(move.Source), r.at(move.Target)
	whole, err := source.diff(ctx, plan.tracked)
	if err != nil {
		return worktree.Moved{}, err
	}

	written := journal{root: move.Target}
	if err := target.write(ctx, move, plan, &written); err != nil {
		return worktree.Moved{}, errors.Join(err, written.undo(ctx, target))
	}

	if len(plan.conflicts) > 0 {
		return worktree.Moved{Merged: paths(plan.conflicting), Kept: true}, nil
	}
	if move.Keep {
		return worktree.Moved{Kept: true}, nil
	}

	if err := source.clean(ctx, cleanParams{Patch: whole, Tracked: plan.tracked, Untracked: plan.untracked}); err != nil {
		return worktree.Moved{}, &worktree.HalfMoved{Cause: err}
	}

	return worktree.Moved{}, nil
}

// ----------------------------------------------------------------------------
// The three sides of a file

// A side is one version of a file: its content, whether it exists and its
// permissions.
type side struct {
	data    []byte
	present bool
	mode    os.FileMode
}

// same reports whether s and other hold the same content, or are both absent.
func (s side) same(other side) bool {
	return s.present == other.present && bytes.Equal(s.data, other.data)
}

// binary reports whether s looks binary the way git decides it.
func (s side) binary() bool {
	return bytes.IndexByte(s.data[:min(len(s.data), binarySniff)], 0) >= 0
}

// A file is one path a move touches, in its three versions: base, the source's
// HEAD; theirs, the source's working copy; ours, the target's.
type file struct {
	path      string
	untracked bool
	base      side
	theirs    side
	ours      side
}

// files reads the three sides of every path check's changes touch, a rename's
// original path included; ours comes from the target worktree, or from
// TargetRef when there is none.
func (r Repo) files(ctx context.Context, check worktree.ConflictCheck) ([]file, error) {
	source := r.at(check.Source)
	ours := func(path string) (side, error) {
		if check.Target != "" {
			return working(check.Target, path)
		}
		return r.blob(ctx, check.TargetRef, path)
	}

	var files []file
	for _, change := range check.Changes {
		untracked := change.Status == worktree.ChangeUntracked
		for _, path := range []string{change.Path, change.OrigPath} {
			if path == "" {
				continue
			}

			f := file{path: path, untracked: untracked}
			var err error
			if !untracked {
				f.base, err = source.blob(ctx, "HEAD", path)
			}
			if err == nil {
				f.theirs, err = working(check.Source, path)
			}
			if err == nil {
				f.ours, err = ours(path)
			}
			if err != nil {
				return nil, err
			}
			files = append(files, f)
		}
	}

	return files, nil
}

// working reads path in the worktree at dir; a missing file, or one under a
// path that is no longer a directory, is an absent side. Anything else but a
// regular file (a symlink, a directory, a nested repository) is refused, so a
// move never copies what a link points to nor fails halfway.
func working(dir, path string) (side, error) {
	full := filepath.Join(dir, path)
	info, err := os.Lstat(full)
	if errors.Is(err, iofs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return side{}, nil
	}
	if err != nil {
		return side{}, err
	}
	if !info.Mode().IsRegular() {
		return side{}, domain.InvalidMessage(strings.TrimSuffix(path, "/") + " is not a regular file: only files can be moved")
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return side{}, err
	}

	return side{data: data, present: true, mode: info.Mode().Perm()}, nil
}

// blob reads path in the tree of rev as the worktree would check it out, its
// smudge and end-of-line filters applied; a path that is not a file there is an
// absent side.
func (r Repo) blob(ctx context.Context, rev, path string) (side, error) {
	out, err := r.git(ctx, "--literal-pathspecs", "ls-tree", "-z", rev, "--", path)
	if err != nil {
		return side{}, err
	}

	meta, _, _ := strings.Cut(string(out), "\t")
	fields := strings.Fields(meta)
	if len(fields) < 3 || fields[1] != "blob" {
		return side{}, nil
	}

	data, err := r.git(ctx, "cat-file", "--filters", "--path="+path, fields[2])
	if err != nil {
		return side{}, err
	}

	return side{data: data, present: true, mode: 0o644}, nil
}

// ----------------------------------------------------------------------------
// Deciding what lands how

// verdict reports whether f conflicts in the target, and of which kind. A file
// the target holds as the source does, or left as at base, never conflicts; a
// text file changed on both sides conflicts only when it does not merge.
func (r Repo) verdict(ctx context.Context, f file) (worktree.ConflictKind, bool, error) {
	if f.ours.same(f.theirs) || f.ours.same(f.base) {
		return "", false, nil
	}

	if f.ours.binary() || f.theirs.binary() || f.base.binary() {
		return worktree.ConflictBinary, true, nil
	}

	kind := worktree.ConflictModified
	if f.untracked {
		kind = worktree.ConflictExists
	}
	if !f.base.present || !f.theirs.present || !f.ours.present {
		return kind, true, nil
	}

	clean, err := r.mergesCleanly(ctx, f)
	return kind, !clean, err
}

// mergesCleanly reports whether git merge-file merges f's three sides without
// a conflict, writing nothing but temporary copies.
func (r Repo) mergesCleanly(ctx context.Context, f file) (bool, error) {
	dir, err := os.MkdirTemp("", "wtm-merge-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)

	sides := []side{f.ours, f.base, f.theirs}
	names := []string{"ours", "base", "theirs"}
	args := []string{"merge-file", "-p", "--quiet"}
	for i, s := range sides {
		path := filepath.Join(dir, names[i])
		if err := os.WriteFile(path, s.data, 0o600); err != nil {
			return false, err
		}
		args = append(args, path)
	}

	_, err = r.git(ctx, args...)
	return mergeOutcome(err)
}

// mergeOutcome reads git merge-file's result: clean, conflicting (a positive
// exit below mergeFatal), or an error.
func mergeOutcome(err error) (bool, error) {
	gitErr, ok := errors.AsType[*Error](err)
	switch {
	case err == nil:
		return true, nil
	case ok && gitErr.Exit > 0 && gitErr.Exit < mergeFatal:
		return false, nil
	}
	return false, err
}

// A plan is how each file of a move lands: the tracked and untracked paths it
// takes from the source, the paths patched in, the files merged, copied, or
// merged with conflict markers, and its conflicts.
type plan struct {
	tracked     []string
	untracked   []string
	patched     []string
	merged      []file
	copied      []file
	conflicting []file
	conflicts   []worktree.Conflict
}

// plan sorts files by how each lands in the target: nothing to do when the
// target already holds it, copied when untracked, patched when the target left
// it at base, merged otherwise.
func (r Repo) plan(ctx context.Context, files []file) (plan, error) {
	var p plan
	for _, f := range files {
		if f.untracked {
			p.untracked = append(p.untracked, f.path)
		} else {
			p.tracked = append(p.tracked, f.path)
		}

		kind, conflicting, err := r.verdict(ctx, f)
		if err != nil {
			return plan{}, err
		}

		switch {
		case conflicting:
			p.conflicts = append(p.conflicts, worktree.Conflict{Path: f.path, Kind: kind})
			p.conflicting = append(p.conflicting, f)
		case f.ours.same(f.theirs):
		case f.untracked:
			p.copied = append(p.copied, f)
		case f.ours.same(f.base):
			p.patched = append(p.patched, f.path)
		default:
			p.merged = append(p.merged, f)
		}
	}

	return p, nil
}

// refusal is the *worktree.ConflictRefusal of a plan with a binary conflict,
// or with any conflict when move does not resolve them; nil otherwise.
func (p plan) refusal(move worktree.Move) error {
	binary := false
	for _, conflict := range p.conflicts {
		binary = binary || conflict.Kind == worktree.ConflictBinary
	}

	if binary || (len(p.conflicts) > 0 && !move.Resolve) {
		return &worktree.ConflictRefusal{Conflicts: p.conflicts}
	}
	return nil
}

// ----------------------------------------------------------------------------
// Writing the target, cleaning the source

// write lands plan p in this worktree, the target: the patch of the files it
// left at base, then the merges, then the copies, journaling each so a failure
// can be undone.
func (r Repo) write(ctx context.Context, move worktree.Move, p plan, written *journal) error {
	patch, err := r.at(move.Source).diff(ctx, p.patched)
	if err != nil {
		return err
	}

	if len(patch) > 0 {
		if _, err := r.gitWith(ctx, patch, "apply", "--whitespace=nowarn", "--3way"); err != nil {
			return err
		}
		written.patch = patch
	}

	labels := []string{"-L", move.TargetLabel, "-L", "base", "-L", "incoming (" + move.SourceLabel + ")"}
	for _, f := range append(p.merged, p.conflicting...) {
		written.keep(f)
		if err := r.mergeInPlace(ctx, f, labels); err != nil {
			return fmt.Errorf("merge %s: %w", f.path, err)
		}
	}

	for _, f := range p.copied {
		written.keep(f)
		if err := put(filepath.Join(r.Dir, f.path), f.theirs); err != nil {
			return fmt.Errorf("copy %s: %w", f.path, err)
		}
	}

	return nil
}

// mergeInPlace merges f's source changes into its file in this worktree,
// leaving conflict markers labelled by labels where they conflict; a missing
// file is merged as empty.
func (r Repo) mergeInPlace(ctx context.Context, f file, labels []string) error {
	target := filepath.Join(r.Dir, f.path)
	if !f.ours.present {
		if err := put(target, side{present: true, mode: 0o644}); err != nil {
			return err
		}
	}

	dir, err := os.MkdirTemp("", "wtm-merge-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	base, theirs := filepath.Join(dir, "base"), filepath.Join(dir, "theirs")
	if err := os.WriteFile(base, f.base.data, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(theirs, f.theirs.data, 0o600); err != nil {
		return err
	}

	_, err = r.git(ctx, append(append([]string{"merge-file"}, labels...), target, base, theirs)...)
	_, err = mergeOutcome(err)
	return err
}

// put writes content at path, creating its directories.
func put(path string, content side) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content.data, content.mode)
}

// A journal is what a write changed in the target worktree at root: the patch
// it applied and the files it merged or copied, with their content before.
type journal struct {
	root  string
	patch []byte
	kept  []file
}

// keep records that f is about to be written.
func (j *journal) keep(f file) {
	j.kept = append(j.kept, f)
}

// undo puts the target back as the journal found it: each file restored or
// removed, the patch reversed.
func (j *journal) undo(ctx context.Context, target Repo) error {
	var errs []error
	for _, f := range j.kept {
		path := filepath.Join(j.root, f.path)
		if f.ours.present {
			errs = append(errs, os.WriteFile(path, f.ours.data, f.ours.mode))
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, iofs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			errs = append(errs, err)
		}
	}

	if len(j.patch) > 0 {
		_, err := target.gitWith(ctx, j.patch, "apply", "--whitespace=nowarn", "--reverse", "--index")
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// diff returns the binary diff of paths against HEAD in this worktree, nil for
// no paths, in the shape git apply reads whatever the user's diff config.
func (r Repo) diff(ctx context.Context, paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	return r.git(ctx, append([]string{"--literal-pathspecs", "diff", "HEAD", "--binary", "--no-ext-diff", "--no-textconv", "--no-color", "--src-prefix=a/", "--dst-prefix=b/", "--"}, paths...)...)
}

// cleanParams is what a move takes out of its source: the patch of its tracked
// changes, those paths to unstage, and the untracked files to delete.
type cleanParams struct {
	Patch     []byte
	Tracked   []string
	Untracked []string
}

// clean removes the moved changes from this worktree, the source, and the
// directories its deleted untracked files leave empty.
func (r Repo) clean(ctx context.Context, params cleanParams) error {
	if len(params.Patch) > 0 {
		if _, err := r.gitWith(ctx, params.Patch, "apply", "--whitespace=nowarn", "--reverse"); err != nil {
			return err
		}
	}

	if len(params.Tracked) > 0 {
		if _, err := r.git(ctx, append([]string{"--literal-pathspecs", "reset", "-q", "HEAD", "--"}, params.Tracked...)...); err != nil {
			return err
		}
	}

	for _, path := range params.Untracked {
		full := filepath.Join(r.Dir, path)
		if err := os.Remove(full); err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return err
		}

		for dir := filepath.Dir(full); dir != r.Dir && strings.HasPrefix(dir, r.Dir+string(filepath.Separator)); dir = filepath.Dir(dir) {
			if os.Remove(dir) != nil {
				break
			}
		}
	}

	return nil
}

// at is the same repository run from the worktree at dir.
func (r Repo) at(dir string) Repo {
	return Repo{Dir: dir, Remote: r.Remote}
}

// paths returns the path of each of files.
func paths(files []file) []string {
	all := make([]string, 0, len(files))
	for _, f := range files {
		all = append(all, f.path)
	}
	return all
}
