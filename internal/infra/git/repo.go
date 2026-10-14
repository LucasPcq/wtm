// Package git is the one way the engine's layers reach git: a repository and its remote, read and written by plain calls that decide nothing.
package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/proc"
)

// Repo is a repository seen from Dir, whose remote branches live under Remote.
type Repo struct {
	Dir    string
	Remote string
}

// ReadWriter is a writer that reads what it is about to change.
type ReadWriter interface {
	Reader
	Writer
}

var _ ReadWriter = Repo{}

func (r Repo) git(ctx context.Context, args ...string) ([]byte, error) {
	return run(ctx, append([]string{"-C", r.Dir}, args...))
}

// run scrubs the repository-scoped variables a hook inherits: GIT_DIR would
// otherwise win over -C, and every call would read the repository it names.
func run(ctx context.Context, args []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := proc.Command(ctx, "git", args...)
	cmd.Env = slices.DeleteFunc(os.Environ(), repoScoped)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return nil, &Error{Args: args, Exit: -1, Stderr: strings.TrimSpace(stderr.String()), Err: ctx.Err()}
	}
	if err != nil {
		return nil, &Error{Args: args, Exit: exitCode(err), Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return stdout.Bytes(), nil
}

func repoScoped(entry string) bool {
	name, _, _ := strings.Cut(entry, "=")
	return slices.Contains(domain.GitRepoScopedEnv, name)
}

func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}
