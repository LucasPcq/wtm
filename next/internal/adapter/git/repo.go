// Package git runs git for wtm-next: the worktrees and branches of a repository, read and changed.
package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
)

type Repo struct {
	Dir    string
	Remote string
}

type Error struct {
	Args   []string
	Exit   int
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	detail := e.Stderr
	if detail == "" {
		detail = e.Err.Error()
	}
	return "git " + strings.Join(e.Args, " ") + ": " + detail
}

func (e *Error) Unwrap() error { return e.Err }

var ErrNoLocalBranch = errors.New("no local branch")

var repoScopedEnv = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_COMMON_DIR",
	"GIT_INDEX_FILE",
	"GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_NAMESPACE",
	"GIT_PREFIX",
}

func (r Repo) git(ctx context.Context, args ...string) ([]byte, error) {
	full := append([]string{"-C", r.Dir}, args...)
	var stdout, stderr bytes.Buffer
	cmd := shell.Command(ctx, "git", full...)
	cmd.Env = slices.DeleteFunc(os.Environ(), repoScoped)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return nil, &Error{Args: full, Exit: -1, Stderr: strings.TrimSpace(stderr.String()), Err: ctx.Err()}
	}
	if err != nil {
		return nil, &Error{Args: full, Exit: exitCode(err), Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.Bytes(), nil
}

func repoScoped(entry string) bool {
	name, _, _ := strings.Cut(entry, "=")
	return slices.Contains(repoScopedEnv, name)
}

func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}
