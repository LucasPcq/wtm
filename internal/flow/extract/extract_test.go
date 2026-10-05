package extract

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	*flowtest.Recorder
	extracted *Outcome
	// order is what the run did, in the order it did it.
	order *[]string
}

func newRecorder() *recorder {
	return &recorder{Recorder: &flowtest.Recorder{}, order: &[]string{}}
}

func (r *recorder) Extracted(outcome Outcome) error {
	r.extracted = &outcome
	*r.order = append(*r.order, "extracted")
	return nil
}

type orderedPrompter struct {
	*flowtest.ScriptedPrompter
	order *[]string
}

func (p orderedPrompter) Ask(session flow.Session) (flow.Answers, error) {
	*p.order = append(*p.order, "ask")
	return p.ScriptedPrompter.Ask(session)
}

func (p orderedPrompter) Confirm(params flow.ConfirmParams) (bool, error) {
	*p.order = append(*p.order, "confirm")
	return p.ScriptedPrompter.Confirm(params)
}

type repo struct {
	ctx flow.Context
	src string
	dst string
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// newRepo holds two worktrees, src carrying a.txt untracked and dst empty.
func newRepo(t *testing.T) repo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	trees := filepath.Join(filepath.Dir(dir), ".trees")
	for _, branch := range []string{"src", "dst"} {
		gittest.CreateBranch(t, dir, branch)
		git(t, dir, "worktree", "add", filepath.Join(trees, branch), branch)
	}
	write(t, filepath.Join(trees, "src", "a.txt"), "a\n")

	config := domain.Config{}
	config.Project.Worktrees.BasePath = "../.trees"
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return repo{
		ctx: flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config},
		src: filepath.Join(trees, "src"),
		dst: filepath.Join(trees, "dst"),
	}
}

func TestAPickedRunAsksEveryQuestionInOrder(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeySource: "src", KeyTarget: "dst", KeyMode: modeMove, KeyRecap: confirmExtract},
		Sets:    map[string][]string{KeyFiles: {"a.txt"}},
	}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{Context: r.ctx, Prompter: prompter, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := prompter.AskedKeys(), "extract.source,extract.files,extract.target,extract.mode,extract.recap"; got != want {
		t.Errorf("asked %s, want %s", got, want)
	}
	if presenter.extracted == nil || len(presenter.extracted.Result.Files) != 1 {
		t.Fatalf("outcome = %+v, want one file extracted", presenter.extracted)
	}
	if read(t, filepath.Join(r.dst, "a.txt")) != "a\n" {
		t.Error("a.txt did not reach the target")
	}
}

func TestThePickedSourceListsItsChangesAsTheFilesStepLoads(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeySource: "src", KeyTarget: "dst", KeyMode: modeKeep, KeyRecap: confirmExtract},
		Sets:    map[string][]string{KeyFiles: {"a.txt"}},
	}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	files := prompter.Content[KeyFiles]
	if len(files.Options) != 1 || files.Options[0].Value != "a.txt" || files.Options[0].Tag != "new" {
		t.Errorf("files step = %+v, want a.txt tagged new", files.Options)
	}
	targets := prompter.Content[KeyTarget].Options
	for _, option := range targets {
		if option.Value == "src" {
			t.Error("the source is offered as its own target")
		}
	}
	if targets[0].Value != targetCreate {
		t.Errorf("the first target is %q, want the create-new row", targets[0].Value)
	}
}

// Every flag given still shows the recap, as create does: the flags say what
// to do, the recap is where the user sees it before it happens.
func TestFlagsLeaveOnlyTheRecapToAsk(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmExtract}}

	request := Request{Source: "src", Files: []string{"a.txt"}, To: "dst", Keep: true, KeepSet: true}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.AskedKeys(); got != KeyRecap {
		t.Errorf("asked %s, want only the recap", got)
	}
	recap := prompter.Content[KeyRecap].Description
	for _, want := range []string{"Source:    src", "Files:     a.txt", "Target:    dst", "Mode:      copy"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap is missing %q:\n%s", want, recap)
		}
	}
	if read(t, filepath.Join(r.src, "a.txt")) != "a\n" {
		t.Error("a copy removed the file from the source")
	}
}

