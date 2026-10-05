package create

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// newFlow builds a flow over a directory with no git repository, so branch
// divergence resolves as unknown and the recap tests stay hermetic.
func newFlow(t *testing.T, request Request, target func(string) domain.BranchTarget) *createFlow {
	t.Helper()
	config := domain.Config{}
	config.Project.Env.Strategy = domain.EnvStrategyExample
	if target == nil {
		target = func(string) domain.BranchTarget { return domain.BranchTarget{} }
	}
	return &createFlow{
		ctx:      flow.Context{ProjectDir: t.TempDir(), Config: config},
		request:  request,
		prompter: flow.Unattended{},
		target:   target,
	}
}

func existing(branchName string) func(string) domain.BranchTarget {
	return func(name string) domain.BranchTarget {
		if name == branchName {
			return domain.BranchTarget{State: domain.BranchTargetExisting}
		}
		return domain.BranchTarget{}
	}
}

func answers(values map[string]string) flow.Answers { return flow.NewAnswers(values) }

// A flag resolves a step instead of asking it, and the recap must still name the
// value — otherwise a line disappears for the users who passed the most flags.
func TestRecapKeepsEveryLineWhateverAnsweredIt(t *testing.T) {
	recap := newFlow(t, Request{}, nil).recap(answers(map[string]string{
		KeyBranch: "feat/x",
		KeySource: "main",
		KeyEnv:    "example",
	}))

	for _, want := range []string{"Branch:    feat/x", "Source:    main", "Env:       example"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap %q should contain %q", recap, want)
		}
	}
}

func TestRecapNamesTheConfigDefaultEnv(t *testing.T) {
	recap := newFlow(t, Request{}, nil).recap(answers(map[string]string{KeyBranch: "feat/x", KeySource: "main"}))
	if !strings.Contains(recap, "Env:       config default") {
		t.Errorf("recap %q should name the empty env choice", recap)
	}
}

func TestRecapCallsTheSourceAParentForAReusedBranch(t *testing.T) {
	recap := newFlow(t, Request{}, existing("feat/x")).recap(answers(map[string]string{
		KeyBranch: "feat/x",
		KeySource: "main",
		KeyEnv:    "example",
	}))

	if !strings.Contains(recap, "Parent:    main") {
		t.Errorf("recap %q should label the source as the recorded parent", recap)
	}
	if strings.Contains(recap, "Source:    ") {
		t.Errorf("recap %q must not present the parent as a start-point", recap)
	}
	if !strings.Contains(recap, domain.BranchReusedSuffix) {
		t.Errorf("recap %q should mark the branch as reused", recap)
	}
}

// The annotation follows whichever branch is actually moved, so the recap can never
// claim to move a branch it leaves alone.
func TestRecapPutsTheFastForwardOnItsSubject(t *testing.T) {
	given := answers(map[string]string{
		KeyBranch:       "feat/x",
		KeySource:       "main",
		KeyEnv:          "example",
		KeySourceUpdate: updateFastForward,
	})

	onSource := newFlow(t, Request{}, nil).recap(given)
	if !strings.Contains(onSource, "Source:    main (fast-forward to origin)") {
		t.Errorf("recap %q should annotate the source line", onSource)
	}

	onBranch := newFlow(t, Request{}, existing("feat/x")).recap(given)
	if !strings.Contains(onBranch, "fast-forward feat/x to origin") {
		t.Errorf("recap %q should carry its own update line for the reused branch", onBranch)
	}
	if strings.Contains(onBranch, "Parent:    main (fast-forward") {
		t.Errorf("recap %q must not annotate the parent it does not move", onBranch)
	}
}

