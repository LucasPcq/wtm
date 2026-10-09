package checkout

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	*flowtest.Recorder
	checkedOut *Outcome
}

func newRecorder() *recorder { return &recorder{Recorder: &flowtest.Recorder{}} }

func (r *recorder) CheckedOut(outcome Outcome) error {
	r.checkedOut = &outcome
	return nil
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

// testContext is a repository whose origin carries feat/thing, the branch of
// PR #42, and whose gh answers for #42, a fork #9 and the open list.
func testContext(t *testing.T) flow.Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	origin := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}
	git(t, dir, "remote", "add", "origin", origin)
	git(t, dir, "push", "-u", "origin", "main")
	git(t, dir, "branch", "feat/thing")
	git(t, dir, "push", "origin", "feat/thing")
	git(t, dir, "branch", "-D", "feat/thing")

	ghtest.Stub(t, ghtest.StubParams{
		PRs: []ghtest.PR{{Number: 42, Branch: "feat/thing", State: "open"}},
		Details: []ghtest.PRDetail{
			{Number: 42, Title: "Add the thing", Branch: "feat/thing", Base: "main"},
			{Number: 9, Title: "From a fork", Branch: "patch-1", Base: "main", Fork: true},
		},
	})

	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

func TestRunAsksWhatTheNumberLeavesOpen(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyParent: "main",
		KeyEnv:    "",
		KeyRecap:  confirmCheckout,
	}}
	presenter := newRecorder()

	outcome, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 42}, Prompter: prompter, Presenter: presenter})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := strings.Join([]string{KeyParent, KeyEnv, KeyRecap}, ",")
	if prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	if got := prompter.Content[KeyParent].Pinned; got != "main" {
		t.Errorf("parent pinned on %q, want the PR base", got)
	}
	if presenter.checkedOut == nil || presenter.checkedOut.PR.Number != 42 {
		t.Fatalf("checked out = %+v, want PR #42 reported", presenter.checkedOut)
	}
	if _, statErr := os.Stat(outcome.Result.Path); statErr != nil {
		t.Errorf("worktree not on disk: %v", statErr)
	}
	stages := strings.Join(presenter.Stages, "|")
	wantStages := strings.Join([]string{domain.CheckoutFetchingPR, domain.CheckoutFetchingBranch, "Creating worktree feat/thing…"}, "|")
	if stages != wantStages {
		t.Errorf("stages = %q, want %q", stages, wantStages)
	}
}

func TestRunPicksAmongTheLoadedPRs(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyPR:     "42",
		KeyParent: "main",
		KeyEnv:    "",
		KeyRecap:  confirmCheckout,
	}}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{Context: testContext(t), Prompter: prompter, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	options := prompter.Content[KeyPR].Options
	if len(options) != 1 || options[0].Value != "42" || options[0].Disabled {
		t.Errorf("options = %+v, want PR #42 offered", options)
	}
	if !strings.Contains(prompter.Content[KeyRecap].Description, "PR:        #42") {
		t.Errorf("recap = %q, want the picked PR named", prompter.Content[KeyRecap].Description)
	}
	if presenter.checkedOut == nil {
		t.Fatal("the picked PR was not checked out")
	}
}

// Like create, a run whose every question a flag answered still shows its recap.
func TestEverythingGivenStillShowsTheRecap(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmCheckout}}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{
		Context:   testContext(t),
		Request:   Request{Number: 42, From: "main", EnvFrom: "example"},
		Prompter:  prompter,
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prompter.AskedKeys() != KeyRecap || prompter.Confirms != 0 {
		t.Errorf("asked %q and confirmed %d times, want the recap alone", prompter.AskedKeys(), prompter.Confirms)
	}
	if presenter.checkedOut == nil {
		t.Fatal("nothing was checked out")
	}
}

// The parent fallback is a line of the recap, as in create, never a question of its own.
func TestTheParentFallbackIsARecapWarning(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	git(t, ctx.ProjectDir, "branch", "develop")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmCheckout}}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Number: 42, From: "develop", EnvFrom: "parent"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if prompter.Confirms != 0 {
		t.Errorf("confirmed %d times, want the warning in the recap only", prompter.Confirms)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.WarningPrefix+domain.EnvParentFallbackWarning) {
		t.Errorf("recap = %q, want the fallback warned", recap)
	}
}

