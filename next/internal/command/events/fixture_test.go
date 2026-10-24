package events_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/registrytest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

var fixedNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// A callLog records, in order, the calls the fakes that share it receive.
type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (l *callLog) add(call string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, call)
}

func (l *callLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.calls)
}

// A fakeBus hands each subscription to the test, which feeds it lines and closes it as the bus would; the first subscriptions fail with fail, in turn. Each subscription is recorded in log, when there is one.
type fakeBus struct {
	mu            sync.Mutex
	fail          []error
	pending       []string // the lines each subscription holds as soon as it is made
	subscriptions chan subscription
	log           *callLog
}

// A subscription is one Subscribe: the repositories it asked for and the lines the test sends it.
type subscription struct {
	Repos []string
	Lines chan []byte
}

func newBus(fail ...error) *fakeBus {
	return &fakeBus{fail: fail, subscriptions: make(chan subscription, 16)}
}

func (b *fakeBus) Subscribe(_ context.Context, repos []string) (<-chan []byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.fail) > 0 {
		err := b.fail[0]
		b.fail = b.fail[1:]
		return nil, err
	}

	b.log.add("subscribe")
	lines := make(chan []byte, 16)
	for _, line := range b.pending {
		lines <- []byte(line)
	}
	b.subscriptions <- subscription{Repos: repos, Lines: lines}
	return lines, nil
}

// A known is the registry of the repositories wtm knows, every one initialized: repos are those it lists first, store what it lists once the stream runs, inert when the state is moved.
type known struct {
	repos []domain.Repo
	moved bool
	store *registrytest.Fake
}

// registers returns the repositories the stream registered.
func (k *known) registers() []domain.Repo {
	return k.store.Repos()[len(k.repos):]
}

// pruned returns how many times the stream pruned the registry.
func (k *known) pruned() int {
	return len(slices.DeleteFunc(k.store.Calls(), func(call string) bool { return call != "Prune" }))
}

// A config is a project's config.toml as repository.Service loads it, which a test can take away (Remove); each load is recorded in log, when there is one.
type config struct {
	*configtest.Fake
	log *callLog
}

// newConfig is config.toml with the trees beside the repository, each call failing as fail says.
func newConfig(fail map[string]error) *config {
	return &config{Fake: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: "../.trees", BaseBranch: "main"}}, Fail: fail})}
}

func (c *config) Load() (project.Config, project.Global, error) {
	c.log.add("observe")
	return c.Fake.Load()
}

// A repo is a repository the stream can open: where it is, its git, its records and its config.
type repo struct {
	Repo   domain.Repo
	Git    gittest.FakeParams
	Meta   map[string]worktree.Meta
	Config *config
}

// newRepo returns the repository at root, its main worktree on main and its linked worktrees on branches.
func newRepo(root string, branches ...string) repo {
	params := gittest.FakeParams{Main: root, Branch: "main"}
	for _, branch := range branches {
		params.Worktrees = append(params.Worktrees, worktree.Worktree{Path: root + "/.trees/" + branch, State: worktree.OnBranch{Branch: branch}})
	}
	return repo{Repo: domain.Repo{Root: root, CommonDir: root + "/.git"}, Git: params, Meta: map[string]worktree.Meta{}, Config: newConfig(nil)}
}

func (r repo) source() events.Source {
	git := gittest.NewFake(r.Git)
	return events.Source{Repo: r.Repo, Repository: repository.Service{Config: r.Config, Git: git, MainPath: r.Repo.Root}, Meta: storetest.NewFake(storetest.FakeParams{Meta: r.Meta})}
}

// opening opens the repositories repos by their root or any directory under it named in dirs; any other directory is in no repository, "/nowhere" no directory at all.
func opening(repos ...repo) events.Open {
	return func(_ context.Context, dir string) (events.Source, error) {
		if dir == "/nowhere" {
			return events.Source{}, fs.ErrNotExist
		}
		for _, r := range repos {
			if dir == r.Repo.Root {
				return r.source(), nil
			}
		}
		return events.Source{}, &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository"}
	}
}

// A reader collects what a stream sends, and can be made to fail.
type reader struct {
	lines    chan events.Received
	warnings chan error
	fail     error
}

func newReader() *reader {
	return &reader{lines: make(chan events.Received, 64), warnings: make(chan error, 64)}
}

func (r *reader) out() events.Out {
	return events.Out{
		Emit: func(received events.Received) error {
			r.lines <- received
			return r.fail
		},
		Warn: func(err error) { r.warnings <- err },
	}
}

var errBusGone = errors.New("connect to the event bus: no such file")
