package prune

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

type recorder struct {
	*flowtest.Recorder
	pruned *Outcome
}

func (r *recorder) Pruned(outcome Outcome) error {
	r.pruned = &outcome
	return nil
}

type pruneFixture struct {
	ctx     flow.Context
	paths   map[string]string
	witness string
}

// newPruneFixture creates one worktree per branch, each holding a namespace in
// a shared postgres that is up. The remove appends what it dropped to the
// witness, so the witness reads as the order of the drops.
func newPruneFixture(t *testing.T, branches ...string) pruneFixture {
	t.Helper()
	globaldir.Isolate(t)
	repo := gittest.InitRepo(t)
	ctx := flow.Context{ProjectDir: repo, StateDir: filepath.Join(repo, ".git", "wtm")}
	ctx.Config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	ctx.Config.Project.Env.Strategy = domain.EnvStrategyExample

	paths := map[string]string{}
	for _, branch := range branches {
		if _, err := worktree.Create(domain.CreateParams{
			ProjectDir: repo, StateDir: ctx.StateDir, Branch: branch, FromBranch: "main",
			SourceBranch: "main", Config: ctx.Config, SkipHooks: true,
		}); err != nil {
			t.Fatalf("create %s: %v", branch, err)
		}
		wt, err := worktree.FindByBranch(worktree.FindByBranchParams{ProjectDir: repo, Branch: branch})
		if err != nil {
			t.Fatal(err)
		}
		paths[branch] = wt.Path
		if err := worktree.RecordNamespaces(worktree.RecordNamespacesParams{StateDir: ctx.StateDir, Branch: branch, Jobs: []string{"postgres"}}); err != nil {
			t.Fatal(err)
		}
	}

	witness := filepath.Join(t.TempDir(), "witness")
	if err := config.WriteRun(config.WriteRunParams{StateDir: ctx.StateDir, Force: true, Config: domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "postgres", Kind: domain.JobKindService, Cmd: "true", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: `echo "$WTM_NAMESPACE" >> ` + witness},
	}}}}); err != nil {
		t.Fatal(err)
	}
	processtest.Serve(t, []domain.JobInfo{{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: repo}})
	return pruneFixture{ctx: ctx, paths: paths, witness: witness}
}

func (p pruneFixture) flow(branches ...string) *pruneFlow {
	f := &pruneFlow{ctx: p.ctx, prompter: &flowtest.ScriptedPrompter{}, presenter: &recorder{Recorder: &flowtest.Recorder{}}}
	for _, branch := range branches {
		f.plan.Selected = append(f.plan.Selected, domain.PruneCandidate{Branch: branch, Path: p.paths[branch], Reason: domain.PruneReasonPRMerged})
	}
	return f
}

func (p pruneFixture) dropped() string {
	body, _ := os.ReadFile(p.witness)
	return strings.Join(strings.Fields(string(body)), " ")
}

func (p pruneFixture) refuseHookIn(t *testing.T, branch string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.paths[branch], "refuse"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// A worktree failing its teardown stops the prune there: the ones before it
// are gone with their data, it and the ones after keep theirs. Dropping every
// database first, as prune once did, cost the data of all of them.
func TestPruneStopsAtTheFirstWorktreeThatFailsAndKeepsTheRestsData(t *testing.T) {
	p := newPruneFixture(t, "feat/a", "feat/b", "feat/c")
	p.ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "test ! -f refuse"}}
	p.refuseHookIn(t, "feat/b")
	f := p.flow("feat/a", "feat/b", "feat/c")

	outcome, err := f.remove(removeParams{})

	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want the run aborted after its report", err)
	}
	if got := p.dropped(); got != "app_feat-a" {
		t.Errorf("dropped %q, want feat/a's alone", got)
	}
	if exists(p.paths["feat/a"]) || !exists(p.paths["feat/b"]) || !exists(p.paths["feat/c"]) {
		t.Errorf("a=%v b=%v c=%v, want only feat/a removed", exists(p.paths["feat/a"]), exists(p.paths["feat/b"]), exists(p.paths["feat/c"]))
	}
	result := outcome.Result
	if len(result.Pruned) != 1 || result.Failed == nil || result.Failed.Branch != "feat/b" {
		t.Errorf("result = %+v, want feat/a pruned and feat/b failed", result)
	}
	if len(result.Namespaces) != 1 || result.Namespaces[0].Branch != "feat/a" || result.Namespaces[0].Status != domain.NamespaceDropped {
		t.Errorf("namespaces = %+v, want feat/a's dropped", result.Namespaces)
	}
	presenter, _ := f.presenter.(*recorder)
	if presenter.pruned == nil || presenter.pruned.Result.Failed == nil {
		t.Error("the partial result must be reported before the run fails")
	}
}

