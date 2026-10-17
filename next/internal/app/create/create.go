// Package create gives each branch asked its worktree, from one source: what the request settles on, what becomes of each branch, then each provisioned whole.
package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const CodeConfigNotFound = "create.config_not_found"

type Request struct {
	Branches    []string            `json:"branches"`
	From        string              `json:"from"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"`
	IfNotExists bool                `json:"if_not_exists"`
	Remember    []string            `json:"remember"`
	Ask         bool                `json:"-"`
}

type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
	AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
	Dirty(ctx context.Context, path string) (bool, error)
}

type Deps struct {
	Git        Git
	LoadConfig func() (project.Config, project.Global, error)
	LoadRun    func() (project.Run, error)
	Remember   func(ctx context.Context, answers map[string]string) (wtm.Written, error)
	Provision  provision.Deps
	MainPath   string
	StateDir   string
}
