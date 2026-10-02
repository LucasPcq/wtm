package clean

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// A locked worktree is one git refuses to remove even with --force.
func lockWorktree(t *testing.T, ctx flow.Context, path string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", ctx.ProjectDir, "worktree", "lock", path).CombinedOutput(); err != nil {
		t.Fatalf("lock %s: %v: %s", path, err, out)
	}
}

func hasOption(options []flow.Option, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func dirty(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunCleansSeveralAndKeepsGoingPastAFailure(t *testing.T) {
	ctx := testContext(t)
	first := makeWorktree(t, ctx, "feat/a")
	locked := makeWorktree(t, ctx, "feat/b")
	last := makeWorktree(t, ctx, "feat/c")
	lockWorktree(t, ctx, locked)
	presenter := newRecorder()

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", "feat/b", "feat/c"}, BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: presenter,
	})

	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want the batch reported as failed", err)
	}
	if len(outcome.Results) != 2 || len(outcome.Failed) != 1 || outcome.Failed[0].Branch != "feat/b" {
		t.Fatalf("outcome = %+v, want feat/a and feat/c removed, feat/b failed", outcome)
	}
	for _, path := range []string{first, last} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("%s still on disk: %v", path, statErr)
		}
	}
	if _, statErr := os.Stat(locked); statErr != nil {
		t.Errorf("the failed worktree must survive: %v", statErr)
	}
	if len(presenter.started) != 3 || len(presenter.done) != 2 || len(presenter.failed) != 1 {
		t.Errorf("started %d, done %d, failed %d, want 3, 2, 1", len(presenter.started), len(presenter.done), len(presenter.failed))
	}
}

func TestUnattendedRefusesTheWholeBatchWhenOneIsUnsafe(t *testing.T) {
	ctx := testContext(t)
	safe := makeWorktree(t, ctx, "feat/a")
	dirty(t, makeWorktree(t, ctx, "feat/b"))

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", "feat/b"}, BaseBranch: "main"},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})

	if err == nil || !strings.Contains(err.Error(), "feat/b") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v, want a refusal naming feat/b and --force", err)
	}
	if _, statErr := os.Stat(safe); statErr != nil {
		t.Errorf("nothing may be removed when the batch is refused: %v", statErr)
	}
}

func TestDeletingTheSafeOnesKeepsTheOthers(t *testing.T) {
	ctx := testContext(t)
	safe := makeWorktree(t, ctx, "feat/a")
	unsafe := makeWorktree(t, ctx, "feat/b")
	dirty(t, unsafe)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteSafe}}

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", "feat/b"}, BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !hasOption(prompter.Content[KeyDelete].Options, deleteSafe) {
		t.Errorf("options = %+v, want the safe-only removal offered", prompter.Content[KeyDelete].Options)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0] != (domain.PruneSkip{Branch: "feat/b", Reason: domain.PruneSkipDirty}) {
		t.Errorf("skipped = %+v, want feat/b kept as dirty", outcome.Skipped)
	}
	if _, statErr := os.Stat(safe); !os.IsNotExist(statErr) {
		t.Errorf("the safe worktree should be gone: %v", statErr)
	}
	if _, statErr := os.Stat(unsafe); statErr != nil {
		t.Errorf("the dirty worktree must survive: %v", statErr)
	}
}

// git refuses "top" beside "top/mid", hence flat names.
func TestRunReparentsAChainOntoTheNearestSurvivor(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "top")
	makeWorktreeFrom(t, ctx, "mid", "top")
	makeWorktreeFrom(t, ctx, "leaf", "mid")

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"top", "mid"}, BaseBranch: "main", ReparentChildren: true},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: newRecorder(),
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Reparented) != 1 || outcome.Reparented[0] != (domain.ReparentResult{Branch: "leaf", OldParent: "mid", NewParent: "main"}) {
		t.Errorf("reparented = %+v, want leaf moved past top onto main", outcome.Reparented)
	}
}

func TestTheParentWorktreeIsLeftOutWithAWarning(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{}
	presenter := newRecorder()

	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"main"}, BaseBranch: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})

	if err != nil {
		t.Fatalf("the parent is refused with a warning, not an error: %v", err)
	}
	if len(prompter.Asked) != 0 || presenter.cleaned == nil || len(presenter.cleaned.Results) != 0 {
		t.Errorf("asked %v, cleaned %+v, want nothing asked and an empty conclusion", prompter.Asked, presenter.cleaned)
	}
	if len(presenter.Notices) != 1 || presenter.Notices[0].Kind != flow.NoticeWarning {
		t.Errorf("notices = %+v, want one warning", presenter.Notices)
	}
}

func TestARepeatedArgumentIsAUsageError(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", " feat/a "}, BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{},
		Presenter: newRecorder(),
	})

	if !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestAnAbsentWorktreeAmongSeveralIsReportedNotFailed(t *testing.T) {
	ctx := testContext(t)
	makeWorktree(t, ctx, "feat/a")

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/ghost", "feat/a"}, BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: newRecorder(),
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Results) != 2 || !outcome.Results[0].AlreadyAbsent || outcome.Results[1].AlreadyAbsent {
		t.Errorf("results = %+v, want feat/ghost already absent and feat/a removed", outcome.Results)
	}
}

