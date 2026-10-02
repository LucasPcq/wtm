package relocate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	flowtest.Recorder
	outcome Outcome
}

func (r *recorder) Relocated(outcome Outcome) error { r.outcome = outcome; return nil }

func testContext(t *testing.T) flow.Context {
	t.Helper()
	dir, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := domain.Config{}
	config.Project.Worktrees.BasePath = "../.trees"
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: stateDir, Config: config}
}

func external(t *testing.T, ctx flow.Context, branch string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(ctx.ProjectDir), strings.ReplaceAll(branch, "/", "-")+"-manual")
	gittest.CreateBranch(t, ctx.ProjectDir, branch)
	gittest.Git(t, ctx.ProjectDir, "worktree", "add", path, branch)
	return path
}

func run(t *testing.T, ctx flow.Context, request Request, prompter flow.Prompter) (Outcome, *recorder, error) {
	t.Helper()
	if request.BaseBranch == "" {
		request.BaseBranch = "main"
	}
	presenter := &recorder{}
	outcome, err := Run(Params{Context: ctx, Request: request, Prompter: prompter, Presenter: presenter})
	return outcome, presenter, err
}

func parentOf(t *testing.T, ctx flow.Context, branch string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rules.WorktreeMetaDir(ctx.StateDir, branch), domain.MetaFileName))
	if err != nil {
		t.Fatalf("read meta.json of %s: %v", branch, err)
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	return meta.SourceBranch
}

func TestRunUnattendedMovesAndAdopts(t *testing.T) {
	ctx := testContext(t)
	from := external(t, ctx, "feat/x")

	outcome, presenter, err := run(t, ctx, Request{}, flow.Unattended{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Result.Steps) != 1 || outcome.Result.Steps[0].Status != domain.RelocateStatusMovedAdopted {
		t.Fatalf("steps = %+v", outcome.Result.Steps)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Errorf("feat/x was not moved: %v", err)
	}
	if got := parentOf(t, ctx, "feat/x"); got != "main" {
		t.Errorf("parent = %q, want main", got)
	}
	if len(presenter.Stages) != 1 || presenter.Stages[0] != domain.RelocateStageMessage {
		t.Errorf("stages = %v", presenter.Stages)
	}
}

func TestRunUnattendedWithNothingToDoIsEmpty(t *testing.T) {
	ctx := testContext(t)

	outcome, _, err := run(t, ctx, Request{}, flow.Unattended{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Empty {
		t.Fatalf("outcome = %+v, want empty", outcome)
	}
}

// --to with no worktree to move still has something to do: the config.
func TestRunToRewritesBasePathWithNoWorktreeToMove(t *testing.T) {
	ctx := testContext(t)

	outcome, _, err := run(t, ctx, Request{To: "../elsewhere"}, flow.Unattended{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Empty || !outcome.Result.BasePathUpdated || outcome.Result.BasePath != "../elsewhere" {
		t.Fatalf("outcome = %+v, want base_path rewritten", outcome)
	}
	data, err := os.ReadFile(filepath.Join(ctx.StateDir, domain.ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `base_path = "../elsewhere"`) {
		t.Errorf("config not rewritten:\n%s", data)
	}
}

func TestRunAsksForBasePathAndParentThenApplies(t *testing.T) {
	ctx := testContext(t)
	gittest.CreateBranch(t, ctx.ProjectDir, "develop")
	external(t, ctx, "feat/x")

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyBasePathGate:     basePathChange,
		KeyBasePath:         "../moved",
		KeyParent("feat/x"): "develop",
		KeyRecap:            confirmApply,
	}}
	outcome, _, err := run(t, ctx, Request{}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := strings.Join([]string{KeyBasePathGate, KeyBasePath, KeyParent("feat/x"), KeyRecap}, ",")
	if got := prompter.AskedKeys(); got != want {
		t.Errorf("asked = %q, want %q", got, want)
	}
	recap := prompter.Content[KeyRecap].Description
	for _, line := range []string{"base_path: ../.trees → ../moved", "feat/x → ../moved/feat-x (adopt, parent: develop)"} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap misses %q:\n%s", line, recap)
		}
	}
	if !outcome.Result.BasePathUpdated {
		t.Errorf("base_path not rewritten: %+v", outcome.Result)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(ctx.ProjectDir), "moved", "feat-x")); err != nil {
		t.Errorf("feat/x not under the new base_path: %v", err)
	}
	if got := parentOf(t, ctx, "feat/x"); got != "develop" {
		t.Errorf("parent = %q, want develop", got)
	}
}

// A flag never erases a recap line: --to answers the base_path steps, and the
// recap still says where the worktrees go from.
func TestRunToIsPresetAndStillRecapped(t *testing.T) {
	ctx := testContext(t)
	external(t, ctx, "feat/x")

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyParent("feat/x"): "main",
		KeyRecap:            confirmApply,
	}}
	if _, _, err := run(t, ctx, Request{To: "../moved"}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := prompter.AskedKeys(), KeyParent("feat/x")+","+KeyRecap; got != want {
		t.Errorf("asked = %q, want %q", got, want)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, "base_path: ../.trees → ../moved") {
		t.Errorf("recap lost the base_path line:\n%s", recap)
	}
}

func TestRunKeepingEverythingSkipsTheRecap(t *testing.T) {
	ctx := testContext(t)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyBasePathGate: basePathKeep}}
	outcome, _, err := run(t, ctx, Request{}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Empty || prompter.AskedKeys() != KeyBasePathGate {
		t.Fatalf("outcome = %+v, asked = %q", outcome, prompter.AskedKeys())
	}
}

func TestRunAbortedChangesNothing(t *testing.T) {
	ctx := testContext(t)
	from := external(t, ctx, "feat/x")

	outcome, presenter, err := run(t, ctx, Request{}, &flowtest.ScriptedPrompter{Abort: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Aborted || len(presenter.Notices) != 1 || !presenter.Notices[0].IsAbort() {
		t.Fatalf("outcome = %+v, notices = %+v", outcome, presenter.Notices)
	}
	if _, err := os.Stat(from); err != nil {
		t.Errorf("feat/x moved anyway: %v", err)
	}
}

func TestRunDryRunAsksAndChangesNothing(t *testing.T) {
	ctx := testContext(t)
	from := external(t, ctx, "feat/x")

	prompter := &flowtest.ScriptedPrompter{}
	outcome, _, err := run(t, ctx, Request{DryRun: true}, prompter)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prompter.AskedKeys() != "" {
		t.Errorf("a dry run asked %q", prompter.AskedKeys())
	}
	if !outcome.DryRun || len(outcome.Plan.Steps) != 1 || outcome.Result.Steps[0].Status != domain.RelocateStatusMovedAdopted {
		t.Fatalf("outcome = %+v", outcome)
	}
	if _, err := os.Stat(from); err != nil {
		t.Errorf("a dry run moved feat/x: %v", err)
	}
}
