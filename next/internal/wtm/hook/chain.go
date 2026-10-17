// Package hook is a chain of hook commands as a worktree runs it: what it runs with, where it logs and how it fails.
package hook

import "github.com/LucasPcq/wtm/next/internal/wtm/project"

type Chain struct {
	Hooks   []project.Hook
	WorkDir string
	Env     map[string]string
	Unset   []string
	Output  func(line string)
	Log     string
}

type Failure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

func (f *Failure) Error() string { return f.Err.Error() }
func (f *Failure) Unwrap() error { return f.Err }
