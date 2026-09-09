package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func answers(values map[string]string) flow.Answers { return flow.NewAnswers(values) }

func TestDeleteRecapStatesWarningsAndTarget(t *testing.T) {
	recap := deleteRecap(deleteRecapParams{Check: domain.CleanCheckResult{
		Branch:          "feat",
		WorktreePath:    "/w/feat",
		IsDirty:         true,
		UnpushedCommits: 2,
		HasOpenPR:       true,
		PRUrl:           "http://pr",
	}})

	for _, want := range []string{"uncommitted changes", "2 commit(s)", "http://pr", "Will delete:", "/w/feat", "feat"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q:\n%s", want, recap)
		}
	}
}

func TestDeleteRecapCarriesTheReparentDecision(t *testing.T) {
	recap := deleteRecap(deleteRecapParams{
		Check:    domain.CleanCheckResult{Branch: "feat", WorktreePath: "/w/feat"},
		Reparent: "Then leave 2 child worktree(s) orphaned.",
	})
	if !strings.Contains(recap, "orphaned") {
		t.Errorf("recap should state what happens to the children:\n%s", recap)
	}
}

func TestDeleteOptionsOfferForceOnlyWhenUnsafe(t *testing.T) {
	safe := deleteOptions(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b"})
	if len(safe) != 1 || safe[0].Value != deleteYes {
		t.Fatalf("options = %+v, want the plain removal only", safe)
	}

	unsafe := deleteOptions(domain.CleanCheckResult{Branch: "b", WorktreePath: "/b", IsDirty: true})
	var forced *flow.Option
	for i := range unsafe {
		if unsafe[i].Value == deleteForce {
			forced = &unsafe[i]
		}
	}
	if forced == nil {
		t.Fatalf("options = %+v, want a force row for an unsafe worktree", unsafe)
	}
	if !forced.Danger {
		t.Error("the force row must read as destructive")
	}
}

func TestReparentProposalListsEveryMove(t *testing.T) {
	text := reparentProposal(domain.CleanReparentPlan{
		Grandparent: "gp",
		Children: []domain.ReparentResult{
			{Branch: "child", OldParent: "old", NewParent: "gp"},
			{Branch: "other", OldParent: "old", NewParent: "gp"},
		},
	})
	for _, want := range []string{"child", "other", "old", "gp"} {
		if !strings.Contains(text, want) {
			t.Errorf("proposal missing %q:\n%s", want, text)
		}
	}
}

// --force lifts the refusal without even running the check, which is what keeps a
// --yes --force run from touching the network.
func TestResolveDeleteForceSkipsTheCheck(t *testing.T) {
	f := &cleanFlow{
		request:   Request{Force: true},
		checks:    map[string]checkResult{},
		presenter: failingPresenter{t: t},
	}

	answer, err := f.resolveDelete(answers(map[string]string{KeyWorktree: "feat"}))
	if err != nil {
		t.Fatalf("resolveDelete: %v", err)
	}
	if answer.Value != deleteYes {
		t.Errorf("answer = %q, want the removal authorized", answer.Value)
	}
}

func TestResolveDeleteKeepsSafetyWithoutForce(t *testing.T) {
	f := &cleanFlow{checks: map[string]checkResult{
		"feat": {check: domain.CleanCheckResult{Branch: "feat", IsDirty: true}},
	}}

	_, err := f.resolveDelete(answers(map[string]string{KeyWorktree: "feat"}))
	if err == nil {
		t.Fatal("expected an unsafe worktree to be refused")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal %q should direct to --force", err)
	}
}

func TestResolveDeleteAllowsASafeWorktree(t *testing.T) {
	f := &cleanFlow{checks: map[string]checkResult{
		"feat": {check: domain.CleanCheckResult{Branch: "feat"}},
	}}

	answer, err := f.resolveDelete(answers(map[string]string{KeyWorktree: "feat"}))
	if err != nil {
		t.Fatalf("resolveDelete: %v", err)
	}
	if answer.Value != deleteYes {
		t.Errorf("answer = %q, want the removal authorized", answer.Value)
	}
}

// --reparent-children answers through the presets, so the recap line and the
// execution read the same answer.
func TestPresetReparentAnswersTheStep(t *testing.T) {
	if got := (&cleanFlow{request: Request{ReparentChildren: true}}).presetReparent(); got != reparentYes {
		t.Errorf("preset = %q, want the reparent authorized", got)
	}
	if got := (&cleanFlow{}).presetReparent(); got != "" {
		t.Errorf("preset = %q, want the step left to be answered", got)
	}
}

type recorder struct {
	*flowtest.Recorder
	cleaned *Outcome
}

func newRecorder() *recorder { return &recorder{Recorder: &flowtest.Recorder{}} }

func (r *recorder) Cleaned(outcome Outcome) error {
	r.cleaned = &outcome
	return nil
}

type failingPresenter struct {
	t *testing.T
}

func (p failingPresenter) Stage(flow.StageParams) error {
	p.t.Error("no progress should be shown")
	return nil
}

func (p failingPresenter) HookPhase(flow.HookPhaseParams) error {
	p.t.Error("no hooks should run")
	return nil
}

func (p failingPresenter) Notice(flow.Notice) { p.t.Error("no notice should be shown") }

func (p failingPresenter) Status(flow.Notice) { p.t.Error("no status should be shown") }

func (p failingPresenter) Cleaned(Outcome) error {
	p.t.Error("nothing should be concluded")
	return nil
}

func testContext(t *testing.T) flow.Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

// makeWorktree creates a worktree through the service, so the fixture does not
// depend on the create flow.
func makeWorktree(t *testing.T, ctx flow.Context, branchName string) string {
	t.Helper()
	result, err := worktree.Create(domain.CreateParams{
		ProjectDir:   ctx.ProjectDir,
		StateDir:     ctx.StateDir,
		Branch:       branchName,
		FromBranch:   "main",
		SourceBranch: "main",
		Config:       ctx.Config,
		SkipHooks:    true,
	})
	if err != nil {
		t.Fatalf("create %s: %v", branchName, err)
	}
	return result.Path
}

func TestRunConfirmsThenRemoves(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/gone")

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}}
	presenter := newRecorder()

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/gone", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	recap := prompter.Content[KeyDelete].Description
	for _, line := range []string{"Will delete:", "feat/gone"} {
		if !strings.Contains(recap, line) {
			t.Errorf("confirmation %q should contain %q", recap, line)
		}
	}
	if outcome.AlreadyAbsent {
		t.Error("the worktree existed, so this is a removal, not a no-op")
	}
	if presenter.cleaned == nil || presenter.cleaned.Branch != "feat/gone" {
		t.Fatalf("cleaned = %+v, want the removal reported", presenter.cleaned)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("worktree still on disk: %v", statErr)
	}
}