func TestANewTargetAsksCreatesQuestionsThenCreatesIt(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			KeyTarget:        targetCreate,
			create.KeyBranch: "feat/split",
			create.KeySource: "main",
			KeyMode:          modeMove,
			KeyRecap:         confirmExtract,
		},
		Sets: map[string][]string{KeyFiles: {"a.txt"}},
	}
	presenter := newRecorder()

	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: Request{Source: "src"}, Prompter: prompter, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := prompter.AskedKeys(), "extract.files,extract.target,create.branch,create.source,extract.mode,extract.recap"; got != want {
		t.Errorf("asked %s, want %s", got, want)
	}
	if !strings.Contains(prompter.Content[KeyRecap].Description, "Target:    new worktree feat/split from main") {
		t.Errorf("recap:\n%s", prompter.Content[KeyRecap].Description)
	}
	result := presenter.extracted.Result
	if result.TargetBranch != "feat/split" || read(t, filepath.Join(result.TargetPath, "a.txt")) != "a\n" {
		t.Errorf("result = %+v, want a.txt in a new feat/split worktree", result)
	}
}

// The parent a new target is offered first is the source's own, ahead of the
// base branch, whether the source was named or picked.
func TestTheNewTargetsParentIsTheSourcesOwn(t *testing.T) {
	r := newRepo(t)
	gittest.CreateBranch(t, r.ctx.ProjectDir, "stack-base")
	write(t, filepath.Join(r.ctx.StateDir, "worktrees", "src", domain.MetaFileName), `{"source_branch":"stack-base"}`)

	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{
			KeySource:        "src",
			KeyTarget:        targetCreate,
			create.KeyBranch: "feat/split",
			create.KeySource: "stack-base",
			KeyMode:          modeMove,
			KeyRecap:         confirmExtract,
		},
		Sets: map[string][]string{KeyFiles: {"a.txt"}},
	}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.Content[create.KeySource].Pinned; got != "stack-base" {
		t.Errorf("the parent offered first = %q, want the picked source's recorded parent", got)
	}

	unattended := &extractFlow{ctx: r.ctx}
	if got := unattended.defaultParent(flow.NewAnswers(map[string]string{KeySource: "dst"})); got != "" {
		t.Errorf("a source with no recorded parent = %q, want create's base branch to stand in", got)
	}
}

func TestAConflictIsAskedAfterTheRecap(t *testing.T) {
	r := newRepo(t)
	write(t, filepath.Join(r.dst, "a.txt"), "already\n")
	presenter := newRecorder()
	prompter := orderedPrompter{
		ScriptedPrompter: &flowtest.ScriptedPrompter{Answers: map[string]string{KeyMode: modeMove, KeyRecap: confirmExtract}, Confirmed: true},
		order:            presenter.order,
	}

	request := Request{Source: "src", Files: []string{"a.txt"}, To: "dst"}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: prompter, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.Join(*presenter.order, ","); got != "ask,confirm,extracted" {
		t.Errorf("order = %s, want the recap, then the conflict, then the result", got)
	}
	if conflicts := presenter.extracted.Result.Conflicts; len(conflicts) != 1 {
		t.Errorf("conflicts = %v, want a.txt", conflicts)
	}
	if !strings.Contains(read(t, filepath.Join(r.dst, "a.txt")), "<<<<<<<") {
		t.Error("an accepted resolve wrote no marker")
	}
}

