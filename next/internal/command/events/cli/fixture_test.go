package cli

import (
	"context"
	"io/fs"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

var fixedNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// A scriptedBus fails its first subscriptions with fail, then hands the next one lines, after which it ends the run with stop, as an interrupt would, unless stop is nil.
type scriptedBus struct {
	mu    sync.Mutex
	fail  []error
	lines []string
	stop  func()
}

func (b *scriptedBus) Subscribe(ctx context.Context, _ []string) (<-chan []byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.fail) > 0 {
		err := b.fail[0]
		b.fail = b.fail[1:]
		return nil, err
	}

	ch := make(chan []byte)
	go func() {
		for _, line := range b.lines {
			select {
			case ch <- []byte(line):
			case <-ctx.Done():
				return
			}
		}
		if b.stop != nil {
			b.stop()
		}
	}()
	return ch, nil
}

// registered lists repos, and registers nothing.
type registered []domain.Repo

func (registered) Register(context.Context, domain.Repo) {}

func (r registered) Prune(context.Context) ([]domain.Repo, error) {
	return r, nil
}

// config is a project's config.toml as repository.Service loads it, missing when absent.
type config struct{ absent bool }

func (c config) Load() (project.Config, project.Global, error) {
	if c.absent {
		return project.Config{}, project.Global{}, project.ErrConfigNotFound
	}
	return project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}, project.Global{}, nil
}

func (config) LoadRun() (project.Run, error) {
	return project.Run{}, nil
}

// app is the repository /code/app, its main worktree on main and feat/a beside it, numbered 1.
var app = domain.Repo{Root: "/code/app", CommonDir: "/code/app/.git"}

// opening opens app, initialized unless absent; /nowhere is no directory, any other directory in no repository.
func opening(absent bool) events.Open {
	return func(_ context.Context, dir string) (events.Source, error) {
		switch dir {
		case "/nowhere":
			return events.Source{}, fs.ErrNotExist
		case app.Root:
			git := gittest.NewFake(gittest.FakeParams{Main: app.Root, Branch: "main", Worktrees: []worktree.Worktree{{Path: "/code/.trees/feat-a", State: worktree.OnBranch{Branch: "feat/a"}}}})
			meta := storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{"feat/a": {SourceBranch: "main", CreatedAt: "2026-10-16T11:00:00Z", Ordinal: 1}}})
			return events.Source{Repo: app, Repository: repository.Service{Config: config{absent: absent}, Git: git, MainPath: app.Root, Init: &domain.FollowUp{Command: domain.CommandInit}}, Meta: meta}, nil
		}
		return events.Source{}, &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository"}
	}
}
