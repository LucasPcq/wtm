package exec

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	flowtest.Recorder
	progress []ExecProgress
	executed *Outcome
}

func (r *recorder) Progress(p ExecProgress)  { r.progress = append(r.progress, p) }
func (r *recorder) Executed(o Outcome) error { r.executed = &o; return nil }

func run(t *testing.T, fx fixture, request Request, prompter flow.Prompter) (Outcome, *recorder, error) {
	t.Helper()
	rec := &recorder{}
	outcome, err := Run(Params{
		Ctx:       context.Background(),
		Context:   flow.Context{ProjectDir: fx.dir, StateDir: fx.stateDir},
		Request:   request,
		Prompter:  prompter,
		Presenter: rec,
	})
	return outcome, rec, err
}

func TestNamedWorktreesRunUnattendedInTheOrderTyped(t *testing.T) {
	fx := newFixture(t, "a", "b")
	outcome, rec, err := run(t, fx, Request{Branches: []string{"b", "a"}, Command: "true", Jobs: 2}, flow.Unattended{})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Results) != 2 || outcome.Results[0].Branch != "b" || outcome.Results[1].Branch != "a" {
		t.Fatalf("results = %+v", outcome.Results)
	}
	if rec.executed == nil || len(rec.progress) != 4 {
		t.Fatalf("executed = %v, beats = %d", rec.executed, len(rec.progress))
	}
}

func TestAllRunsEveryCandidateIncludingMain(t *testing.T) {
	fx := newFixture(t, "a")
	outcome, _, err := run(t, fx, Request{All: true, Command: "true", Jobs: 2}, flow.Unattended{})
	if err != nil || len(outcome.Results) != 2 {
		t.Fatalf("results = %+v, %v", outcome.Results, err)
	}
}

func TestUnattendedWithoutSelectionNamesAll(t *testing.T) {
	fx := newFixture(t, "a")
	_, _, err := run(t, fx, Request{Command: "true", Jobs: 1}, flow.Unattended{})
	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnknownWorktreeIsRefusedBeforeAnythingRuns(t *testing.T) {
	fx := newFixture(t, "a")
	_, rec, err := run(t, fx, Request{Branches: []string{"a", "nope"}, Command: "true", Jobs: 1}, flow.Unattended{})
	if !errors.Is(err, domain.ErrExecUnknownWorktree) || len(rec.progress) != 0 {
		t.Fatalf("err = %v, beats = %d", err, len(rec.progress))
	}
}

func TestAFailureIsReportedThenAborts(t *testing.T) {
	fx := newFixture(t, "a", "b")
	outcome, rec, err := run(t, fx, Request{Branches: []string{"a", "b"}, Command: "exit 4", Jobs: 2}, flow.Unattended{})
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v", err)
	}
	if rec.executed == nil || outcome.Results[0].Status != domain.ExecStatusFailed {
		t.Fatalf("the report must be presented before aborting: %+v", outcome)
	}
}

func TestPickerPrechecksTheCurrentWorktreeAndRecapReadsBackPresets(t *testing.T) {
	fx := newFixture(t, "a", "b")
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"b"}},
		Answers: map[string]string{KeyConfirm: domain.ExecConfirmValue},
	}
	_, _, err := run(t, fx, Request{Command: "true", Jobs: 1, Dir: fx.worktreePath("a") + "/sub"}, prompter)
	if err != nil {
		t.Fatal(err)
	}
	var prechecked []string
	for _, option := range prompter.Content[KeySelection].Options {
		if option.Selected {
			prechecked = append(prechecked, option.Value)
		}
	}
	if strings.Join(prechecked, ",") != "a" {
		t.Errorf("prechecked = %v, want the worktree the dir sits in", prechecked)
	}
	recap := prompter.Content[KeyConfirm].Description
	for _, want := range []string{"b", "true", "1"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap %q misses %q", recap, want)
		}
	}
}

func TestPresetSelectionStillShowsTheRecap(t *testing.T) {
	fx := newFixture(t, "a")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyConfirm: domain.ExecConfirmValue}}
	_, _, err := run(t, fx, Request{Branches: []string{"a"}, Command: "true", Jobs: 1}, prompter)
	if err != nil || prompter.AskedKeys() != KeyConfirm {
		t.Fatalf("asked = %q, err = %v", prompter.AskedKeys(), err)
	}
}

func TestAbortRunsNothing(t *testing.T) {
	fx := newFixture(t, "a")
	outcome, rec, err := run(t, fx, Request{Branches: []string{"a"}, Command: "true", Jobs: 1}, &flowtest.ScriptedPrompter{Abort: true})
	if err != nil || !outcome.Aborted || len(rec.progress) != 0 {
		t.Fatalf("outcome = %+v, err = %v", outcome, err)
	}
}

