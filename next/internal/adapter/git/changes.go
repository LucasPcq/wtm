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
	binarySniff = 8000
	mergeFatal  = 128
)

func (r Repo) Changes(ctx context.Context, dir string) ([]worktree.Change, error) {
	out, err := r.at(dir).git(ctx, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parseChanges(out)
}

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

func (r Repo) Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error) {
	files, err := r.files(ctx, check)
	if err != nil {
		return nil, err
	}
	plan, err := r.plan(ctx, files)
	return plan.conflicts, err
}

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
	if err := target.write(ctx, writeParams{Move: move, Plan: plan, Journal: &written}); err != nil {
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

type side struct {
	data    []byte
	present bool
	mode    os.FileMode
}

func (s side) same(other side) bool {
	return s.present == other.present && bytes.Equal(s.data, other.data)
}

func (s side) binary() bool {
	return bytes.IndexByte(s.data[:min(len(s.data), binarySniff)], 0) >= 0
}

type file struct {
	path      string
	untracked bool
	base      side
	theirs    side
	ours      side
}

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

func working(dir, path string) (side, error) {
	full := filepath.Join(dir, path)
	info, err := os.Lstat(full)
	if errors.Is(err, iofs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return side{}, nil
	}
	if err != nil {
		return side{}, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return side{}, err
	}
	return side{data: data, present: true, mode: info.Mode().Perm()}, nil
}

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
	data, err := r.git(ctx, "cat-file", "blob", fields[2])
	if err != nil {
		return side{}, err
	}
	return side{data: data, present: true, mode: 0o644}, nil
}

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

type plan struct {
	tracked     []string
	untracked   []string
	patched     []string
	merged      []file
	copied      []file
	conflicting []file
	conflicts   []worktree.Conflict
}

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

type writeParams struct {
	Move    worktree.Move
	Plan    plan
	Journal *journal
}

func (r Repo) write(ctx context.Context, params writeParams) error {
	patch, err := r.at(params.Move.Source).diff(ctx, params.Plan.patched)
	if err != nil {
		return err
	}
	if len(patch) > 0 {
		if _, err := r.gitWith(ctx, patch, "apply", "--whitespace=nowarn", "--3way"); err != nil {
			return err
		}
		params.Journal.patch = patch
	}
	labels := []string{"-L", params.Move.TargetLabel, "-L", "base", "-L", "incoming (" + params.Move.SourceLabel + ")"}
	for _, f := range append(params.Plan.merged, params.Plan.conflicting...) {
		params.Journal.keep(f)
		if err := r.mergeInPlace(ctx, f, labels); err != nil {
			return fmt.Errorf("merge %s: %w", f.path, err)
		}
	}
	for _, f := range params.Plan.copied {
		params.Journal.keep(f)
		if err := put(filepath.Join(r.Dir, f.path), f.theirs); err != nil {
			return fmt.Errorf("copy %s: %w", f.path, err)
		}
	}
	return nil
}

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

func put(path string, content side) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content.data, content.mode)
}

type journal struct {
	root  string
	patch []byte
	kept  []file
}

func (j *journal) keep(f file) {
	j.kept = append(j.kept, f)
}

func (j *journal) undo(ctx context.Context, target Repo) error {
	var errs []error
	for _, f := range j.kept {
		path := filepath.Join(j.root, f.path)
		if f.ours.present {
			errs = append(errs, os.WriteFile(path, f.ours.data, f.ours.mode))
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, iofs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	if len(j.patch) > 0 {
		_, err := target.gitWith(ctx, j.patch, "apply", "--whitespace=nowarn", "--reverse", "--index")
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (r Repo) diff(ctx context.Context, paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	return r.git(ctx, append([]string{"--literal-pathspecs", "diff", "HEAD", "--binary", "--"}, paths...)...)
}

type cleanParams struct {
	Patch     []byte
	Tracked   []string
	Untracked []string
}

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

func (r Repo) at(dir string) Repo {
	return Repo{Dir: dir, Remote: r.Remote}
}

func paths(files []file) []string {
	all := make([]string, 0, len(files))
	for _, f := range files {
		all = append(all, f.path)
	}
	return all
}