func TestSessionAsksOnlyWhatIsMissing(t *testing.T) {
	full := newFlow(t, Request{Branches: []string{"feat/x"}, From: "main", EnvFrom: "example"}, nil).session()
	for _, key := range []string{KeyBranch, KeySource, KeyEnv} {
		if _, preset := full.Presets.Get(key); !preset {
			t.Errorf("step %q should be answered by the request", key)
		}
	}

	bare := newFlow(t, Request{}, nil).session()
	for _, key := range []string{KeyBranch, KeySource, KeyEnv} {
		if _, preset := bare.Presets.Get(key); preset {
			t.Errorf("step %q should be left to be asked", key)
		}
	}
	if len(bare.Steps) != 6 {
		t.Errorf("declared %d steps, want branch, source, env, env ports, source update and recap", len(bare.Steps))
	}
}

func TestBranchStepRefusesWithoutABranchName(t *testing.T) {
	step := newFlow(t, Request{}, nil).branchStep()

	if _, err := step.Resolve(flow.Answers{}); err == nil {
		t.Fatal("expected a refusal without a branch name")
	}
	if _, err := flow.CheckEntry(step, flow.EntryCheck{Entry: "   "}); err == nil {
		t.Error("a blank branch name should be rejected as it is typed")
	}
	if _, err := flow.CheckEntry(step, flow.EntryCheck{Entry: "feat/x"}); err != nil {
		t.Errorf("a real branch name should validate: %v", err)
	}
	if err := step.ValidateSet(nil); err == nil {
		t.Error("an empty list should be refused")
	}
}

func TestSourceStepRefusesAGuessedParent(t *testing.T) {
	f := newFlow(t, Request{}, existing("feat/x"))
	f.ctx.Config.Project.Worktrees.BaseBranch = "main"

	_, err := f.resolveSource(answers(map[string]string{KeyBranch: "feat/x"}))
	if err == nil {
		t.Fatal("expected a refusal for a branch whose parent cannot be inferred")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Errorf("refusal %q should name the --%s flag", err, domain.FlagFrom)
	}
}

// --ff accepts the offer unattended; its absence keeps the branch where it is.
func TestSourceUpdateResolvesFromTheFlagOnly(t *testing.T) {
	answer, err := newFlow(t, Request{FastForward: true}, nil).sourceUpdateStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != updateFastForward {
		t.Errorf("answer = %q, want the fast-forward accepted", answer.Value)
	}

	answer, err = newFlow(t, Request{}, nil).sourceUpdateStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if answer.Value != updateKeep {
		t.Errorf("answer = %q, want the branch left as-is", answer.Value)
	}
}

func TestEnvSummaryNamesTheDefault(t *testing.T) {
	if got := envSummary(flow.Answer{}); got != domain.EnvSummaryConfigDefault {
		t.Errorf("summary = %q, want the config default named", got)
	}
	if got := envSummary(flow.Answer{Value: "main"}); got != "main" {
		t.Errorf("summary = %q, want the chosen strategy", got)
	}
}

type recorder struct {
	*flowtest.Recorder
	created *Outcome
}

func newRecorder() *recorder { return &recorder{Recorder: &flowtest.Recorder{}} }

func (r *recorder) Created(outcome Outcome) error {
	r.created = &outcome
	return nil
}

func (r *recorder) BranchStarted(flow.Progress) {}

func (r *recorder) BranchCreated(domain.CreateResult) {}

func (r *recorder) BranchFailed(domain.BatchFailure) {}

func testContext(t *testing.T) flow.Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

func TestRunAsksEveryQuestionThenCreates(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyBranch: "feat/w",
		KeySource: "main",
		KeyEnv:    "",
		KeyRecap:  confirmCreate,
	}}
	presenter := newRecorder()

	outcome, err := Run(Params{Context: testContext(t), Prompter: prompter, Presenter: presenter})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := strings.Join([]string{KeyBranch, KeySource, KeyEnv, KeyRecap}, ",")
	if prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	if _, asked := prompter.Content[KeySourceUpdate]; asked {
		t.Error("the source-update step should skip itself when there is nothing to reconcile")
	}

	recap := prompter.Content[KeyRecap].Description
	for _, line := range []string{"Branch:    feat/w", "Source:    main", "Env:       config default"} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should contain %q", recap, line)
		}
	}
	if presenter.created == nil || len(presenter.created.Results) != 1 || presenter.created.Results[0].Branch != "feat/w" {
		t.Fatalf("created = %+v, want the new worktree reported", presenter.created)
	}
	if outcome.Results[0].Metadata.SourceBranch != "main" {
		t.Errorf("source_branch = %q, want the answered source recorded", outcome.Results[0].Metadata.SourceBranch)
	}
	if _, statErr := os.Stat(outcome.Results[0].Path); statErr != nil {
		t.Errorf("worktree not on disk: %v", statErr)
	}
	if len(presenter.Stages) != 1 {
		t.Errorf("stages = %v, want just the creation", presenter.Stages)
	}
}

