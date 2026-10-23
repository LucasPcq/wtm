package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	mainPath    = "/repo/main"
	stateDir    = "/repo/main/.git/wtm"
	branchName  = "feat/x"
	fingerprint = "0123456789abcdef01234567"
)

// useCase is what the form calls of create's use case, so a test can stand in for it.
type useCase = form.UseCase[create.Request, create.Questions, create.Outcome]

type session = form.Session[create.Request, create.Questions, create.Outcome]

type sessionParams struct {
	App     useCase
	Request create.Request
	Options domain.RunOptions
}

// The steps of create's form, in the order asked, then the recap.
const (
	stepBranches = iota
	stepFrom
	stepEnvFrom
	stepIsolation
	stepFastForward
	stepIfNotExists
	stepRecap
)

var createdAt = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type seed struct {
	Env        uicli.Env // zero runs in the main worktree
	Remembered map[string]string
	OnCreate   []project.Hook
	Linked     []string
	Behind     bool
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	Locks      error
	Open       error
	GitFails   map[string]error
	StoreFails map[string]error
	ConfigErr  error // what loading config.toml fails with, nil when it loads
}

type world struct {
	git   *gittest.Fake
	store *storetest.Fake
	seed  seed
	seen  *seen
}

// seen is what the use case was given: the directory it was opened from and the operations it locked.
type seen struct {
	dirs []string
	ops  []domain.Operation
}

func (w world) env() uicli.Env {
	env := uicli.Env{Cwd: mainPath, ProjectDir: mainPath}
	if w.seed.Env.ProjectDir != "" {
		env = w.seed.Env
	}
	return env
}

func newWorld(s seed) world {
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: filepath.Join("/elsewhere", worktree.PathSegment(name)), State: worktree.OnBranch{Branch: name}})
	}
	config := project.Config{
		Worktrees:  project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:        project.Env{Strategy: project.EnvExample},
		OnCreate:   s.OnCreate,
		Remembered: s.Remembered,
	}
	return world{
		git:   gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Remote: []string{"main"}, Worktrees: linked, Commits: commits, Fail: s.GitFails}),
		store: storetest.NewFake(storetest.FakeParams{Config: &config, Fail: s.StoreFails}),
		seed:  s,
		seen:  &seen{},
	}
}

func (w world) params(args []string) uicli.Params {
	streams := output.System()
	return uicli.Params{Version: "1.2.3", Streams: streams, Args: args, Commands: []*cobra.Command{New(streams, w.env(), w.open, formtest.CommandLine)}, KillCancelled: shell.KillCancelled, AwaitShields: shell.AwaitShields}
}

func (w world) open(_ context.Context, dir string) (create.UseCase, error) {
	w.seen.dirs = append(w.seen.dirs, dir)
	if w.seed.Open != nil {
		return create.UseCase{}, w.seed.Open
	}
	return w.usecase(), nil
}

