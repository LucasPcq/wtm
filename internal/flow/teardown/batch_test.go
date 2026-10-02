package teardown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/teardown"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func repoContext(t *testing.T) flow.Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

func makeTarget(t *testing.T, ctx flow.Context, branch string) teardown.Target {
	t.Helper()
	result, err := worktree.Create(domain.CreateParams{
		ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch,
		FromBranch: "main", SourceBranch: "main", Config: ctx.Config, SkipHooks: true,
	})
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	return teardown.Target{Branch: branch, Path: result.Path}
}

func TestBatchKeepsGoingPastAFailureUnlessAskedToStop(t *testing.T) {
	for name, stop := range map[string]bool{"continue": false, "stop": true} {
		t.Run(name, func(t *testing.T) {
			globaldir.Isolate(t)
			processtest.Serve(t, nil)
			ctx := repoContext(t)
			targets := []teardown.Target{makeTarget(t, ctx, "feat/a"), makeTarget(t, ctx, "feat/b"), makeTarget(t, ctx, "feat/c")}
			gittest.JamWorktree(t, targets[1].Path)
			var started, done int

			removals := teardown.Batch(teardown.BatchParams{
				Context:       ctx,
				Presenter:     &flowtest.Recorder{},
				Targets:       targets,
				ForceRemoval:  true,
				StopOnFailure: stop,
				OnStart:       func(flow.Progress) { started++ },
				OnDone:        func(teardown.Removal) { done++ },
			})

			if removals[0].Err != nil || removals[1].Err == nil {
				t.Fatalf("removals = %+v, want feat/a removed and feat/b refused", removals)
			}
			_, statErr := os.Stat(targets[2].Path)
			if stop && (len(removals) != 2 || statErr != nil) {
				t.Errorf("stop: removals = %d, feat/c stat = %v, want feat/c untouched", len(removals), statErr)
			}
			if !stop && (len(removals) != 3 || !os.IsNotExist(statErr)) {
				t.Errorf("continue: removals = %d, feat/c stat = %v, want feat/c removed", len(removals), statErr)
			}
			if started != len(removals) || done != len(removals) {
				t.Errorf("started %d, done %d, want one each per removal attempted", started, done)
			}
		})
	}
}

func TestBatchTitlesEachHookPhaseByWorktreeOnlyWhenSeveral(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "true"}}
	targets := []teardown.Target{makeTarget(t, ctx, "feat/a"), makeTarget(t, ctx, "feat/b")}
	presenter := &flowtest.Recorder{}

	teardown.Batch(teardown.BatchParams{Context: ctx, Presenter: presenter, Targets: targets, ForceRemoval: true})

	if len(presenter.Hooks) != 2 || !strings.Contains(presenter.Hooks[1], "feat/b") {
		t.Errorf("hook titles = %v, want each naming its worktree", presenter.Hooks)
	}

	single := &flowtest.Recorder{}
	teardown.Batch(teardown.BatchParams{Context: ctx, Presenter: single, Targets: []teardown.Target{makeTarget(t, ctx, "feat/c")}, ForceRemoval: true})
	if len(single.Hooks) != 1 || single.Hooks[0] != domain.HooksTitleOnClean {
		t.Errorf("hook titles = %v, want the plain title for one worktree", single.Hooks)
	}
}

func TestBatchNamesEveryHookPhaseWhenAskedTo(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "true"}}
	presenter := &flowtest.Recorder{}

	teardown.Batch(teardown.BatchParams{
		Context:        ctx,
		Presenter:      presenter,
		Targets:        []teardown.Target{makeTarget(t, ctx, "feat/only")},
		ForceRemoval:   true,
		NameHookPhases: true,
	})

	if len(presenter.Hooks) != 1 || !strings.Contains(presenter.Hooks[0], "feat/only") {
		t.Errorf("hook titles = %v, want the phase named after its worktree", presenter.Hooks)
	}
}
