package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[exec.Request, exec.Questions, exec.Outcome]

type sessionParams struct {
	App     form.UseCase[exec.Request, exec.Questions, exec.Outcome]
	Request exec.Request
}

// The steps of exec's form, in the order asked, then the recap.
const (
	stepWorktrees = iota
	stepCommand
	stepRecap = 5
)

const (
	mainPath    = "/repo/main"
	featA       = "feat/a"
	featB       = "feat/b"
	gone        = "feat/gone"
	fingerprint = "0123456789abcdef01234567"
	// failsInB is a command the fake shell fails in feat/b alone, exiting 3.
	failsInB = "pnpm test"
)

var startedAt = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

type world struct {
	shell  exec.RunHooks // fakeShell unless a test runs its own
	git    *gittest.Fake
	store  *storetest.Fake
	config *configtest.Fake
	locks  *operationtest.Locks
}

func pathOf(branch string) string {
	return "/repo/wt-" + worktree.PathSegment(branch)
}

func newWorld() world {
	return world{
		git: gittest.NewFake(gittest.FakeParams{
			Main:   mainPath,
			Branch: "main",
			Worktrees: []worktree.Worktree{
				{Path: pathOf(featA), State: worktree.OnBranch{Branch: featA}},
				{Path: pathOf(featB), State: worktree.OnBranch{Branch: featB}},
				{Path: pathOf(gone), State: worktree.OnBranch{Branch: gone}},
			},
			Gone: []string{pathOf(gone)},
		}),
		store:  storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{featA: {SourceBranch: "main"}, featB: {SourceBranch: "main"}}}),
		config: configtest.NewFake(configtest.FakeParams{Config: &project.Config{Worktrees: project.Worktrees{BasePath: project.DefaultBasePath, BaseBranch: project.DefaultBaseBranch}}}),
		locks:  &operationtest.Locks{},
	}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: pathOf(featA), ProjectDir: mainPath}
}

func (w world) open(context.Context, string) (exec.UseCase, error) {
	return w.usecase(), nil
}

func (w world) usecase() exec.UseCase {
	shell := w.shell
	if shell == nil {
		shell = fakeShell
	}
	return exec.UseCase{
		Git:        w.git,
		Meta:       w.store,
		EnvFiles:   w.store,
		RunHooks:   shell,
		Syntax:     fakeSyntax,
		OpenLog:    fakeLog,
		StateDir:   "/repo/main/.git/wtm",
		CPUs:       8,
		Now:        func() time.Time { return startedAt },
		Repository: repository.Service{Config: w.config, Git: w.git, MainPath: mainPath, Init: &domain.FollowUp{Command: "init"}},
		Operations: operation.Service{
			Locker:    w.locks,
			Publisher: w.locks,
			NewOpID:   func() string { return "op-1" },
			Now:       func() time.Time { return startedAt },
		},
	}
}

var errExit3 = errors.New("exit status 3")

// fakeShell prints a line per command and fails failsInB in feat/b, saying so on its last lines.
func fakeShell(_ context.Context, chain env.HookChain) error {
	if chain.Hooks[0].Cmd != failsInB || chain.WorkDir != pathOf(featB) {
		chain.Output("ok")
		return nil
	}

	for _, line := range []string{"> test", "FAIL src/app.test.ts", "  expected 2, got 3"} {
		chain.Output(line)
	}
	code := 3
	return &env.HookFailure{Cmd: chain.Hooks[0].Cmd, ExitCode: &code, Err: errExit3}
}

// fakeSyntax refuses a line holding a then with no if, as /bin/sh does.
func fakeSyntax(_ context.Context, line string) error {
	if strings.Contains(line, "if then") {
		return errors.New("/bin/sh: -c: line 0: syntax error near unexpected token `then'")
	}
	return nil
}

// fakeLog opens each run's log nowhere, under the name the adapter gives it.
func fakeLog(dir, run string) (string, io.WriteCloser, error) {
	return dir + "/" + run + ".log", nopCloser{io.Discard}, nil
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