func TestADeclinedConflictChangesNothing(t *testing.T) {
	r := newRepo(t)
	write(t, filepath.Join(r.dst, "a.txt"), "already\n")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyMode: modeMove, KeyRecap: confirmExtract}}
	presenter := newRecorder()

	request := Request{Source: "src", Files: []string{"a.txt"}, To: "dst"}
	outcome, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: prompter, Presenter: presenter})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !outcome.Aborted || len(presenter.Notices) != 1 || !presenter.Notices[0].IsAbort() {
		t.Errorf("outcome %+v, notices %+v: want an abort, said once", outcome, presenter.Notices)
	}
	if read(t, filepath.Join(r.dst, "a.txt")) != "already\n" || read(t, filepath.Join(r.src, "a.txt")) != "a\n" {
		t.Error("a declined conflict touched a worktree")
	}
}

func TestAnAbortedWizardSaysSoAndChangesNothing(t *testing.T) {
	r := newRepo(t)
	presenter := newRecorder()
	outcome, err := Run(t.Context(), Params{Context: r.ctx, Prompter: &flowtest.ScriptedPrompter{Abort: true}, Presenter: presenter})
	if err != nil || !outcome.Aborted || presenter.extracted != nil {
		t.Errorf("outcome %+v, err %v: want an abort with no conclusion", outcome, err)
	}
}

func TestUnattendedRefusesWhatOnlyAPickerCouldAnswer(t *testing.T) {
	r := newRepo(t)
	cases := []struct {
		name    string
		request Request
		want    error
	}{
		{"no source", Request{Files: []string{"a.txt"}, To: "dst"}, domain.ErrExtractSourceRequired},
		{"no files", Request{Source: "src", To: "dst"}, domain.ErrExtractFilesRequired},
		{"no target", Request{Source: "src", Files: []string{"a.txt"}}, domain.ErrExtractTargetRequired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Run(t.Context(), Params{Context: r.ctx, Request: c.request, Prompter: flow.Unattended{}, Presenter: newRecorder()})
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestNoWorktreeWithChangesIsNothingToDo(t *testing.T) {
	r := newRepo(t)
	if err := os.Remove(filepath.Join(r.src, "a.txt")); err != nil {
		t.Fatal(err)
	}
	presenter := newRecorder()
	if _, err := Run(t.Context(), Params{Context: r.ctx, Prompter: &flowtest.ScriptedPrompter{}, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if presenter.extracted == nil || !errors.Is(presenter.extracted.Nothing, domain.ErrNoDirtyWorktrees) {
		t.Errorf("outcome = %+v, want nothing to extract", presenter.extracted)
	}
}

// A --to naming a worktree leaves create's questions out of the session, so the
// breadcrumb counts only what can still be asked.
func TestAnExistingTargetLeavesCreatesStepsOut(t *testing.T) {
	r := newRepo(t)
	f := &extractFlow{ctx: r.ctx, request: Request{Source: "src", To: "dst"}, changes: map[string][]domain.ExtractFile{}, paths: map[string]string{}}
	f.create = f.embed()
	for _, step := range f.session().Steps {
		if strings.HasPrefix(step.Key, "create.") {
			t.Errorf("step %s is in a session whose target exists", step.Key)
		}
	}
}

func TestANewTargetIsNotOfferedAsItsOwnParent(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeySource: "src", create.KeySource: "main", KeyMode: modeMove, KeyRecap: confirmExtract},
		Sets:    map[string][]string{KeyFiles: {"a.txt"}},
	}
	gittest.CreateBranch(t, r.ctx.ProjectDir, "old-br")
	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: Request{To: "old-br"}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	excluded := prompter.Content[create.KeySource].ExcludeBranches
	if len(excluded) != 1 || excluded[0] != "old-br" {
		t.Errorf("excluded = %v, want the target itself", excluded)
	}
}

func TestAFromNamingTheTargetIsRefused(t *testing.T) {
	r := newRepo(t)
	request := Request{Source: "src", Files: []string{"a.txt"}, To: "feat/x", From: "feat/x"}
	_, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: flow.Unattended{}, Presenter: newRecorder()})
	if err == nil || !strings.Contains(err.Error(), "own parent") {
		t.Errorf("err = %v, want the own-parent refusal", err)
	}
}

func TestFilesMatchingNoChangeAreRefusedBeforeTheRecap(t *testing.T) {
	r := newRepo(t)
	prompter := &flowtest.ScriptedPrompter{}
	_, err := Run(t.Context(), Params{Context: r.ctx, Request: Request{Source: "src", Files: []string{"zzz.txt"}, To: "dst"}, Prompter: prompter, Presenter: newRecorder()})
	if err == nil || len(prompter.Asked) != 0 {
		t.Errorf("err = %v, asked %v: want a refusal before any question", err, prompter.Asked)
	}
}

func TestTheRecapNamesTheFilesADirectoryStandsFor(t *testing.T) {
	r := newRepo(t)
	write(t, filepath.Join(r.src, "dir", "x.txt"), "x\n")
	write(t, filepath.Join(r.src, "dir", "y.txt"), "y\n")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyMode: modeMove, KeyRecap: confirmExtract}}
	request := Request{Source: "src", Files: []string{"dir/"}, To: "dst"}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if recap := prompter.Content[KeyRecap].Description; !strings.Contains(recap, "Files:     dir/x.txt, dir/y.txt") {
		t.Errorf("recap:\n%s", recap)
	}
}