func TestRunSkipsTheQuestionsTheRequestAnswers(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmCreate}}

	if _, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/flagged"}, From: "main", EnvFrom: "example"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if prompter.AskedKeys() != KeyRecap {
		t.Errorf("asked %q, want the recap alone", prompter.AskedKeys())
	}
	recap := prompter.Content[KeyRecap].Description
	for _, line := range []string{"Branch:    feat/flagged", "Source:    main", "Env:       example"} {
		if !strings.Contains(recap, line) {
			t.Errorf("recap %q should still contain %q", recap, line)
		}
	}
}

func TestRunAbortedCreatesNothing(t *testing.T) {
	presenter := newRecorder()

	outcome, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/nope"}, From: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted {
		t.Error("outcome should report the abort")
	}
	if len(presenter.Notices) != 1 || presenter.Notices[0].Text != domain.AbortedMessage {
		t.Errorf("notices = %+v, want a single %q", presenter.Notices, domain.AbortedMessage)
	}
	if presenter.created != nil || len(presenter.Stages) != 0 {
		t.Error("nothing should have been created")
	}
}

func TestRunRunsHooksAsTheirOwnPhase(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "echo hooked"}}
	presenter := newRecorder()

	if _, err := Run(Params{
		Context: ctx,
		Request: Request{Branches: []string{"feat/hooked"}, From: "main"},
		Prompter: &flowtest.ScriptedPrompter{Answers: map[string]string{
			KeyEnv:   "",
			KeyRecap: confirmCreate,
		}},
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(presenter.Hooks) != 1 || presenter.Hooks[0] != domain.HooksTitleOnCreate {
		t.Errorf("hook phases = %v, want one titled %q", presenter.Hooks, domain.HooksTitleOnCreate)
	}
}

func TestRunRefusesABranchHeldElsewhereBeforeAsking(t *testing.T) {
	ctx := testContext(t)
	gittest.CreateBranch(t, ctx.ProjectDir, "feat/taken")
	gittest.Git(t, ctx.ProjectDir, "worktree", "add", filepath.Join(t.TempDir(), "taken"), "feat/taken")

	prompter := &flowtest.ScriptedPrompter{}
	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/taken"}, From: "main"},
		Prompter:  prompter,
		Presenter: newRecorder(),
	})
	if !errors.Is(err, domain.ErrWorktreeExists) {
		t.Fatalf("err = %v, want it to wrap ErrWorktreeExists", err)
	}
	if len(prompter.Asked) != 0 {
		t.Errorf("asked %v, want nothing asked before the refusal", prompter.Asked)
	}
}

func TestFastForwardSubjectIsSharedWithTheOtherFlows(t *testing.T) {
	if got := decide.FastForwardSubject(decide.FastForwardSubjectParams{
		Target:     domain.BranchTarget{State: domain.BranchTargetExisting},
		FromBranch: "main",
		Branch:     "feat/x",
	}); got != "feat/x" {
		t.Errorf("subject = %q, want the reused branch", got)
	}
}

// withRunConfig gives the flow a state dir holding this run.toml.
func withRunConfig(t *testing.T, f *createFlow, cfg domain.RunConfig) *createFlow {
	t.Helper()
	stateDir := t.TempDir()
	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Force: true, Config: cfg}); err != nil {
		t.Fatalf("write run config: %v", err)
	}
	f.ctx.StateDir = stateDir
	return f
}

