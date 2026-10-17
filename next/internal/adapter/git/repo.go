// Package git runs git for wtm-next: the worktrees and branches of a repository, read and changed.
package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
)

// Repo is a git repository, run from Dir with git's own repository discovery.
// Its methods implement the git ports of the use cases and services.
type Repo struct {
	Dir    string // a directory inside the repository: the main worktree or a linked one
	Remote string // the remote whose branches are origin's, usually "origin"
}

// Error is a git command that failed: its arguments, its exit code (-1 when it
// did not exit, cancelled or not started), what it printed on stderr and the
// error that ended it.
type Error struct {
	Args   []string
	Exit   int
	Stderr string
	Err    error
}

// Error returns the command and its stderr, or the underlying error when git
// printed nothing.
func (e *Error) Error() string {
	detail := e.Stderr
	if detail == "" {
		detail = e.Err.Error()
	}

	return "git " + strings.Join(e.Args, " ") + ": " + detail
}

// Unwrap returns the error that ended the command, so errors.Is finds a
// cancelled context or ErrNotRepository through an *Error.
func (e *Error) Unwrap() error { return e.Err }

var (
	// ErrNoLocalBranch is returned, wrapped, by AddWorktree when an existing
	// branch to check out has no local branch.
	ErrNoLocalBranch = errors.New("no local branch")
	// ErrNotRepository is found, through an *Error, when git ran outside any
	// repository.
	ErrNotRepository = errors.New("not a git repository")
)

// repoScopedEnv are the variables that pin git to one repository; they are
// dropped from git's environment so a git hook calling wtm cannot redirect it.
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

// git runs git with args in r.Dir and returns its stdout.
func (r Repo) git(ctx context.Context, args ...string) ([]byte, error) {
	return r.gitWith(ctx, nil, args...)
}

// gitWith runs git with args in r.Dir, stdin fed to it when not nil, and
// returns its stdout; a failure is an *Error, whose Err is ctx's error when ctx
// was cancelled.
func (r Repo) gitWith(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", r.Dir}, args...)

	var stdout, stderr bytes.Buffer
	cmd := shell.Command(ctx, "git", full...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Env = shell.Environ(repoScopedEnv, promptless())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return nil, &Error{Args: full, Exit: -1, Stderr: strings.TrimSpace(stderr.String()), Err: ctx.Err()}
	}
	if err != nil {
		code, exited := shell.ExitCode(err)
		if !exited {
			code = -1
		}
		if strings.Contains(stderr.String(), ErrNotRepository.Error()) {
			err = errors.Join(ErrNotRepository, err)
		}
		return nil, &Error{Args: full, Exit: code, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}

	return stdout.Bytes(), nil
}

// promptless is the environment that keeps git from asking for credentials on
// the terminal, so a fetch fails instead of hanging; a GIT_SSH_COMMAND the user
// set is kept.
func promptless() map[string]string {
	set := map[string]string{"GIT_TERMINAL_PROMPT": "0"}
	if _, own := os.LookupEnv("GIT_SSH_COMMAND"); !own {
		set["GIT_SSH_COMMAND"] = "ssh -o BatchMode=yes"
	}
	return set
}
