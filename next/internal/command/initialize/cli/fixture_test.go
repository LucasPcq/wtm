package cli

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[initialize.Request, initialize.Questions, initialize.Outcome]

const (
	mainPath    = "/repo/main"
	configPath  = "/repo/main/.git/wtm/config.toml"
	fingerprint = "0123456789abcdef01234567"
)

var startedAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// detected is what an acme repository suggests: two .env files with their template, its install and its teardown.
var detected = project.Detected{
	EnvFiles: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}, {Target: "apps/web/.env", Template: "apps/web/.env.example"}},
	OnCreate: []project.FoundHook{{Hook: project.Hook{Cmd: "pnpm install"}, File: "pnpm-lock.yaml"}},
	OnClean:  []project.FoundHook{{Hook: project.Hook{Cmd: "docker compose down"}, File: "docker-compose.yml"}},
	Dirs:     []string{"apps", "apps/api", "apps/web"},
}

type seed struct {
	Saved     *project.Config // config.toml as written, nil for a repository not set up
	WriteFail error
}

type world struct {
	git   repo
	store *store
	locks *operationtest.Locks
}

// repo is the fake git repository, whose origin names main its default branch.
type repo struct {
	*gittest.Fake
}

func (repo) OriginHead(context.Context) (string, error) {
	return "main", nil
}

// store is config.toml in memory, nil when there is none; each write changes its digest.
type store struct {
	saved *project.Saved
	fail  error
}

func (s *store) Saved() (project.Saved, error) {
	if s.saved == nil {
		return project.Saved{}, project.ErrConfigNotFound
	}
	return *s.saved, nil
}

func (s *store) Save(_ context.Context, rewrite project.Rewrite) (domain.Written, error) {
	if s.fail != nil {
		return domain.Written{}, s.fail
	}
	s.saved = &project.Saved{Config: rewrite.Config, Digest: "written"}
	return domain.Written{Changed: true}, nil
}

// newWorld is a repository with branches main and develop and a worktree made elsewhere, set up as s.Saved says.
func newWorld(s seed) world {
	w := world{
		git: repo{gittest.NewFake(gittest.FakeParams{
			Main:      mainPath,
			Branch:    "main",
			Local:     []string{"develop"},
			Worktrees: []worktree.Worktree{{Path: "/elsewhere/feat-old", State: worktree.OnBranch{Branch: "feat/old"}}},
		})},
		store: &store{fail: s.WriteFail},
		locks: &operationtest.Locks{},
	}
	if s.Saved != nil {
		w.store.saved = &project.Saved{Config: *s.Saved, Digest: "on-disk"}
	}
	return w
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (initialize.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() initialize.UseCase {
	return initialize.UseCase{
		Git:       w.git,
		Detect:    func() project.Detected { return detected },
		RunConfig: configtest.NewFake(configtest.FakeParams{}),
		Register:  func(context.Context) {},
		Settings:  settings.Service{Store: w.store},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return startedAt },
		},
		MainPath:   mainPath,
		ConfigPath: configPath,
	}
}

// setUp is config.toml as a first init wrote it, the web's .env file since removed by hand.
func setUp() *project.Config {
	return &project.Config{
		Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"},
		Env:       project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: "apps/api/.env", Template: "apps/api/.env.example"}}},
		OnCreate:  []project.Hook{{Cmd: "pnpm install"}},
		OnClean:   []project.Hook{},
	}
}