func portedConfig(isolation domain.Isolation) domain.RunConfig {
	return domain.RunConfig{
		Isolation: isolation,
		Jobs: []domain.JobConfig{{
			Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev",
			Ports: map[string]int{"PORT": 3000},
		}},
	}
}

// What the step decides is written into the .env this run provisions, so the
// question belongs to that run — not to a second confirmation put after the
// worktree exists.
func TestIsolationIsAskedBeforeTheWorktreeExists(t *testing.T) {
	f := newFlow(t, Request{}, nil)
	session := f.session()

	var found bool
	for index, step := range session.Steps {
		if step.Key != KeyIsolation {
			continue
		}
		found = true
		if step.Key == session.Steps[len(session.Steps)-1].Key {
			t.Error("the isolation step is the recap, want it asked before it")
		}
		if index == 0 {
			t.Error("the isolation step leads the session, want it after what it depends on")
		}
	}
	if !found {
		t.Fatal("the session declares no isolation step")
	}
}

// With nothing to isolate both answers do the same thing, so nobody is asked.
func TestIsolationIsNotAskedWithNothingToIsolate(t *testing.T) {
	step := newFlow(t, Request{}, nil).isolationStep()

	skip, reason := step.Skip(flow.Answers{})
	if !skip {
		t.Fatal("the step was posed for a project that declares nothing to isolate")
	}
	if reason != domain.IsolationStepIrrelevant {
		t.Errorf("reason = %q, want %q", reason, domain.IsolationStepIrrelevant)
	}
}

func TestIsolationIsAskedOnceAPortIsDeclared(t *testing.T) {
	step := withRunConfig(t, newFlow(t, Request{}, nil), portedConfig("")).isolationStep()
	if skip, reason := step.Skip(flow.Answers{}); skip {
		t.Errorf("skipped (%s), want the question put for a project declaring a port", reason)
	}
}

// Nobody to ask: the project's standing answer decides, and a project that
// never gave one gets what every worktree got before the question existed.
func TestIsolationResolvesToTheProjectDefault(t *testing.T) {
	cases := []struct {
		name      string
		isolation domain.Isolation
		want      domain.Isolation
	}{
		{name: "unset", isolation: "", want: domain.IsolationIsolated},
		{name: "verbatim", isolation: domain.IsolationVerbatim, want: domain.IsolationVerbatim},
	}
	for _, tc := range cases {
		f := withRunConfig(t, newFlow(t, Request{}, nil), portedConfig(tc.isolation))
		answer, err := f.isolationStep().Resolve(flow.Answers{})
		if err != nil {
			t.Fatalf("%s: Resolve: %v", tc.name, err)
		}
		if domain.Isolation(answer.Value) != tc.want {
			t.Errorf("%s: Resolve = %q, want %q", tc.name, answer.Value, tc.want)
		}
		if first := f.isolationStep().Options[0].Value; domain.Isolation(first) != tc.want {
			t.Errorf("%s: first option = %q, want the default %q first", tc.name, first, tc.want)
		}
	}
}

// --isolation answers the step, and the recap still names it.
func TestRecapNamesTheIsolation(t *testing.T) {
	f := newFlow(t, Request{Isolation: domain.IsolationVerbatim}, nil)

	values := f.session().Presets
	if got := domain.Isolation(values.Value(KeyIsolation)); got != domain.IsolationVerbatim {
		t.Errorf("isolation = %q, want the flag's", got)
	}
	if recap := f.recap(values); !strings.Contains(recap, "Isolation: "+domain.IsolationSummaryVerbatim) {
		t.Errorf("recap = %q, want the isolation named", recap)
	}

	// Not asked, not recapped: a project with nothing to isolate never saw it.
	silent := f.recap(answers(map[string]string{KeyBranch: "feat/x", KeySource: "main"}))
	if strings.Contains(silent, domain.RecapFieldIsolation) {
		t.Errorf("recap = %q, want no line for a step that was never posed", silent)
	}
}