func TestATargetGitWouldRefuseIsRefusedFirst(t *testing.T) {
	r := newRepo(t)
	request := Request{Source: "src", Files: []string{"a.txt"}, To: "bad..name"}
	_, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: flow.Unattended{}, Presenter: newRecorder()})
	if !errors.Is(err, domain.ErrUsage) {
		t.Errorf("err = %v, want the branch name refused as a usage error", err)
	}
}

func TestCreationFlagsOnAnExistingTargetAreSaidToBeIgnored(t *testing.T) {
	r := newRepo(t)
	presenter := newRecorder()
	request := Request{Source: "src", Files: []string{"a.txt"}, To: "dst", From: "main", FastForward: true}
	if _, err := Run(t.Context(), Params{Context: r.ctx, Request: request, Prompter: flow.Unattended{}, Presenter: presenter}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	warnings := presenter.extracted.Result.Warnings
	if len(warnings) != 2 || !strings.HasPrefix(warnings[0], "--from ignored: dst") || !strings.HasPrefix(warnings[1], "--ff ignored: dst") {
		t.Errorf("warnings = %v, want --from and --ff named", warnings)
	}
	if len(presenter.Statuses) != 2 {
		t.Errorf("statuses = %v, want each said as it happens", presenter.Statuses)
	}
}

func TestOnlyANewTargetIsPublished(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers map[string]string
		want    int
	}{
		{"new", map[string]string{KeyTarget: targetCreate, create.KeyBranch: "feat/split", create.KeySource: "main", KeyMode: modeMove, KeyRecap: confirmExtract}, 1},
		{"existing", map[string]string{KeyTarget: "dst", KeyMode: modeMove, KeyRecap: confirmExtract}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRepo(t)
			presenter := newRecorder()
			r.ctx.Publisher = presenter.Recorder
			prompter := &flowtest.ScriptedPrompter{Answers: tc.answers, Sets: map[string][]string{KeyFiles: {"a.txt"}}}

			if _, err := Run(t.Context(), Params{Context: r.ctx, Request: Request{Source: "src"}, Prompter: prompter, Presenter: presenter}); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if len(presenter.Published) != 2*tc.want {
				t.Fatalf("published %v, want %d worktree.created + provisioned", presenter.PublishedTypes(), tc.want)
			}
			if tc.want == 1 && (presenter.Published[0].Type != domain.EventWorktreeCreated || presenter.Published[0].Worktree.Branch != "feat/split" || presenter.Published[1].Type != domain.EventWorktreeProvisioned) {
				t.Fatalf("published %+v", presenter.Published)
			}
		})
	}
}