func (w world) usecase() create.UseCase {
	runHooks := w.seed.RunHooks
	if runHooks == nil {
		runHooks = func(context.Context, env.HookChain) error { return nil }
	}
	return create.UseCase{
		Git:        w.git,
		Meta:       w.store,
		Repository: repository.Service{Config: stored{Fake: w.store, err: w.seed.ConfigErr}, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    locker{err: w.seed.Locks, seen: w.seen},
			Publisher: locker{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{
			Git:        w.git,
			Meta:       w.store,
			EnvFiles:   w.store,
			StateDir:   stateDir,
			Answers:    w.store,
			RunHooks:   runHooks,
			PublicPort: func(context.Context, int) int { return 0 },
			Shield:     shell.Shield,
			Now:        func() time.Time { return createdAt },
			LeftBehind: cleanLine,
		},
	}
}

type stored struct {
	*storetest.Fake
	err error
}

func (s stored) Load() (project.Config, project.Global, error) {
	if s.err != nil {
		return project.Config{}, project.Global{}, s.err
	}
	config, found := s.Config()
	if !found {
		return project.Config{}, project.Global{}, project.ErrConfigNotFound
	}
	return config, project.Global{}, nil
}

func (stored) LoadRun() (project.Run, error) { return project.Run{}, nil }

type locker struct {
	err  error
	seen *seen
}

func (l locker) Take(op domain.Operation) (operation.Hold, error) {
	if l.seen != nil {
		l.seen.ops = append(l.seen.ops, op)
	}
	if l.err != nil {
		return nil, l.err
	}
	return held{}, nil
}

func (locker) Open(domain.Operation) domain.Report { return func(domain.Event) {} }

// held stands in for both the locker and the event publisher: every lock is held, and nothing is published.
type held struct{}

func (held) Grow([]domain.Lock) (bool, error) { return false, nil }
func (held) Protected() bool                  { return true }
func (held) Release()                         {}

func hookFails(context.Context, env.HookChain) error {
	return errors.New("hook failed: exit status 3")
}

var meta = worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T12:00:00Z", EnvStrategy: project.EnvExample, Isolation: project.Isolated}

const (
	cwd   = "/repo/main"
	trees = "/repo/main/.trees"
)

// twoCreated is the outcome of two branches created cleanly from main, their worktrees under trees.
func twoCreated(trees string) create.Outcome {
	return create.Outcome{
		Op: "op-1",
		Plan: create.Plan{
			Branches: []create.BranchPlan{
				{Branch: "feat/a", Path: filepath.Join(trees, "feat-a"), Action: create.ActionCreate},
				{Branch: "feat/b", Path: filepath.Join(trees, "feat-b"), Action: create.ActionCreate},
			},
			From:    "main",
			EnvFrom: project.EnvExample,
			Recap:   []domain.RecapRow{recapRow("Branches", "feat/a", "feat/b"), recapRow("From", "main"), recapRow("Env", "example")},
		},
		Fingerprint: "0123456789abcdef01234567",
		Result: entered(&create.Result{Items: []create.Item{
			{Subject: "feat/a", Status: domain.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-a"), Metadata: meta}},
			{Subject: "feat/b", Status: domain.StatusDone, Detail: &create.Detail{Path: filepath.Join(trees, "feat-b"), Metadata: meta}},
		}}),
	}
}

// mixed is an outcome with a branch of every kind: checked out behind origin with its env ports, already open, failed in its hook, cancelled, and skipped.
func mixed(trees string) create.Outcome {
	hook := &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed", Message: "on_create failed", Params: domain.Params{"phase": "on_create"}, Cause: errors.New("hook failed: exit status 3")}
	return create.Outcome{
		Op: "op-2",
		Plan: create.Plan{
			Branches: []create.BranchPlan{
				{Branch: "feature", Path: filepath.Join(trees, "feature"), Action: create.ActionCheckOut},
				{Branch: "linked", Path: "/repo/main/elsewhere/linked", Action: create.ActionUnchanged},
				{Branch: "feat/c", Path: filepath.Join(trees, "feat-c"), Action: create.ActionCreate},
				{Branch: "feat/d", Path: filepath.Join(trees, "feat-d"), Action: create.ActionCreate},
				{Branch: "remote-only", Path: filepath.Join(trees, "remote-only"), Action: create.ActionResume, Upstream: "origin/remote-only"},
			},
			From:      "main",
			EnvFrom:   project.EnvMain,
			Isolation: project.Isolated,
			Recap: []domain.RecapRow{
				recapRow("Branches", "feature (existing)", "linked (already open)", "feat/c", "feat/d", "remote-only (resumes origin/remote-only)"),
				recapRow("From", "main"), recapRow("Env", "main"), recapRow("Isolation", "isolated"),
			},
		},
		Warnings: []domain.Warning{{Code: "create.ff_dirty", Message: "couldn't fast-forward main: its worktree has uncommitted changes — it starts from your local branch", Params: domain.Params{"branch": "main"}}},
		Result: entered(&create.Result{Items: []create.Item{
			{Subject: "feature", Status: domain.StatusDone, Warnings: []domain.Warning{
				{Code: "create.behind_origin", Message: "2 commit(s) behind origin — the worktree starts from your local branch.", Params: domain.Params{"behind": "2"}},
				{Code: "provision.env_ports_left_alone", Message: "Env ports left alone", Params: domain.Params{"count": "1"}, Ports: []env.Entry{{File: ".env", Key: "DB", Port: "pg", Base: 5432, Status: env.StatusMissingKey}}},
			}, Detail: &create.Detail{
				Path: filepath.Join(trees, "feature"), Metadata: meta,
				Standing: worktree.Standing{ExistingBranch: true, OriginState: "behind", OriginBehind: 2},
				EnvPorts: env.Redacted{Offset: 10, Applied: true, Entries: []env.Entry{
					{File: ".env", Key: "PORT", Port: "http", Base: 3000, Resolved: 3010, Status: env.StatusRewrite},
					{File: ".env", Key: "DB", Port: "pg", Base: 5432, Status: env.StatusMissingKey},
				}},
			}},
			{Subject: "linked", Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &create.Detail{Path: "/repo/main/elsewhere/linked", Metadata: worktree.Meta{SourceBranch: "main"}}},
			{Subject: "feat/c", Status: domain.StatusFailed, Reason: domain.ReasonIncomplete, Err: hook, Warnings: []domain.Warning{{Code: "provision.env_no_template", Message: "no template found for .env, skipping", Params: domain.Params{"target": ".env"}}, {Code: "provision.ports_not_settled_run", Message: "run.toml: line 3 — ports not settled", FollowUp: &domain.FollowUp{Command: "env", Answers: []domain.Answered{{Path: "worktree", Values: []string{"feat/c"}}}, Code: "provision.settle_ports", Message: "settle the ports once run.toml is fixed"}}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-c"), Metadata: meta}},
			{Subject: "feat/d", Status: domain.StatusCancelled, Reason: domain.ReasonIncomplete, Err: &domain.Error{Kind: domain.KindCancelled, Code: "unit.interrupted", Message: "interrupted", Params: domain.Params{"phase": "on_create"}}, Detail: &create.Detail{Path: filepath.Join(trees, "feat-d"), Metadata: meta}},
			{Subject: "remote-only", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted},
		}}),
	}
}

// entered is result ending, as an apply does, on the step into each worktree that stands.
func entered(result *create.Result) *create.Result {
	for _, item := range result.Items {
		if item.Detail != nil {
			enter := domain.Enter(item.Subject)
			enter.Code = create.CodeGo
			result.FollowUps = append(result.FollowUps, enter)
		}
	}
	return result
}

func invalid() *domain.Error {
	return domain.Invalid([]domain.FieldError{
		{Path: "branches[0]", Code: "create.checked_out_elsewhere", Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in", Params: domain.Params{"branch": "feat/x", "path": "/p"}},
		{Path: "branches[1]", Code: "distinct", Template: "{path}: {value} is given twice", Params: domain.Params{"value": "feat/x"}},
		{Path: "env_from", Code: "one_of", Template: "{path} must be one of {accepted}, not {value}", Params: domain.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "from", Code: "required", Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)", Params: domain.Params{"because": "create.parent_required"}},
	})
}

func newApp(s seed) create.UseCase {
	commits := map[worktree.Ref][]string{}
	if s.Behind {
		commits[worktree.RemoteBranch{Name: "main"}] = []string{"upstream main"}
	}
	var linked []worktree.Worktree
	for _, name := range s.Linked {
		linked = append(linked, worktree.Worktree{Path: "/elsewhere/" + worktree.PathSegment(name), State: worktree.OnBranch{Branch: name}})
	}
	var remote []string
	if s.Behind {
		remote = []string{"main"}
	}
	git := gittest.NewFake(gittest.FakeParams{Main: mainPath, Branch: "main", Local: []string{"feature"}, Remote: remote, Worktrees: linked, Commits: commits})
	config := project.Config{
		Worktrees:  project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch},
		Env:        project.Env{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env"}}},
		Remembered: s.Remembered,
	}
	store := storetest.NewFake(storetest.FakeParams{Config: &config})
	return create.UseCase{
		Git:        git,
		Meta:       store,
		Repository: repository.Service{Config: loaded{config: config}, Git: git, MainPath: mainPath},
		Operations: operation.Service{
			Locker:    held{},
			Publisher: held{},
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return createdAt },
		},
		Provision: provision.Service{
			Git:        git,
			Meta:       store,
			EnvFiles:   store,
			StateDir:   mainPath + "/.git/wtm",
			Answers:    store,
			RunHooks:   func(context.Context, env.HookChain) error { return nil },
			PublicPort: func(context.Context, int) int { return 0 },
			Shield:     shell.Shield,
			Now:        func() time.Time { return createdAt },
			LeftBehind: cleanLine,
		},
	}
}

type loaded struct {
	config project.Config
}

func (l loaded) Load() (project.Config, project.Global, error) {
	return l.config, project.Global{}, nil
}
func (loaded) LoadRun() (project.Run, error) { return project.Run{}, nil }

func (held) Take(domain.Operation) (operation.Hold, error) { return held{}, nil }
func (held) Open(domain.Operation) domain.Report           { return func(domain.Event) {} }

// refusing is a use case whose run is refused with err.
type refusing struct {
	useCase
	err error
}

func (r refusing) Run(context.Context, create.Request, domain.RunOptions) (create.Outcome, error) {
	return create.Outcome{}, r.err
}

// previewing is a use case whose every preview has a fingerprint of its own and whose applies end on errs in turn; it records the fingerprint each apply was held to.
type previewing struct {
	useCase
	previews int
	errs     []error
	held     []string
	ids      []string // the correlation ID of each run, previews included
}

func (p *previewing) Run(ctx context.Context, req create.Request, opts domain.RunOptions) (create.Outcome, error) {
	p.ids = append(p.ids, opts.CorrelationID)
	if opts.DryRun {
		p.previews++
		outcome, err := p.useCase.Run(ctx, req, opts)
		outcome.Fingerprint = fmt.Sprintf("preview-%d", p.previews)
		return outcome, err
	}

	p.held = append(p.held, opts.ExpectFingerprint)
	err := p.errs[0]
	p.errs = p.errs[1:]
	return create.Outcome{}, err
}

// enters is a terminal on which the user keeps pressing enter.
type enters struct{}

func (enters) Read(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	p[0] = '\r'
	return 1, nil
}

func cleanLine(branch string) domain.FollowUp {
	return domain.FollowUp{Command: "clean", Answers: []domain.Answered{{Path: "worktree", Values: []string{branch}}, {Path: "force", Values: []string{"true"}}}}
}

func armedUntilCancelled(ctx context.Context, _ env.HookChain) error {
	fmt.Println("armed")
	<-ctx.Done()
	return ctx.Err()
}

func armedForever(context.Context, env.HookChain) error {
	fmt.Println("armed")
	select {}
}