func multiRun(t *testing.T, request Request, sets []string) (Outcome, *flowtest.ScriptedPrompter, error) {
	t.Helper()
	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeySource: "main", KeyEnv: "", KeyRecap: confirmCreate},
		Sets:    map[string][]string{KeyBranch: sets},
	}
	outcome, err := Run(Params{Context: testContext(t), Request: request, Prompter: prompter, Presenter: newRecorder()})
	return outcome, prompter, err
}

func TestMultiAsksTheListThenTheSharedQuestionsOnce(t *testing.T) {
	outcome, prompter, err := multiRun(t, Request{}, []string{"feat/a", "feat/b"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := strings.Join([]string{KeyBranch, KeySource, KeyEnv, KeyRecap}, ",")
	if prompter.AskedKeys() != want {
		t.Errorf("asked %q, want %q", prompter.AskedKeys(), want)
	}
	if !strings.Contains(prompter.Content[KeyRecap].Description, "Branches:  feat/a, feat/b") {
		t.Errorf("recap %q should list both branches", prompter.Content[KeyRecap].Description)
	}
	if len(outcome.Results) != 2 {
		t.Errorf("results = %d, want both created", len(outcome.Results))
	}
}

func TestMultiWithSeveralArgumentsSkipsTheListStep(t *testing.T) {
	outcome, prompter, err := multiRun(t, Request{Branches: []string{"feat/a", "feat/b"}}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, asked := prompter.Content[KeyBranch]; asked {
		t.Error("several arguments answer the step, as a single one does")
	}
	if !strings.Contains(prompter.Content[KeyRecap].Description, "feat/b") {
		t.Errorf("recap %q should list both arguments", prompter.Content[KeyRecap].Description)
	}
	if len(outcome.Results) != 2 {
		t.Errorf("results = %d, want both arguments created", len(outcome.Results))
	}
}

func TestMultiWithOneArgumentSkipsTheListStep(t *testing.T) {
	_, prompter, err := multiRun(t, Request{Branches: []string{"feat/a"}}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, asked := prompter.Content[KeyBranch]; asked {
		t.Error("a single argument answers the step, as it always did")
	}
}

func TestMultiUnattendedTakesTheArguments(t *testing.T) {
	outcome, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/a", "feat/b"}},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Results) != 2 {
		t.Errorf("results = %d, want both created", len(outcome.Results))
	}
}

func TestRunRefusesADuplicateArgument(t *testing.T) {
	presenter := newRecorder()
	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/a", "feat/a"}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err == nil || !strings.Contains(err.Error(), "feat/a") {
		t.Fatalf("err = %v, want the duplicate named", err)
	}
	if len(presenter.Stages) != 0 {
		t.Errorf("stages = %v, nothing must be created", presenter.Stages)
	}
}

func TestRunRefusesAClashInsideTheListBeforeCreatingAnything(t *testing.T) {
	ctx := testContext(t)
	linkedRunConfig(t, ctx, ".env")
	presenter := newRecorder()

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/x", "feat.x"}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if !errors.Is(err, domain.ErrWorktreeNameTaken) {
		t.Fatalf("err = %v, want ErrWorktreeNameTaken", err)
	}
	if len(presenter.Stages) != 0 {
		t.Errorf("stages = %v, nothing must be created", presenter.Stages)
	}
}

func TestMixedListWithoutFromIsRefusedUnattended(t *testing.T) {
	ctx := testContext(t)
	gittest.CreateBranch(t, ctx.ProjectDir, "feat/old")

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/new", "feat/old"}},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err == nil || !strings.Contains(err.Error(), "feat/old") || !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Fatalf("err = %v, want a refusal naming feat/old and --%s", err, domain.FlagFrom)
	}
}

func TestEntryBadgeNamesNewAndExisting(t *testing.T) {
	f := newFlow(t, Request{}, existing("feat/old"))
	if got := f.entryBadge("feat/old").Text; got != domain.BranchEntryExisting {
		t.Errorf("badge = %q, want existing", got)
	}
	if got := f.entryBadge("feat/new").Text; got != domain.BranchEntryNew {
		t.Errorf("badge = %q, want new", got)
	}
}