// behindLocally leaves feat/thing checked out here one commit behind origin.
func behindLocally(t *testing.T, ctx flow.Context) {
	t.Helper()
	git(t, ctx.ProjectDir, "branch", "feat/thing", "origin/feat/thing")
	git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "server-commit")
	git(t, ctx.ProjectDir, "push", "origin", "main:feat/thing")
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestABehindBranchIsOfferedAFastForwardBeforeTheRecap(t *testing.T) {
	ctx := testContext(t)
	behindLocally(t, ctx)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyParent:       "main",
		KeyEnv:          "",
		KeySourceUpdate: decide.UpdateFastForward,
		KeyRecap:        confirmCheckout,
	}}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: prompter, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := strings.Join([]string{KeyParent, KeyEnv, KeySourceUpdate, KeyRecap}, ",")
	if prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, "fast-forward feat/thing to origin") {
		t.Errorf("recap = %q, want the fast-forward named", recap)
	}
	if local, origin := revParse(t, ctx.ProjectDir, "feat/thing"), revParse(t, ctx.ProjectDir, "origin/feat/thing"); local != origin {
		t.Errorf("feat/thing = %s, want fast-forwarded to %s", local, origin)
	}
	if presenter.checkedOut.Target.AheadBehind.Behind != 0 {
		t.Errorf("target = %+v, want the branch reported as updated", presenter.checkedOut.Target)
	}
}

func TestANewBranchIsNeverOfferedAFastForward(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyRecap: confirmCheckout}}
	if _, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 42}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, asked := prompter.Content[KeySourceUpdate]; asked {
		t.Error("a branch that does not exist here has nothing to fast-forward")
	}
}

// LUC-281: --ff only answered unattended runs, so the wizard still offered the
// fast-forward the flag had already accepted.
func TestFastForwardFlagIsNeverAsked(t *testing.T) {
	ctx := testContext(t)
	behindLocally(t, ctx)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyRecap: confirmCheckout}}

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42, FastForward: true}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := strings.Join([]string{KeyParent, KeyEnv, KeyRecap}, ","); prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q: --ff answers the source update", prompter.AskedKeys(), want)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, "fast-forward feat/thing to origin") {
		t.Errorf("recap = %q, want the fast-forward named", recap)
	}
	if local, origin := revParse(t, ctx.ProjectDir, "feat/thing"), revParse(t, ctx.ProjectDir, "origin/feat/thing"); local != origin {
		t.Errorf("feat/thing = %s, want fast-forwarded to %s", local, origin)
	}
}

func TestUnattendedFastForwardsOnlyWithTheFlag(t *testing.T) {
	for _, ff := range []bool{false, true} {
		ctx := testContext(t)
		behindLocally(t, ctx)
		before := revParse(t, ctx.ProjectDir, "feat/thing")

		if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42, FastForward: ff}, Prompter: flow.Unattended{}, Presenter: newRecorder()}); err != nil {
			t.Fatalf("Run (ff=%v): %v", ff, err)
		}
		moved := revParse(t, ctx.ProjectDir, "feat/thing") != before
		if moved != ff {
			t.Errorf("ff=%v: branch moved = %v", ff, moved)
		}
	}
}

func TestADivergedBranchIsARecapWarning(t *testing.T) {
	ctx := testContext(t)
	behindLocally(t, ctx)
	git(t, ctx.ProjectDir, "checkout", "feat/thing")
	git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "local-only")
	git(t, ctx.ProjectDir, "checkout", "main")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyParent: "main", KeyEnv: "", KeyRecap: confirmCheckout}}

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, asked := prompter.Content[KeySourceUpdate]; asked {
		t.Error("a diverged branch is not offered a fast-forward")
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, domain.WarningPrefix+domain.SourceDivergedWarning) {
		t.Errorf("recap = %q, want the divergence warned", recap)
	}
}

func TestUnattendedWithoutANumberIsRefused(t *testing.T) {
	_, err := Run(t.Context(), Params{Context: testContext(t), Prompter: flow.Unattended{}, Presenter: newRecorder()})
	if err == nil || err.Error() != domain.CheckoutPRRequired {
		t.Errorf("err = %v, want %q", err, domain.CheckoutPRRequired)
	}
}

func TestUnattendedTakesThePRBase(t *testing.T) {
	presenter := newRecorder()
	outcome, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 42}, Prompter: flow.Unattended{}, Presenter: presenter})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Result.Metadata.SourceBranch != "main" {
		t.Errorf("parent = %q, want the PR base", outcome.Result.Metadata.SourceBranch)
	}
}

func TestAForkIsRefusedBeforeAsking(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{}
	_, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 9}, Prompter: prompter, Presenter: newRecorder()})
	if err == nil || !strings.Contains(err.Error(), "is from a fork") {
		t.Errorf("err = %v, want the fork refused", err)
	}
	if prompter.AskedKeys() != "" {
		t.Errorf("asked %q before refusing", prompter.AskedKeys())
	}
}