func TestRunPurgesTheWorktreeJobLogs(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/logged")

	logs := writeJobLog(t, ctx, "feat/logged")
	kept := writeJobLog(t, ctx, "feat/kept")

	if _, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/logged", BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(logs); !os.IsNotExist(err) {
		t.Errorf("the removed worktree kept its job logs: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("another worktree's job logs were purged: %v", err)
	}
}

func TestRunSucceedsWhenTheJobLogPurgeFails(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/logged")

	// A regular file where the logs/ directory belongs: every purge under it
	// fails with ENOTDIR, which the removal must not notice.
	if err := os.WriteFile(filepath.Join(ctx.StateDir, "logs"), []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("plant the blocking file: %v", err)
	}
	blocked := rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: ctx.StateDir, Branch: "feat/logged"})
	if err := process.PurgeWorktreeLogs(blocked); err == nil {
		t.Fatalf("the fixture does not make the purge fail, so it proves nothing")
	}

	if _, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/logged", BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("a purge that cannot happen must not fail the removal: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree still on disk: %v", err)
	}
}

// writeJobLog plants a job log for a branch's worktree and returns its directory.
func writeJobLog(t *testing.T, ctx flow.Context, branch string) string {
	t.Helper()
	dir := rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: ctx.StateDir, Branch: branch})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web.log"), []byte("listening\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return dir
}

func TestRunOffersForceOnlyWhenUnsafe(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/dirty")
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("dirty the worktree: %v", err)
	}

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteForce}}
	if _, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/dirty", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	content := prompter.Content[KeyDelete]
	if !strings.Contains(content.Description, "uncommitted changes") {
		t.Errorf("confirmation should warn about the dirty worktree:\n%s", content.Description)
	}
	var hasForce bool
	for _, option := range content.Options {
		if option.Value == deleteForce {
			hasForce = true
		}
	}
	if !hasForce {
		t.Errorf("options = %+v, want a force row for a dirty worktree", content.Options)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("the forced removal should have removed the dirty worktree")
	}
}

func TestRunOnAbsentWorktreeConcludesWithoutAsking(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{}
	presenter := newRecorder()

	outcome, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branch: "feat/ghost", BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("cleaning an absent worktree must succeed: %v", err)
	}
	if !outcome.AlreadyAbsent {
		t.Error("outcome should report the no-op")
	}
	if len(prompter.Asked) != 0 {
		t.Errorf("asked %v, want nothing asked", prompter.Asked)
	}
	if presenter.cleaned == nil || !presenter.cleaned.AlreadyAbsent {
		t.Errorf("cleaned = %+v, want the no-op reported", presenter.cleaned)
	}
}

func TestRunAbortedRemovesNothing(t *testing.T) {
	ctx := testContext(t)
	path := makeWorktree(t, ctx, "feat/keep")

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branch: "feat/keep", BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted {
		t.Error("outcome should report the abort")
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the worktree must survive a cancelled clean: %v", statErr)
	}
}

// The default drops the worktree's databases, so the recap has to say so: a
// flag must never make a line disappear from it, and silence here would have a
// reader confirm a DROP DATABASE they were never shown.
func TestDeleteRecapNamesTheDataItGivesBack(t *testing.T) {
	recap := deleteRecap(deleteRecapParams{
		Check:      domain.CleanCheckResult{Branch: "feat", WorktreePath: "/w/feat"},
		Namespaces: []string{fmt.Sprintf(domain.CleanWillDeleteNamespaceFmt, "crm_feat", "db")},
	})
	for _, want := range []string{"crm_feat", "db"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q:\n%s", want, recap)
		}
	}
}

func TestDeleteRecapSaysWhenTheDataIsKept(t *testing.T) {
	recap := deleteRecap(deleteRecapParams{
		Check:      domain.CleanCheckResult{Branch: "feat", WorktreePath: "/w/feat"},
		Namespaces: []string{domain.CleanKeepDataLine},
	})
	if !strings.Contains(recap, "--keep-data") {
		t.Errorf("recap does not say the data is kept:\n%s", recap)
	}
}

// The line is built from run.toml, so a project with no shared service adds
// nothing and the recap reads exactly as it did before.
func TestNamespaceLinesEmptyWithoutASharedService(t *testing.T) {
	flow := &cleanFlow{ctx: flow.Context{StateDir: t.TempDir()}}
	if got := flow.namespaceLines("feat"); len(got) != 0 {
		t.Errorf("lines = %v, want none", got)
	}
}
