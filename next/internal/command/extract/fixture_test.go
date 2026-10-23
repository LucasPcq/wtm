package extract_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	source      = "feat/login"
	other       = "feat/other"
	split       = "feat/split"
	existing    = "feature"
	remote      = "remote-only"
	projectToml = "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n"
	jobsToml    = "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"npm start\"\nports = { PORT = 3000 }\n"
	remembering = projectToml + "\n[wizard.remembered]\nisolation = \"verbatim\"\nsource_update = \"ff\"\n"
	envToml     = projectToml + "\n[env]\nstrategy = \"example\"\n\n[[env.file]]\ntarget = \".env\"\n"
	mainEnv     = "FROM=main\n"
	exampleEnv  = "FROM=example\n"
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Config string
	Run    string
	Behind bool
}

type world struct {
	root      string
	main      string
	stateDir  string
	trees     string
	repo      git.Repo
	meta      store.Meta
	published *recorder
	usecase   extract.UseCase
}

func newWorld(t *testing.T, s seed) world {
	t.Helper()
	root := gittest.TempDir(t)
	main := filepath.Join(root, "main")
	commits := map[worktree.Ref][]string{
		worktree.LocalBranch{Name: existing}:  {"feature work"},
		worktree.RemoteBranch{Name: existing}: {"feature work"},
		worktree.RemoteBranch{Name: remote}:   {"remote work"},
	}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	repo := gittest.Build(t, gittest.FakeParams{
		Main:   main,
		Branch: "main",
		Local:  []string{existing},
		Remote: []string{"main", existing, remote},
		Worktrees: []worktree.Worktree{
			{Path: linkedPath(root, source), State: worktree.OnBranch{Branch: source}},
			{Path: linkedPath(root, other), State: worktree.OnBranch{Branch: other}},
		},
		Commits: commits,
	})
	stateDir := filepath.Join(main, ".git", "wtm")
	writeFile(t, filepath.Join(stateDir, "config.toml"), configOf(s))
	if s.Run != "" {
		writeFile(t, filepath.Join(stateDir, "run.toml"), s.Run)
	}
	meta := store.Meta{StateDir: stateDir}
	projectConfig := config.Project{StateDir: stateDir, GlobalPath: filepath.Join(root, "no-global.toml")}
	published := &recorder{}
	ids := 0
	return world{
		root:      root,
		main:      main,
		stateDir:  stateDir,
		trees:     filepath.Join(root, ".trees"),
		repo:      repo,
		meta:      meta,
		published: published,
		usecase: extract.UseCase{
			Git:        repo,
			Repository: repository.Service{Config: projectConfig, Git: repo, MainPath: main},
			Meta:       meta,
			Shield:     shell.Shield,
			Operations: operation.Service{
				Locker:    locks{oplock.Locks{StateDir: stateDir}},
				Publisher: published,
				NewOpID: func() string {
					ids++
					return "op-" + string(rune('0'+ids))
				},
				Now: func() time.Time { return createdAt },
			},
			Provision: provision.Service{
				Git:        repo,
				Meta:       meta,
				EnvFiles:   store.EnvFiles{},
				StateDir:   stateDir,
				Answers:    projectConfig,
				RunHooks:   shell.RunHooks,
				PublicPort: func(context.Context, int) int { return 0 },
				Shield:     shell.Shield,
				Now:        func() time.Time { return createdAt },
				LeftBehind: cleanLine,
				Settle:     settleLine,
			},
		},
	}
}

func configOf(s seed) string {
	if s.Config != "" {
		return s.Config
	}
	return projectToml
}

func linkedPath(root, name string) string {
	return filepath.Join(root, "wt-"+worktree.PathSegment(name))
}

type locks struct {
	oplock.Locks
}

func (l locks) Take(op domain.Operation) (operation.Hold, error) {
	return l.Locks.Take(op)
}

// interrupting cancels the operation as the move starts, recording what the move's context saw of it.
type interrupting struct {
	git.Repo
	cancel context.CancelFunc
	seen   *error
}

func (i interrupting) Move(ctx context.Context, move worktree.Move) (worktree.Moved, error) {
	i.cancel()
	*i.seen = ctx.Err()
	return i.Repo.Move(ctx, move)
}

// checking records the conflict checks the use case asks.
type checking struct {
	git.Repo
	checks *[]worktree.ConflictCheck
}

func (c checking) Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error) {
	*c.checks = append(*c.checks, check)
	return c.Repo.Conflicts(ctx, check)
}

// halfMoving copies the changes into the target, then fails to remove them from the source.
type halfMoving struct {
	git.Repo
}

func (halfMoving) Move(context.Context, worktree.Move) (worktree.Moved, error) {
	return worktree.Moved{}, &worktree.HalfMoved{Cause: errors.New("source busy")}
}

func cleanLine(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "clean", Answers: []domain.Answered{{Path: "worktree", Values: []string{branch}}, {Path: "force", Values: []string{"true"}}}}
}

func settleLine(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "env", Answers: []domain.Answered{{Path: "worktree", Values: []string{branch}}}}
}
