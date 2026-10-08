package teardown_test

import (
	"os"
	"path/filepath"
	"slices"
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
	result, err := worktree.Create(t.Context(), domain.CreateParams{
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

			removals := teardown.Batch(t.Context(), teardown.BatchParams{
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

	teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: presenter, Targets: targets, ForceRemoval: true})

	if len(presenter.Hooks) != 2 || !strings.Contains(presenter.Hooks[1], "feat/b") {
		t.Errorf("hook titles = %v, want each naming its worktree", presenter.Hooks)
	}

	single := &flowtest.Recorder{}
	teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: single, Targets: []teardown.Target{makeTarget(t, ctx, "feat/c")}, ForceRemoval: true})
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

	teardown.Batch(t.Context(), teardown.BatchParams{
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

func TestEveryRemovalIsPublishedWithItsLastState(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	targets := []teardown.Target{makeTarget(t, ctx, "feat/a"), makeTarget(t, ctx, "feat/b")}
	recorder := &flowtest.Recorder{}
	ctx.Publisher = recorder

	teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: targets, ForceRemoval: true})

	want := []domain.EventType{domain.EventWorktreeDeprovisioned, domain.EventWorktreeRemoved, domain.EventWorktreeDeprovisioned, domain.EventWorktreeRemoved}
	if got := recorder.PublishedTypes(); !slices.Equal(got, want) {
		t.Fatalf("published %v, want %v", got, want)
	}
	for i, event := range recorder.Published {
		if event.Worktree.Branch != targets[i/2].Branch || event.Worktree.Parent != "main" {
			t.Errorf("event %d = %+v", i, event)
		}
	}
}

func TestAHalfRemovedWorktreeIsPublishedOnce(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	target := makeTarget(t, ctx, "feat/a")
	locked := filepath.Join(target.Path, "root-owned")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "pgdata"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	recorder := &flowtest.Recorder{}
	ctx.Publisher = recorder

	removals := teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: []teardown.Target{target}, ForceRemoval: true})

	if removals[0].Err != nil {
		t.Fatalf("removal: %v", removals[0].Err)
	}
	if got := recorder.PublishedTypes(); !slices.Equal(got, deprovisionedThenRemoved) {
		t.Fatalf("published %v, want %v", got, deprovisionedThenRemoved)
	}
}

// git deletes the branch only once the worktree is gone: a branch it refuses to
// drop (unmerged) fails the removal, but the worktree no longer exists, and a
// consumer must hear it rather than keep a ghost until its next snapshot.
func TestAWorktreeGoneIsPublishedEvenWhenItsBranchStays(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	target := makeTarget(t, ctx, "feat/a")
	if err := os.WriteFile(filepath.Join(target.Path, "work.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Git(t, target.Path, "add", "work.txt")
	gittest.Git(t, target.Path, "commit", "-m", "unmerged")
	recorder := &flowtest.Recorder{}
	ctx.Publisher = recorder

	removals := teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: []teardown.Target{target}})

	if removals[0].Err == nil {
		t.Fatal("the unmerged branch was expected to fail the removal")
	}
	if worktree.StillTracked(t.Context(), worktree.FindByBranchParams{ProjectDir: ctx.ProjectDir, Branch: "feat/a"}) {
		t.Fatal("fixture: git still tracks the worktree")
	}
	if got := recorder.PublishedTypes(); !slices.Equal(got, deprovisionedThenRemoved) {
		t.Fatalf("published %v, want the removal reported", got)
	}
}

var deprovisionedThenRemoved = []domain.EventType{domain.EventWorktreeDeprovisioned, domain.EventWorktreeRemoved}

func TestARemovalWithoutHooksIsDeprovisionedThenRemoved(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	recorder := &flowtest.Recorder{}
	ctx.Publisher = recorder

	teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: []teardown.Target{makeTarget(t, ctx, "feat/a")}, ForceRemoval: true})

	if got := recorder.PublishedTypes(); !slices.Equal(got, deprovisionedThenRemoved) {
		t.Fatalf("published %v, want %v", got, deprovisionedThenRemoved)
	}
	if ok := recorder.Published[0].OK; ok == nil || !*ok {
		t.Fatalf("deprovisioned = %+v", recorder.Published[0])
	}
}

func TestAFailingOnCleanHookStopsAtDeprovisioned(t *testing.T) {
	globaldir.Isolate(t)
	processtest.Serve(t, nil)
	ctx := repoContext(t)
	ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "exit 5"}}
	target := makeTarget(t, ctx, "feat/a")
	recorder := &flowtest.Recorder{}
	ctx.Publisher = recorder

	removals := teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: []teardown.Target{target}, ForceRemoval: true})

	if removals[0].Err == nil {
		t.Fatal("want the hook's error")
	}
	if got := recorder.PublishedTypes(); !slices.Equal(got, []domain.EventType{domain.EventWorktreeDeprovisioned}) {
		t.Fatalf("published %v, want deprovisioned alone", got)
	}
	got := recorder.Published[0]
	if *got.OK || got.Hook != "exit 5" || got.ExitCode == nil || *got.ExitCode != 5 {
		t.Fatalf("deprovisioned = %+v", got)
	}
	if _, err := os.Stat(target.Path); err != nil {
		t.Fatalf("the worktree must survive: %v", err)
	}
}

// A directory deleted by hand is still a worktree git lists: without hooks
// its removal goes through; on_clean hooks cannot run there and stop it.
func TestAWorktreeWhoseDirectoryIsGone(t *testing.T) {
	for name, tc := range map[string]struct {
		hooks []domain.HookCommand
		ok    bool
		want  []domain.EventType
	}{
		"without hooks": {ok: true, want: deprovisionedThenRemoved},
		"with hooks":    {hooks: []domain.HookCommand{{Cmd: "true"}}, want: []domain.EventType{domain.EventWorktreeDeprovisioned}},
	} {
		t.Run(name, func(t *testing.T) {
			globaldir.Isolate(t)
			processtest.Serve(t, nil)
			ctx := repoContext(t)
			ctx.Config.Project.Hooks.OnClean = tc.hooks
			target := makeTarget(t, ctx, "feat/a")
			if err := os.RemoveAll(target.Path); err != nil {
				t.Fatal(err)
			}
			recorder := &flowtest.Recorder{}
			ctx.Publisher = recorder

			teardown.Batch(t.Context(), teardown.BatchParams{Context: ctx, Presenter: recorder, Targets: []teardown.Target{target}, ForceRemoval: true})

			if got := recorder.PublishedTypes(); !slices.Equal(got, tc.want) || *recorder.Published[0].OK != tc.ok {
				t.Fatalf("published %+v, want %v with ok=%v", recorder.Published, tc.want, tc.ok)
			}
		})
	}
}