type batchRecorder struct {
	*recorder
	started   []flow.Progress
	announced []string
	failed    []domain.BatchFailure
}

func (r *batchRecorder) BranchStarted(p flow.Progress) { r.started = append(r.started, p) }

func (r *batchRecorder) BranchCreated(c domain.CreateResult) {
	r.announced = append(r.announced, c.Branch)
}

func (r *batchRecorder) BranchFailed(f domain.BatchFailure) { r.failed = append(r.failed, f) }

func occupy(t *testing.T, ctx flow.Context, branchName string) {
	t.Helper()
	dir := filepath.Join(ctx.ProjectDir, ctx.Config.Project.Worktrees.BasePath, rules.SanitizeBranchName(branchName))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAFailureInTheMiddleDoesNotStopTheRest(t *testing.T) {
	ctx := testContext(t)
	occupy(t, ctx, "feat/b")
	presenter := &batchRecorder{recorder: newRecorder()}

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/a", "feat/b", "feat/c"}, From: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})

	if len(outcome.Results) != 2 || len(outcome.Failed) != 1 || outcome.Failed[0].Branch != "feat/b" {
		t.Fatalf("outcome = %+v, want feat/a and feat/c created, feat/b failed", outcome)
	}
	if !errors.Is(err, domain.ErrAborted) {
		t.Errorf("err = %v, want ErrAborted: the readout already reported the failure", err)
	}
	if !errors.Is(err, domain.ErrWorktreePathExists) {
		t.Errorf("err = %v, want the first cause kept for the exit code", err)
	}
	if outcome.Failed[0].ExitCode != domain.ExitCodeWorktreeExists {
		t.Errorf("exit_code = %d", outcome.Failed[0].ExitCode)
	}
	if len(presenter.started) != 3 || presenter.started[1].Position != 2 || presenter.started[1].Total != 3 {
		t.Errorf("started = %+v, want one header per branch", presenter.started)
	}
	if len(presenter.failed) != 1 {
		t.Errorf("failed = %+v", presenter.failed)
	}
	if !slices.Equal(presenter.announced, []string{"feat/a", "feat/c"}) {
		t.Errorf("announced = %v, want each branch announced as soon as it exists", presenter.announced)
	}
	if presenter.created == nil {
		t.Error("the conclusion must be presented even with a failure")
	}
}

func TestASingleBranchFailsAsBefore(t *testing.T) {
	ctx := testContext(t)
	occupy(t, ctx, "feat/b")
	presenter := &batchRecorder{recorder: newRecorder()}

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/b"}, From: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if errors.Is(err, domain.ErrAborted) || !errors.Is(err, domain.ErrWorktreePathExists) {
		t.Errorf("err = %v, want the raw cause so the root prints it as today", err)
	}
	if len(presenter.started) != 0 || len(presenter.failed) != 0 || len(presenter.announced) != 0 {
		t.Error("a single branch gets no per-branch header or failure line")
	}
}

func TestFastForwardReachesAnExistingBranchWhenTheSourceIsUpToDate(t *testing.T) {
	ctx := testContext(t)
	gittest.AddOrigin(t, ctx.ProjectDir)
	gittest.Git(t, ctx.ProjectDir, "checkout", "-b", "feat/old")
	gittest.Git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "on origin only")
	gittest.PushBranch(t, ctx.ProjectDir, "feat/old")
	gittest.Git(t, ctx.ProjectDir, "checkout", "main")
	gittest.Git(t, ctx.ProjectDir, "branch", "-f", "feat/old", "main")

	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/new", "feat/old"}, From: "main", FastForward: true},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	local, remote := revParse(t, ctx.ProjectDir, "feat/old"), revParse(t, ctx.ProjectDir, "origin/feat/old")
	if local != remote {
		t.Errorf("feat/old = %s, want it fast-forwarded to origin %s: --ff promises it for every existing branch", local, remote)
	}
}