func TestDeleteOptionsOfABatch(t *testing.T) {
	safe := domain.CleanCheckResult{Branch: "a", WorktreePath: "/a"}
	unsafe := domain.CleanCheckResult{Branch: "b", WorktreePath: "/b", IsDirty: true}

	if mixed := deleteOptions([]domain.CleanCheckResult{safe, unsafe}); !hasOption(mixed, deleteSafe) || !hasOption(mixed, deleteForce) || hasOption(mixed, deleteYes) {
		t.Errorf("mixed options = %+v, want safe-only and force", mixed)
	}
	if all := deleteOptions([]domain.CleanCheckResult{unsafe, {Branch: "c", IsDirty: true}}); len(all) != 1 || all[0].Value != deleteForce {
		t.Errorf("all-unsafe options = %+v, want force alone", all)
	}
}

func TestABatchStatesEachRefusalUnderItsWorktree(t *testing.T) {
	f := &cleanFlow{
		request: Request{Branches: []string{"feat/a", "feat/b"}},
		checks: map[string]domain.CleanCheckEntry{
			"feat/a": {Check: domain.CleanCheckResult{Branch: "feat/a", WorktreePath: "/w/a", IsDirty: true}},
			"feat/b": {Check: domain.CleanCheckResult{Branch: "feat/b", WorktreePath: "/w/b", IsDirty: true}},
		},
	}

	content, err := f.deleteStep().Build(flow.NewAnswers(nil).WithValues(KeyWorktree, []string{"feat/a", "feat/b"}))
	if err != nil {
		t.Fatalf("build the delete step: %v", err)
	}

	if len(content.Blockers) != 2 || content.Blockers[0].Key == content.Blockers[1].Key {
		t.Fatalf("blockers = %+v, want one per worktree, keyed apart", content.Blockers)
	}
	for _, want := range []string{"Will delete 2 worktrees", "/w/a", "/w/b"} {
		if !strings.Contains(content.Description, want) {
			t.Errorf("recap missing %q:\n%s", want, content.Description)
		}
	}
}

func TestABatchDropsTheDataOfEveryWorktreeItRemoves(t *testing.T) {
	d := newDataFixture(t)
	makeWorktree(t, d.ctx, "feat/more")
	if err := worktree.RecordNamespaces(worktree.RecordNamespacesParams{StateDir: d.ctx.StateDir, Branch: "feat/more", Jobs: []string{"postgres"}}); err != nil {
		t.Fatal(err)
	}

	_, err := Run(Params{
		Context:   d.ctx,
		Request:   Request{Branches: []string{d.branch, "feat/more"}, BaseBranch: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
		Presenter: newRecorder(),
	})

	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	lines := strings.Split(d.dropped(t), "\n")
	if len(lines) != 2 {
		t.Fatalf("dropped %q, want both namespaces given back", d.dropped(t))
	}
	for _, line := range lines {
		if line == "present" {
			t.Errorf("a namespace was dropped while its worktree was still there: %q", d.dropped(t))
		}
	}
}

// The moves follow what was removed, not what was asked: a parent that failed
// to go is still there for its children.
func TestReparentingFollowsWhatTheRunActuallyRemoved(t *testing.T) {
	cases := map[string]struct {
		locked string
		want   []domain.ReparentResult
	}{
		"the top failed": {locked: "top", want: []domain.ReparentResult{{Branch: "leaf", OldParent: "mid", NewParent: "top"}}},
		"the mid failed": {locked: "mid", want: []domain.ReparentResult{{Branch: "mid", OldParent: "top", NewParent: "main"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := testContext(t)
			paths := map[string]string{
				"top": makeWorktree(t, ctx, "top"),
				"mid": makeWorktreeFrom(t, ctx, "mid", "top"),
			}
			makeWorktreeFrom(t, ctx, "leaf", "mid")
			lockWorktree(t, ctx, paths[c.locked])

			outcome, _ := Run(Params{
				Context:   ctx,
				Request:   Request{Branches: []string{"top", "mid"}, BaseBranch: "main", ReparentChildren: true, Force: true},
				Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyDelete: deleteYes}},
				Presenter: newRecorder(),
			})

			if len(outcome.Reparented) != len(c.want) || (len(c.want) > 0 && outcome.Reparented[0] != c.want[0]) {
				t.Errorf("reparented = %+v, want %+v", outcome.Reparented, c.want)
			}
		})
	}
}

// An agent pipes clean's stdout into a JSON parser: naming only the parent
// worktree still answers with an envelope, empty.
func TestNamingOnlyTheParentStillConcludes(t *testing.T) {
	presenter := newRecorder()

	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"main"}, BaseBranch: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if presenter.cleaned == nil || len(presenter.cleaned.Results) != 0 {
		t.Errorf("cleaned = %+v, want an empty conclusion", presenter.cleaned)
	}
}
