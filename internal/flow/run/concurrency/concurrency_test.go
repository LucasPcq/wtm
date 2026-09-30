package concurrency

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

const here = "/wt/here"

func running(dirs ...string) []domain.JobInfo {
	jobs := make([]domain.JobInfo, 0, len(dirs))
	for i, dir := range dirs {
		jobs = append(jobs, domain.JobInfo{
			Name:    []string{"web", "db", "api"}[i%3],
			WorkDir: dir,
			Status:  domain.JobStatusRunning,
		})
	}
	return jobs
}

// request is what a run hands the question, reduced to what these tests vary.
type request struct {
	Exclusive bool
	Parallel  bool
	Config    domain.RunConfig
}

func questionWith(req request, jobs []domain.JobInfo) *Question {
	return questionFor(questionParams{Request: req, Running: jobs, Presenter: &flowtest.Recorder{}})
}

type questionParams struct {
	Request   request
	Running   []domain.JobInfo
	Presenter flow.Presenter
	StateDir  string
}

func questionFor(params questionParams) *Question {
	cfg := params.Request.Config
	return New(Params{
		Context:   flow.Context{StateDir: params.StateDir},
		Presenter: params.Presenter,
		Exclusive: params.Request.Exclusive,
		Parallel:  params.Request.Parallel,
		Config:    cfg,
		Running:   params.Running,
		WorkDirs: func(answers flow.Answers) []string {
			return target.WorkDirs(target.WorkDirsParams{Answers: answers, Cwd: here})
		},
		Starting: func(flow.Answers) []domain.JobConfig { return rules.JobsWithEffectivePorts(cfg, cfg.Jobs) },
	})
}

func TestConcurrencyIsNotAskedWhenNothingRunsElsewhere(t *testing.T) {
	f := questionWith(request{}, running(here))

	skip, reason := f.Step().Skip(flow.Answers{})
	if !skip || reason != domain.RunConcurrencySkipAlone {
		t.Errorf("Skip = (%v, %q), want the step skipped for want of a neighbour", skip, reason)
	}
}

// A worktree's own jobs are never a reason to ask: `run up X` must not offer to
// stop X's own services.
func TestConcurrencyMeasuresAgainstTheTargetNotTheCurrentDirectory(t *testing.T) {
	f := questionWith(request{}, running("/wt/other"))
	answers := flow.NewAnswers(map[string]string{"run.worktree": "/wt/other"})

	if skip, _ := f.Step().Skip(answers); !skip {
		t.Error("the step was asked about the very worktree the run targets")
	}
}

func TestConcurrencyIsAskedOnceThenNeverAgain(t *testing.T) {
	asked := questionWith(request{}, running(here, "/wt/other"))
	if skip, _ := asked.Step().Skip(flow.Answers{}); skip {
		t.Fatal("the step was skipped although another worktree is running jobs")
	}

	settled := questionWith(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, running(here, "/wt/other"))
	skip, reason := settled.Step().Skip(flow.Answers{})
	if !skip || reason != domain.RunConcurrencySkipSettled {
		t.Errorf("Skip = (%v, %q), want the config to have settled it", skip, reason)
	}
}

func TestConcurrencyFlagsAnswerWithoutAsking(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request request
		want    domain.Concurrency
	}{
		{"--exclusive", request{Exclusive: true}, domain.ConcurrencyExclusive},
		{"--parallel", request{Parallel: true}, domain.ConcurrencyParallel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := questionWith(tc.request, running(here, "/wt/other"))
			step := f.Step()

			if skip, _ := step.Skip(flow.Answers{}); !skip {
				t.Error("the step was asked although a flag answered it")
			}
			answer, err := step.Resolve(flow.Answers{})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if concurrencyOf(answer.Value) != tc.want {
				t.Errorf("Resolve = %q, want %q", answer.Value, tc.want)
			}
		})
	}
}

// The safe default stops nothing: an unattended run must never tear down
// another worktree's services on its own initiative.
func TestConcurrencyResolvesToLeavingTheOthersAlone(t *testing.T) {
	f := questionWith(request{}, running(here, "/wt/other"))

	answer, err := f.Step().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if concurrencyOf(answer.Value) != domain.ConcurrencyParallel {
		t.Errorf("Resolve = %q, want the answer that stops nothing", answer.Value)
	}
}