func TestADuplicateArgumentIsAUsageError(t *testing.T) {
	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/a", "feat/a"}},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if code := rules.ExitCode(err); code != domain.ExitCodeUsage {
		t.Errorf("exit code = %d, want %d for a malformed command line", code, domain.ExitCodeUsage)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func TestRecapConfirmNamesHowManyWorktrees(t *testing.T) {
	_, prompter, err := multiRun(t, Request{}, []string{"feat/a", "feat/b"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.Content[KeyRecap].Options[0].Label; got != "Yes, create 2 worktrees" {
		t.Errorf("confirm = %q, want the count", got)
	}
}

func TestPositionalArgumentsAreTrimmed(t *testing.T) {
	outcome, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{" feat/a ", "feat/b"}},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if outcome.Results[0].Branch != "feat/a" {
		t.Errorf("branch = %q, want the argument trimmed as the wizard would", outcome.Results[0].Branch)
	}
}

func TestABlankArgumentIsAUsageError(t *testing.T) {
	presenter := newRecorder()
	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/a", "  "}},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if code := rules.ExitCode(err); code != domain.ExitCodeUsage {
		t.Errorf("err = %v (exit %d), want a usage error", err, code)
	}
	if len(presenter.Stages) != 0 {
		t.Errorf("stages = %v, nothing must be created", presenter.Stages)
	}
}

func TestARepeatedArgumentIsWordedForTheCommandLine(t *testing.T) {
	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/a", "feat/a"}},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err == nil || !strings.Contains(err.Error(), "feat/a is given twice") {
		t.Errorf("err = %v, want it worded for arguments", err)
	}
}

func TestTheWizardSpeaksInThePluralForSeveralBranches(t *testing.T) {
	ctx := testContext(t)
	linkedRunConfig(t, ctx, ".env")
	gittest.AddOrigin(t, ctx.ProjectDir)
	gittest.Git(t, ctx.ProjectDir, "commit", "--allow-empty", "-m", "on origin only")
	gittest.Git(t, ctx.ProjectDir, "push", "origin", "main")
	gittest.Git(t, ctx.ProjectDir, "reset", "--hard", "HEAD~1")

	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeySource: "main", KeyEnv: "", KeyIsolation: string(domain.IsolationIsolated), KeySourceUpdate: updateKeep, KeyRecap: confirmCreate},
		Sets:    map[string][]string{KeyBranch: {"feat/a", "feat/b"}},
	}
	if _, err := Run(Params{Context: ctx, Request: Request{}, Prompter: prompter, Presenter: newRecorder()}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for key, want := range map[string]string{
		KeySource:       domain.CreateSourceStepDescriptionMany,
		KeyEnv:          domain.CreateEnvStepDescriptionMany,
		KeyIsolation:    domain.IsolationStepDescriptionMany,
		KeySourceUpdate: domain.SourceFastForwardDescriptionMany,
	} {
		if got := prompter.Content[key].Description; got != want {
			t.Errorf("%s description = %q, want %q", key, got, want)
		}
	}
	options := prompter.Content[KeyIsolation].Options
	if len(options) != 2 || options[0].Label != domain.IsolationOptionIsolatedMany || options[1].Label != domain.IsolationOptionVerbatimMany {
		t.Errorf("isolation options = %+v, want the plural labels", options)
	}
}

