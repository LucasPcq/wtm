package infra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiffFilesParams holds inputs for producing a patch of tracked changes.
type DiffFilesParams struct {
	WorktreePath string
	Files        []string
}

// DiffFiles returns a binary-safe patch of the working-tree changes (staged and
// unstaged) for the given tracked files, relative to HEAD. The patch reproduces
// the change when applied onto another worktree. Returns an empty slice when
// Files is empty.
func DiffFiles(ctx context.Context, params DiffFilesParams) ([]byte, error) {
	if len(params.Files) == 0 {
		return nil, nil
	}

	args := append([]string{"-C", params.WorktreePath, "diff", "HEAD", "--binary", "--"}, params.Files...)
	cmd := Command(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return out, nil
}

// ApplyPatchParams holds inputs for applying a patch to a worktree.
type ApplyPatchParams struct {
	WorktreePath string
	Patch        []byte
	ThreeWay     bool
	Reverse      bool
	Check        bool
}

// ApplyPatch runs `git apply` against the worktree, reading the patch from
// stdin. Check performs a dry run (no changes). ThreeWay enables the 3-way merge
// fallback. Reverse undoes the patch. A non-nil error means the patch did not
// apply cleanly.
func ApplyPatch(ctx context.Context, params ApplyPatchParams) error {
	if len(params.Patch) == 0 {
		return nil
	}

	args := []string{"-C", params.WorktreePath, "apply", "--whitespace=nowarn"}
	if params.ThreeWay {
		args = append(args, "--3way")
	}
	if params.Reverse {
		args = append(args, "--reverse")
	}
	if params.Check {
		args = append(args, "--check")
	}

	cmd := Command(ctx, "git", args...)
	cmd.Stdin = bytes.NewReader(params.Patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// MergeFileParams holds inputs for a 3-way file merge between worktrees.
type MergeFileParams struct {
	SourceWorktree string
	TargetWorktree string
	RelPath        string
	SourceBranch   string
	TargetBranch   string
}

// MergeFile performs a 3-way merge of RelPath with `git merge-file`, writing the
// result into the target worktree's copy. The merge base is the source's HEAD
// version; "ours" is the target's current content; "theirs" is the source's
// working content. On conflict it writes conflict markers in place and returns
// conflicted=true; a clean merge returns false.
func MergeFile(ctx context.Context, params MergeFileParams) (bool, error) {
	base, err := writeTempFile("wtm-merge-base-*", showHead(ctx, params.SourceWorktree, params.RelPath))
	if err != nil {
		return false, err
	}
	defer os.Remove(base)

	otherPath := filepath.Join(params.SourceWorktree, params.RelPath)
	if !FileExists(otherPath) {
		empty, tmpErr := writeTempFile("wtm-merge-other-*", nil)
		if tmpErr != nil {
			return false, tmpErr
		}
		defer os.Remove(empty)
		otherPath = empty
	}

	targetPath := filepath.Join(params.TargetWorktree, params.RelPath)
	if !FileExists(targetPath) {
		if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o755); mkErr != nil {
			return false, mkErr
		}
		if wErr := os.WriteFile(targetPath, nil, 0o644); wErr != nil {
			return false, wErr
		}
	}

	cmd := Command(ctx, "git", "merge-file",
		"-L", params.TargetBranch, "-L", "base", "-L", "incoming ("+params.SourceBranch+")",
		targetPath, base, otherPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 && exitErr.ExitCode() < 128 {
		return true, nil
	}
	return false, fmt.Errorf("git merge-file %s: %s: %w", params.RelPath, strings.TrimSpace(string(out)), err)
}

// showHead returns the HEAD version of relPath in the worktree, or nil when the
// file is not tracked at HEAD (e.g. newly added).
func showHead(ctx context.Context, worktree, relPath string) []byte {
	cmd := Command(ctx, "git", "-C", worktree, "show", "HEAD:"+relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

func writeTempFile(pattern string, content []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return f.Name(), nil
}

// ResetPathsParams holds inputs for unstaging paths.
type ResetPathsParams struct {
	WorktreePath string
	Files        []string
}

// ResetPaths unstages the given paths (`git reset -- <paths>`) so the index
// matches HEAD after the working tree has been reverted.
func ResetPaths(ctx context.Context, params ResetPathsParams) error {
	if len(params.Files) == 0 {
		return nil
	}

	args := append([]string{"-C", params.WorktreePath, "reset", "--quiet", "HEAD", "--"}, params.Files...)
	cmd := Command(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// CopyPathParams holds inputs for copying a file or directory.
type CopyPathParams struct {
	SourceDir string
	TargetDir string
	RelPath   string
}

// CopyPath copies RelPath from SourceDir to TargetDir, preserving permissions
// and creating parent directories. Directories are copied recursively, which
// covers untracked entries that git reports as a directory.
func CopyPath(params CopyPathParams) error {
	src := filepath.Join(params.SourceDir, params.RelPath)
	dst := filepath.Join(params.TargetDir, params.RelPath)

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if info.IsDir() {
		return copyTree(src, dst)
	}
	return copyFile(src, dst, info.Mode())
}

// SameContentParams holds the two paths to compare.
type SameContentParams struct {
	A string
	B string
}

// SameContent reports whether two paths are regular files with identical bytes.
// A directory on either side, or an unreadable path, is never "same": the caller
// treats that as a difference to be resolved rather than a silent no-op.
func SameContent(params SameContentParams) bool {
	a, err := os.ReadFile(params.A)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(params.B)
	if err != nil {
		return false
	}
	return bytes.Equal(a, b)
}

// IsBinaryParams holds the path to sniff.
type IsBinaryParams struct {
	Path string
}

// binarySniffLimit is how many leading bytes are scanned for a NUL byte, the
// same heuristic git uses to decide a blob is binary.
const binarySniffLimit = 8000

// IsBinary reports whether a file looks binary, i.e. holds a NUL byte in its
// first bytes. Conflict markers are meaningless in such a file, so it is never
// merged. An unreadable path is treated as binary (the conservative answer).
func IsBinary(params IsBinaryParams) bool {
	f, err := os.Open(params.Path)
	if err != nil {
		return true
	}
	defer f.Close()

	buf := make([]byte, binarySniffLimit)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	return bytes.IndexByte(buf[:n], 0) >= 0
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", dst, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