func TestConcurrencyOffersFourAnswersAndNamesTheNeighbours(t *testing.T) {
	f := questionWith(request{}, running(here, "/wt/other"))

	content, err := f.Step().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	values := make([]string, 0, len(content.Options))
	for _, option := range content.Options {
		if option.Separator {
			continue
		}
		values = append(values, option.Value)
	}
	want := []string{answerParallel, answerParallelAlways, answerExclusive, answerExclusiveAlways}
	if strings.Join(values, ",") != strings.Join(want, ",") {
		t.Errorf("options = %v, want %v", values, want)
	}
	if !strings.Contains(content.Description, "other") {
		t.Errorf("description = %q, want it to name the worktree it is about", content.Description)
	}
}

func TestRememberWritesTheAnswerToRunTomlAndSaysSo(t *testing.T) {
	stateDir := t.TempDir()
	recorder := &flowtest.Recorder{}
	f := questionFor(questionParams{
		StateDir:  stateDir,
		Request:   request{Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}}}},
		Presenter: recorder,
	})

	answers := flow.NewAnswers(map[string]string{Key: answerExclusiveAlways})
	if _, err := f.remember(answers); err != nil {
		t.Fatalf("remember: %v", err)
	}

	saved, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	if saved.Concurrency != domain.ConcurrencyExclusive {
		t.Errorf("run.toml concurrency = %q, want %q", saved.Concurrency, domain.ConcurrencyExclusive)
	}
	if len(recorder.Statuses) != 1 || !strings.Contains(recorder.Statuses[0].Text, string(domain.ConcurrencyExclusive)) {
		t.Errorf("statuses = %v, want the write to be announced", recorder.Statuses)
	}
}

// A one-off answer changes nothing on disk: only the two "always" options do.
func TestAOneOffAnswerIsNotRemembered(t *testing.T) {
	stateDir := t.TempDir()
	f := questionFor(questionParams{StateDir: stateDir, Presenter: &flowtest.Recorder{}})

	if _, err := f.remember(flow.NewAnswers(map[string]string{Key: answerExclusive})); err != nil {
		t.Fatalf("remember: %v", err)
	}

	saved, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	if saved.Concurrency != "" {
		t.Errorf("run.toml concurrency = %q, want it untouched", saved.Concurrency)
	}
}

// The regression that shipped: Skip short-circuits Resolve, so a settled step
// carries no value at all. Reading the answer alone turned every
// non-interactive --exclusive — and every project that had written
// concurrency = "exclusive" — into a parallel run that stopped nothing.
func TestASettledConcurrencyIsStillActedOn(t *testing.T) {
	cases := []struct {
		name    string
		request request
		want    domain.Concurrency
	}{
		{"--exclusive", request{Exclusive: true}, domain.ConcurrencyExclusive},
		{"--parallel", request{Parallel: true}, domain.ConcurrencyParallel},
		{"config says exclusive", request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, domain.ConcurrencyExclusive},
		{"config says parallel", request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyParallel}}, domain.ConcurrencyParallel},
		{"nobody answered", request{}, domain.ConcurrencyParallel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := questionWith(tc.request, running(here, "/wt/other"))

			// What an unattended run produces: Skip wins, and the step is recorded
			// as skipped with no value.
			answers, err := flow.Unattended{}.Ask(flow.Session{Steps: []flow.Step{f.Step()}})
			if err != nil {
				t.Fatalf("Ask: %v", err)
			}
			if answers.Answered(Key) {
				t.Fatal("the step was answered, so this test no longer covers the skipped path")
			}

			if got := f.Decided(answers); got != tc.want {
				t.Errorf("concurrency = %q, want %q", got, tc.want)
			}
		})
	}
}