type fixture struct {
	dir      string
	stateDir string
	root     string
}

// newFixture spells every path the way the shell resolves it: on macOS the
// temp dir is /var, which git and $PWD report as /private/var.
func newFixture(t *testing.T, branches ...string) fixture {
	t.Helper()
	dir := gittest.InitRepo(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, branch := range branches {
		gittest.Git(t, dir, "worktree", "add", "-b", branch, filepath.Join(root, branch), "main")
	}
	return fixture{dir: dir, stateDir: filepath.Join(dir, ".git", "wtm"), root: root}
}

func (f fixture) worktreePath(branch string) string { return filepath.Join(f.root, branch) }

// The fixture above spells every path canonically; this one keeps the temp
// dir as the OS hands it out (/var on macOS, /private/var to git and $PWD).
func TestTheCurrentWorktreeIsFoundThroughASymlinkedPath(t *testing.T) {
	dir := gittest.InitRepo(t)
	raw := t.TempDir()
	gittest.Git(t, dir, "worktree", "add", "-b", "a", filepath.Join(raw, "a"), "main")
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"a"}},
		Answers: map[string]string{KeyConfirm: domain.ExecConfirmValue},
	}
	_, err := Run(Params{
		Ctx:       context.Background(),
		Context:   flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm")},
		Request:   Request{Command: "true", Jobs: 1, Dir: filepath.Join(raw, "a")},
		Prompter:  prompter,
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range prompter.Content[KeySelection].Options {
		if option.Value == "a" && !option.Selected {
			t.Fatalf("a is where the user stands, through %s", raw)
		}
	}
}

func TestWithoutACommandTheWizardAsksForItBetweenWorktreesAndConfirm(t *testing.T) {
	fx := newFixture(t, "a")
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"a"}},
		Answers: map[string]string{KeyCommand: "echo hi", KeyConfirm: domain.ExecConfirmValue},
	}
	outcome, _, err := run(t, fx, Request{Jobs: 1}, prompter)
	if err != nil {
		t.Fatal(err)
	}
	if got := prompter.AskedKeys(); got != KeySelection+","+KeyCommand+","+KeyConfirm {
		t.Errorf("asked %q", got)
	}
	if outcome.Command != "echo hi" || len(outcome.Results) != 1 || outcome.Results[0].Tail[0] != "hi" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if recap := prompter.Content[KeyConfirm].Description; !strings.Contains(recap, "echo hi") {
		t.Errorf("recap %q misses the typed command", recap)
	}
}

func TestUnattendedWithoutACommandIsAUsageErrorNamingTheDash(t *testing.T) {
	fx := newFixture(t, "a")
	_, rec, err := run(t, fx, Request{Branches: []string{"a"}, Jobs: 1}, flow.Unattended{})
	if !errors.Is(err, domain.ErrUsage) || !errors.Is(err, domain.ErrExecNoCommand) || !strings.Contains(err.Error(), "--") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.progress) != 0 {
		t.Fatal("nothing may run without a command")
	}
}

func TestTheCommandStepRefusesWhatTheShellCannotRun(t *testing.T) {
	step := (&execFlow{}).commandStep()
	for _, bad := range []string{"", "   ", "if then"} {
		if step.Validate(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := step.Validate(""); strings.Contains(err.Error(), "--") {
		t.Errorf("the wizard is asking for the command; pointing at -- reads as a CLI error: %v", err)
	}
	if err := step.Validate("pnpm test && pnpm lint"); err != nil {
		t.Errorf("valid line refused: %v", err)
	}
}

func TestTheStepsReadLikeCreate(t *testing.T) {
	fx := newFixture(t, "a", "b")
	f := &execFlow{params: Params{Context: flow.Context{ProjectDir: fx.dir}, Request: Request{Jobs: 1}}}
	var labels []string
	for _, step := range f.session().Steps {
		labels = append(labels, step.Label)
	}
	if got := strings.Join(labels, " > "); got != "Worktrees > Command > Confirm & run" {
		t.Errorf("breadcrumb = %q", got)
	}

	confirm := func(branches ...string) string {
		content, err := f.confirmStep().Build(flow.NewAnswers(nil).WithValues(KeySelection, branches))
		if err != nil {
			t.Fatal(err)
		}
		return content.Options[0].Label
	}
	if got := confirm("a", "b"); got != "Yes, run in 2 worktrees" {
		t.Errorf("many = %q", got)
	}
	if got := confirm("a"); got != "Yes, run in a" {
		t.Errorf("one = %q", got)
	}
}
