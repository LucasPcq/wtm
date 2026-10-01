package create

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
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
	if err := step.Validate("   "); err == nil {
		t.Error("a blank branch name should be rejected as it is typed")
	}
	if err := step.Validate("feat/x"); err != nil {
		t.Errorf("a real branch name should validate: %v", err)
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
	request.Multi = true
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

func TestMultiPrefillsTheListFromSeveralArguments(t *testing.T) {
	_, prompter, err := multiRun(t, Request{Branches: []string{"feat/a", "feat/b"}}, []string{"feat/a", "feat/b", "feat/c"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.Join(prompter.Content[KeyBranch].Entries, ","); got != "feat/a,feat/b" {
		t.Errorf("pre-fill = %q, want the arguments", got)
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
		Request:   Request{Branches: []string{"feat/a", "feat/b"}, Multi: true},
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
		Request:   Request{Branches: []string{"feat/a", "feat/a"}, Multi: true},
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
		Request:   Request{Branches: []string{"feat/x", "feat.x"}, Multi: true},
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
		Request:   Request{Branches: []string{"feat/new", "feat/old"}, Multi: true},
		Prompter:  flow.Unattended{},
		Presenter: newRecorder(),
	})
	if err == nil || !strings.Contains(err.Error(), "feat/old") || !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Fatalf("err = %v, want a refusal naming feat/old and --%s", err, domain.FlagFrom)
	}
}

func TestEntryBadgeNamesNewAndExisting(t *testing.T) {
	f := newFlow(t, Request{Multi: true}, existing("feat/old"))
	if got := f.entryBadge("feat/old").Text; got != domain.BranchEntryExisting {
		t.Errorf("badge = %q, want existing", got)
	}
	if got := f.entryBadge("feat/new").Text; got != domain.BranchEntryNew {
		t.Errorf("badge = %q, want new", got)
	}
}