// A picker's answer outranks what the flags and the config would have resolved.
func TestAnAnsweredConcurrencyOutranksTheFallback(t *testing.T) {
	f := questionWith(request{Parallel: true}, running(here, "/wt/other"))
	answers := flow.Answers{}.With(Key, flow.Answer{Value: answerExclusiveAlways, Asked: true})

	if got := f.Decided(answers); got != domain.ConcurrencyExclusive {
		t.Errorf("concurrency = %q, want the answer that was actually given", got)
	}
}

// The setting says one stack at a time and the run brings up three. It is a
// guard rail, not an ambush: the contradiction is one the user just created.
func TestASettledExclusiveIsPutBackToTheUserOnAMultiWorktreeRun(t *testing.T) {
	f := questionWith(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, nil)
	answers := flow.Answers{}.WithValues("run.worktree", []string{here, "/wt/other"})

	step := f.Step()
	if skip, reason := step.Skip(answers); skip {
		t.Fatalf("the step was skipped (%q) although the run contradicts the setting", reason)
	}

	content, err := step.Build(answers)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Title != domain.RunConcurrencyContradictionTitle {
		t.Errorf("title = %q, want the contradiction asked as its own question", content.Title)
	}
	// Both ways out start every worktree that was selected: the run is what was
	// just asked for, and exclusive is what cannot be applied to it.
	for _, option := range content.Options {
		if concurrencyOf(option.Value) != domain.ConcurrencyParallel {
			t.Errorf("option %q resolves to %q, want every way out to start them all",
				option.Label, concurrencyOf(option.Value))
		}
	}
}

// A single-worktree run against the same setting is not a contradiction: it is
// exactly what the setting asked for.
func TestASettledExclusiveStandsOnASingleWorktreeRun(t *testing.T) {
	f := questionWith(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, running(here, "/wt/other"))
	answers := flow.Answers{}.WithValues("run.worktree", []string{here})

	if skip, reason := f.Step().Skip(answers); !skip || reason != domain.RunConcurrencySkipSettled {
		t.Errorf("Skip = (%v, %q), want the settled answer to stand", skip, reason)
	}
	if got := f.Decided(answers); got != domain.ConcurrencyExclusive {
		t.Errorf("concurrency = %q, want the setting applied", got)
	}
}

// Where nobody can be asked, the safe default destroys nothing.
func TestAnUnattendedContradictionStopsNothing(t *testing.T) {
	f := questionWith(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}}, running("/wt/other"))
	answers := flow.Answers{}.WithValues("run.worktree", []string{here, "/wt/second"})

	answer, err := f.Step().Resolve(answers)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if concurrencyOf(answer.Value) != domain.ConcurrencyParallel {
		t.Errorf("Resolve = %q, want the answer that stops nothing", answer.Value)
	}
}

// `run up A B` must not offer to stop B's own jobs on A's behalf.
func TestTheOtherWorktreesAreMeasuredAgainstTheWholeSelection(t *testing.T) {
	f := questionWith(request{}, running(here, "/wt/second"))
	answers := flow.Answers{}.WithValues("run.worktree", []string{here, "/wt/second"})

	if others := f.otherWorktrees(answers); len(others) != 0 {
		t.Errorf("others = %v, want nothing outside the selection", others)
	}
}

// A default that goes unsaid is a default nobody can correct.
func TestTheUnattendedContradictionIsAnnounced(t *testing.T) {
	recorder := &flowtest.Recorder{}
	f := questionFor(questionParams{
		Request:   request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}},
		Presenter: recorder,
	})
	answers := flow.Answers{}.WithValues("run.worktree", []string{here, "/wt/second"})

	f.noticeOverridden(answers)

	if len(recorder.Statuses) != 1 {
		t.Fatalf("statuses = %v, want the setting's suspension announced once", recorder.Statuses)
	}
	if !strings.Contains(recorder.Statuses[0].Text, string(domain.ConcurrencyExclusive)) {
		t.Errorf("status = %q, want it to name the setting it set aside", recorder.Statuses[0].Text)
	}
}