func TestPruneRunsEachWorktreesTeardownInTurn(t *testing.T) {
	p := newPruneFixture(t, "feat/a", "feat/b")

	outcome, err := p.flow("feat/a", "feat/b").remove(removeParams{})

	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if got := p.dropped(); got != "app_feat-a app_feat-b" {
		t.Errorf("dropped %q, want both in turn", got)
	}
	if len(outcome.Result.Pruned) != 2 || len(outcome.Result.Namespaces) != 2 {
		t.Errorf("result = %+v, want both pruned with their data", outcome.Result)
	}
}

// Only the children of a worktree actually removed move onto their nearest surviving ancestor.
func TestPruneReparentsOnlyTheChildrenOfWhatItRemoved(t *testing.T) {
	p := newPruneFixture(t, "feat/a", "feat/b")
	p.ctx.Config.Project.Hooks.OnClean = []domain.HookCommand{{Cmd: "test ! -f refuse"}}
	p.refuseHookIn(t, "feat/b")
	f := p.flow("feat/a", "feat/b")
	f.plan.Reparents = []domain.ReparentResult{
		{Branch: "child-of-a", OldParent: "feat/a", NewParent: "main"},
		{Branch: "child-of-b", OldParent: "feat/b", NewParent: "main"},
	}

	outcome, _ := f.remove(removeParams{})

	if len(outcome.Result.Orphaned) != 1 || outcome.Result.Orphaned[0].Branch != "child-of-a" {
		t.Errorf("orphaned = %+v, want child-of-a alone: feat/b is still there", outcome.Result.Orphaned)
	}
}

// A re-run over worktrees already gone is not a failure.
func TestPruneToleratesAWorktreeAlreadyGone(t *testing.T) {
	p := newPruneFixture(t, "feat/a")
	gittest.Git(t, p.ctx.ProjectDir, "worktree", "remove", "--force", p.paths["feat/a"])

	outcome, err := p.flow("feat/a").remove(removeParams{})

	if err != nil || len(outcome.Result.Pruned) != 1 {
		t.Errorf("outcome = %+v, %v — want the absent worktree counted as pruned", outcome.Result, err)
	}
}

// A job that would not stop refuses that worktree's removal unless forced.
func TestPruneRefusesAWorktreeWhoseJobsWouldNotStop(t *testing.T) {
	p := newPruneFixture(t, "feat/a")
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: p.ctx.ProjectDir},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: p.paths["feat/a"]},
	})
	daemon.Survive = true

	outcome, err := p.flow("feat/a").remove(removeParams{})

	if !errors.Is(err, domain.ErrAborted) || outcome.Result.Failed == nil {
		t.Fatalf("outcome = %+v, %v — want feat/a refused", outcome.Result, err)
	}
	if !exists(p.paths["feat/a"]) || p.dropped() != "" {
		t.Error("a refused worktree must keep its directory and its data")
	}

	if _, err := p.flow("feat/a").remove(removeParams{Force: true}); err != nil {
		t.Fatalf("prune --force: %v", err)
	}
	if exists(p.paths["feat/a"]) {
		t.Error("--force must remove it anyway")
	}
}

// Releasing a pruned worktree's claim may stop the service the next worktree
// still needs to take its drop: every claim goes once all the drops are done.
func TestPruneReleasesTheClaimsAfterEveryDrop(t *testing.T) {
	p := newPruneFixture(t, "feat/a", "feat/b")
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "postgres", Status: domain.JobStatusRunning, WorkDir: p.ctx.ProjectDir}})

	if _, err := p.flow("feat/a", "feat/b").remove(removeParams{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	want := "stop_all:@" + p.paths["feat/a"] + " stop_all:@" + p.paths["feat/b"]
	if got := strings.Join(daemon.Actions(), " "); got != want {
		t.Errorf("requests = %q, want %q", got, want)
	}
}

func TestPruneDeletesTheBranchOfWhatItRemoved(t *testing.T) {
	p := newPruneFixture(t, "feat/a")

	if _, err := p.flow("feat/a").remove(removeParams{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if out, _ := exec.Command("git", "-C", p.ctx.ProjectDir, "branch", "--list", "feat/a").Output(); strings.TrimSpace(string(out)) != "" {
		t.Error("feat/a's branch survived its prune")
	}
}

// A dry run that finds nothing is still a dry run: the outcome must not read
// as a prune that ran and removed nothing.
func TestPruneDryRunWithNothingToPruneIsStillADryRun(t *testing.T) {
	p := newPruneFixture(t)
	presenter := &recorder{Recorder: &flowtest.Recorder{}}

	outcome, err := Run(Params{
		Context:   p.ctx,
		Request:   Request{Gone: true, NoFetch: true, Force: true, DryRun: true},
		Prompter:  &flowtest.ScriptedPrompter{},
		Presenter: presenter,
	})

	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Empty || !outcome.Result.DryRun {
		t.Errorf("outcome = %+v, want an empty dry run", outcome)
	}
	if presenter.pruned == nil || !presenter.pruned.Result.DryRun {
		t.Error("the presenter must be told the empty run was a dry run")
	}
}