func TestTheWizardKeepsTheSingularForOneBranch(t *testing.T) {
	_, prompter, err := multiRun(t, Request{}, []string{"feat/a"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.Content[KeySource].Description; got != domain.CreateSourceStepDescription {
		t.Errorf("source description = %q, want the singular", got)
	}
	if got := prompter.Content[KeyEnv].Description; got != domain.CreateEnvStepDescription {
		t.Errorf("env description = %q, want the singular", got)
	}
}

// Interactively the recap warns of it; unattended, the run says it afterwards.
func TestUnattendedRunWarnsTheParentFallback(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	if err := os.WriteFile(filepath.Join(ctx.ProjectDir, ".env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "branch", "develop")
	cmd.Dir = ctx.ProjectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %s: %v", out, err)
	}
	presenter := newRecorder()

	outcome, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat/fallback"}, From: "develop", EnvFrom: string(domain.EnvStrategyParent)},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if warnings := outcome.Results[0].Warnings; len(warnings) != 1 || warnings[0] != domain.EnvParentFallbackWarning {
		t.Errorf("warnings = %v, want the fallback named", warnings)
	}
	if len(presenter.Statuses) != 1 {
		t.Errorf("statuses = %+v, want the fallback said", presenter.Statuses)
	}
}

func TestAFromNamingTheBranchItselfIsRefused(t *testing.T) {
	_, err := Run(Params{
		Context:   testContext(t),
		Request:   Request{Branches: []string{"feat/x"}, From: "feat/x"},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err == nil || !strings.Contains(err.Error(), "own parent") {
		t.Errorf("err = %v, want the own-parent refusal", err)
	}
}

func TestRunPublishesTheNewWorktreeOnceAndAReusedOneNever(t *testing.T) {
	ctx := testContext(t)
	presenter := newRecorder()
	ctx.Publisher = presenter.Recorder
	run := func() {
		t.Helper()
		if _, err := Run(Params{
			Context:   ctx,
			Request:   Request{Branches: []string{"feat/pub"}, From: "main", IfNotExists: true},
			Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyEnv: "", KeyRecap: confirmCreate}},
			Presenter: presenter,
		}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	run()
	run()

	want := []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeProvisioned}
	if got := presenter.PublishedTypes(); !slices.Equal(got, want) {
		t.Fatalf("published %v, want %v once: the second run met the worktree already there", got, want)
	}
	if created := presenter.Published[0].Worktree; created.Branch != "feat/pub" || created.Parent != "main" {
		t.Fatalf("created = %+v", created)
	}
}

func runCreate(t *testing.T, ctx flow.Context, presenter Presenter, branches ...string) error {
	t.Helper()
	_, err := Run(Params{
		Context:   ctx,
		Request:   Request{Branches: branches, From: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	return err
}

func provisioned(events []domain.Event) []domain.Event {
	var out []domain.Event
	for _, event := range events {
		if event.Type == domain.EventWorktreeProvisioned {
			out = append(out, event)
		}
	}
	return out
}

func TestCreatePublishesProvisionedAfterCreated(t *testing.T) {
	ctx := testContext(t)
	presenter := newRecorder()
	ctx.Publisher = presenter.Recorder

	if err := runCreate(t, ctx, presenter, "feat/pub"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []domain.EventType{domain.EventWorktreeCreated, domain.EventWorktreeProvisioned}
	if got := presenter.PublishedTypes(); !slices.Equal(got, want) {
		t.Fatalf("published %v, want %v", got, want)
	}
	if got := presenter.Published[1]; got.OK == nil || !*got.OK || got.Worktree.Branch != "feat/pub" {
		t.Fatalf("provisioned = %+v", got)
	}
}

func TestAFailingOnCreateHookIsProvisionedNotOK(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "exit 4"}}
	presenter := newRecorder()
	ctx.Publisher = presenter.Recorder

	if err := runCreate(t, ctx, presenter, "feat/broken"); err == nil {
		t.Fatal("want the hook's error")
	}

	got := provisioned(presenter.Published)
	if len(got) != 1 || *got[0].OK || got[0].Hook != "exit 4" || got[0].ExitCode == nil || *got[0].ExitCode != 4 {
		t.Fatalf("provisioned = %+v", got)
	}
}

func TestABatchPublishesOneProvisionedPerBranch(t *testing.T) {
	ctx := testContext(t)
	ctx.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: `test "$(basename "$PWD")" != feat-b`}}
	presenter := &batchRecorder{recorder: newRecorder()}
	ctx.Publisher = presenter.Recorder

	_ = runCreate(t, ctx, presenter, "feat/a", "feat/b")

	got := provisioned(presenter.Published)
	if len(got) != 2 || got[0].Worktree.Branch != "feat/a" || !*got[0].OK || got[1].Worktree.Branch != "feat/b" || *got[1].OK {
		t.Fatalf("provisioned = %+v, want feat/a ok then feat/b not ok", got)
	}
}