// Someone answered, so there is nothing to announce in their place.
func TestAnAnsweredContradictionIsNotAnnounced(t *testing.T) {
	recorder := &flowtest.Recorder{}
	f := questionFor(questionParams{
		Request:   request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyExclusive}},
		Presenter: recorder,
	})
	answers := flow.Answers{}.
		WithValues("run.worktree", []string{here, "/wt/second"}).
		With(Key, flow.Answer{Value: answerParallel, Asked: true})

	f.noticeOverridden(answers)

	if len(recorder.Statuses) != 0 {
		t.Errorf("statuses = %v, want nothing said over an answer someone gave", recorder.Statuses)
	}
}

// clashFlow runs `web` here while the same job is up in /wt/other, the two
// worktrees on the offsets given — a verbatim worktree shares its source's.
func clashFlow(req request, otherOffset int) *Question {
	req.Config.Jobs = []domain.JobConfig{{
		Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev",
		Ports: map[string]int{"PORT": 3000},
	}}
	f := questionWith(req, []domain.JobInfo{{Name: "web", WorkDir: "/wt/other", Status: domain.JobStatusRunning}})
	f.offsets = map[string]int{here: 0, "/wt/other": otherOffset}
	return f
}

// Running side by side is not an answer when the ports are the same: the
// question becomes stop the other one, or don't start — whatever the project
// settled as its preference.
func TestAPortClashOverridesAParallelPreference(t *testing.T) {
	f := clashFlow(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyParallel}}, 0)
	step := f.Step()

	if skip, reason := step.Skip(flow.Answers{}); skip {
		t.Fatalf("skipped (%s), want the clash put to the user", reason)
	}
	content, err := step.Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if content.Title != domain.RunPortClashTitle || !strings.Contains(content.Description, "3000") {
		t.Errorf("content = %+v, want the clash named with its port", content)
	}
	if len(content.Options) != 2 || content.Options[1].Value != answerCancel {
		t.Errorf("options = %+v, want stop-the-other or don't start", content.Options)
	}
}

// Nobody to ask: stopping another worktree is not a default to take silently,
// and running both is not possible — the run refuses, naming the way out.
func TestAPortClashRefusesAnUnattendedRun(t *testing.T) {
	_, err := clashFlow(request{}, 0).Step().Resolve(flow.Answers{})
	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagExclusive) {
		t.Errorf("err = %v, want a refusal naming --%s", err, domain.FlagExclusive)
	}
}

func TestAPortClashIsSettledByExclusive(t *testing.T) {
	step := clashFlow(request{Exclusive: true}, 0).Step()
	if skip, _ := step.Skip(flow.Answers{}); !skip {
		t.Error("--exclusive answers the clash, want the step skipped")
	}
	answer, err := step.Resolve(flow.Answers{})
	if err != nil || answer.Value != answerExclusive {
		t.Errorf("Resolve = (%q, %v), want exclusive", answer.Value, err)
	}
}

// Isolated worktrees sit a block apart: the same job up next door is not a clash.
func TestIsolatedWorktreesDoNotClash(t *testing.T) {
	f := clashFlow(request{Config: domain.RunConfig{Concurrency: domain.ConcurrencyParallel}}, 10)
	if clashes := f.clashes(flow.Answers{}); len(clashes) != 0 {
		t.Errorf("clashes = %+v, want none a block apart", clashes)
	}
	if skip, _ := f.Step().Skip(flow.Answers{}); !skip {
		t.Error("the settled preference applies when nothing clashes")
	}
}

// The clash's stop answer is the exclusive one, which stops every other
// worktree: the option names each of them, not only the one holding the port.
func TestAPortClashNamesEveryWorktreeItsAnswerStops(t *testing.T) {
	f := clashFlow(request{}, 0)
	f.params.Running = append(f.params.Running, domain.JobInfo{Name: "db", WorkDir: "/wt/bystander", Status: domain.JobStatusRunning})
	f.offsets["/wt/bystander"] = 20

	content, err := f.Step().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	stop := content.Options[0]
	if stop.Value != answerExclusive {
		t.Fatalf("first option = %+v, want the exclusive answer", stop)
	}
	for _, name := range []string{"other", "bystander"} {
		if !strings.Contains(stop.Label, name) {
			t.Errorf("stop option %q does not name %s, which it stops", stop.Label, name)
		}
	}
}