func TestAbortChecksOutNothing(t *testing.T) {
	presenter := newRecorder()
	outcome, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 42}, Prompter: &flowtest.ScriptedPrompter{Abort: true}, Presenter: presenter})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted || presenter.checkedOut != nil {
		t.Errorf("outcome = %+v, want an abort with nothing checked out", outcome)
	}
	if len(presenter.Notices) != 1 || !presenter.Notices[0].IsAbort() {
		t.Errorf("notices = %+v, want the abort said, as every flow says it", presenter.Notices)
	}
}

func TestRunRunsHooksAsTheirOwnPhase(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "echo hooked"}}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: flow.Unattended{}, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(presenter.Hooks) != 1 || presenter.Hooks[0] != domain.HooksTitleOnCreate {
		t.Errorf("hook phases = %v, want one titled %q", presenter.Hooks, domain.HooksTitleOnCreate)
	}
}

func TestAPresenterErrorIsTheRunsError(t *testing.T) {
	presenter := &failingPresenter{recorder: newRecorder()}
	_, err := Run(t.Context(), Params{Context: testContext(t), Request: Request{Number: 42}, Prompter: flow.Unattended{}, Presenter: presenter})
	if !errors.Is(err, errWrite) {
		t.Errorf("err = %v, want the presenter's", err)
	}
}

var errWrite = errors.New("write failed")

type failingPresenter struct{ *recorder }

func (failingPresenter) CheckedOut(Outcome) error { return errWrite }

// staleBehind leaves feat/thing here level with what this clone last fetched,
// while origin has moved on: only a fetch can tell the branch is behind.
func staleBehind(t *testing.T, ctx flow.Context) {
	t.Helper()
	git(t, ctx.ProjectDir, "branch", "feat/thing", "origin/feat/thing")
	seen := revParse(t, ctx.ProjectDir, "origin/feat/thing")
	git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "server-commit")
	git(t, ctx.ProjectDir, "push", "origin", "main:feat/thing")
	git(t, ctx.ProjectDir, "update-ref", "refs/remotes/origin/feat/thing", seen)
}

func TestFFReadsOriginAsItIsNow(t *testing.T) {
	ctx := testContext(t)
	staleBehind(t, ctx)

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42, FastForward: true}, Prompter: flow.Unattended{}, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if local, origin := revParse(t, ctx.ProjectDir, "feat/thing"), revParse(t, ctx.ProjectDir, "origin/feat/thing"); local != origin {
		t.Errorf("feat/thing = %s, want fast-forwarded to origin's %s", local, origin)
	}
}

func TestAStaleRefStillOffersTheFastForward(t *testing.T) {
	ctx := testContext(t)
	staleBehind(t, ctx)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeySourceUpdate: decide.UpdateKeep, KeyRecap: confirmCheckout}}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Number: 42, From: "main", EnvFrom: "example"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, asked := prompter.Content[KeySourceUpdate]; !asked {
		t.Error("a branch behind origin must be offered its fast-forward, whatever the last fetch saw")
	}
}

func TestABranchHeldElsewhereIsRefusedBeforeAsking(t *testing.T) {
	ctx := testContext(t)
	git(t, ctx.ProjectDir, "worktree", "add", filepath.Join(t.TempDir(), "held"), "-b", "feat/thing", "origin/feat/thing")
	prompter := &flowtest.ScriptedPrompter{}

	_, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: prompter, Presenter: newRecorder()})
	if !errors.Is(err, domain.ErrWorktreeExists) {
		t.Fatalf("err = %v, want the held branch refused", err)
	}
	if prompter.AskedKeys() != "" {
		t.Errorf("asked %q before refusing", prompter.AskedKeys())
	}
}

func TestACheckoutPublishesTheWorktreeItCreates(t *testing.T) {
	ctx := testContext(t)
	presenter := newRecorder()
	ctx.Publisher = presenter.Recorder

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: flow.Unattended{}, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeProvisioned}
	if got := presenter.PublishedTypes(); !slices.Equal(got, want) || presenter.Published[0].Worktree.Branch != "feat/thing" || !*presenter.Published[1].OK {
		t.Fatalf("published %+v, want %v for feat/thing", presenter.Published, want)
	}
}

func TestAFailingOnCreateHookIsPublishedByCheckout(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "exit 6"}}
	presenter := newRecorder()
	ctx.Publisher = presenter.Recorder

	if _, err := Run(t.Context(), Params{Context: ctx, Request: Request{Number: 42}, Prompter: flow.Unattended{}, Presenter: presenter}); err == nil {
		t.Fatal("want the hook's error")
	}

	last := presenter.Published[len(presenter.Published)-1]
	if last.Type != domain.EventWorktreeProvisioned || *last.OK || last.Hook != "exit 6" || *last.ExitCode != 6 {
		t.Fatalf("provisioned = %+v", last)
	}
}
