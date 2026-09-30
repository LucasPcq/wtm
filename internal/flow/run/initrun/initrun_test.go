package initrun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

type recorder struct {
	*flowtest.Recorder
	outcomes []Outcome
}

func (r *recorder) Initialized(outcome Outcome) error {
	r.outcomes = append(r.outcomes, outcome)
	return nil
}

type fakeWizard struct {
	answer    func(Question) (domain.InitProjectAnswers, error)
	questions []Question
}

func (w *fakeWizard) AskServices(question Question) (domain.InitProjectAnswers, error) {
	w.questions = append(w.questions, question)
	return w.answer(question)
}

func detectionDefaults(question Question) (domain.InitProjectAnswers, error) {
	return rules.AutoServicesAnswers(rules.AutoServicesAnswersParams{Detection: question.Detection}), nil
}

func fixture(t *testing.T) flow.Context {
	t.Helper()
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	files := map[string]string{
		"package.json":          `{"name":"root","private":true}`,
		"pnpm-workspace.yaml":   "packages:\n  - \"apps/*\"\n",
		"pnpm-lock.yaml":        "lockfileVersion: '9.0'\n",
		"apps/api/package.json": `{"name":"api","scripts":{"dev":"tsx watch src/index.ts"}}`,
		"apps/api/.env":         "PORT=4001\n",
		"apps/web/package.json": `{"name":"web","scripts":{"dev":"vite"}}`,
		"apps/web/.env":         "VITE_PORT=5173\nVITE_API_URL=http://localhost:4001\n",
	}
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := domain.Config{}
	cfg.Project.Env.Files = []domain.EnvFile{{Target: "apps/api/.env"}, {Target: "apps/web/.env"}}
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: cfg}
}

func run(t *testing.T, params Params) (Outcome, *recorder) {
	t.Helper()
	presenter := &recorder{Recorder: &flowtest.Recorder{}}
	params.Presenter = presenter
	outcome, err := Run(params)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return outcome, presenter
}

func runTOMLExists(ctx flow.Context) bool {
	_, err := os.Stat(filepath.Join(ctx.StateDir, domain.RunFileName))
	return err == nil
}

func TestUnattendedConfiguresFromDetectionWithoutTheWizard(t *testing.T) {
	ctx := fixture(t)
	wizard := &fakeWizard{answer: detectionDefaults}

	outcome, presenter := run(t, Params{Context: ctx, Prompter: flow.Unattended{}, Wizard: wizard})

	if len(wizard.questions) != 0 {
		t.Errorf("wizard asked %d time(s), want never on an unattended run", len(wizard.questions))
	}
	if len(presenter.Stages) != 1 || presenter.Stages[0] != domain.RunInitDetectingMessage {
		t.Errorf("stages = %v, want the detection stage", presenter.Stages)
	}
	if outcome.Report.Added != 2 || !runTOMLExists(ctx) {
		t.Errorf("added = %d, run.toml written = %v, want both dev servers", outcome.Report.Added, runTOMLExists(ctx))
	}
	if len(outcome.Report.Links) != 0 {
		t.Errorf("links = %+v, want none without --link-env", outcome.Report.Links)
	}
	if len(presenter.outcomes) != 1 {
		t.Errorf("conclusions = %d, want one", len(presenter.outcomes))
	}
}

func TestInteractiveAsksTheWizardThenTheLinks(t *testing.T) {
	ctx := fixture(t)
	wizard := &fakeWizard{answer: detectionDefaults}
	prompter := &flowtest.ScriptedPrompter{Confirmed: true}

	outcome, _ := run(t, Params{Context: ctx, Prompter: prompter, Wizard: wizard})

	if len(wizard.questions) != 1 || wizard.questions[0].Prefill != nil {
		t.Fatalf("questions = %+v, want one, unprefilled on a first init", wizard.questions)
	}
	if prompter.Confirms != 1 || len(outcome.Report.Links) == 0 {
		t.Errorf("confirms = %d, links = %+v, want the links offered and taken", prompter.Confirms, outcome.Report.Links)
	}
}

func TestADeclinedLinkQuestionWritesNoLink(t *testing.T) {
	ctx := fixture(t)
	prompter := &flowtest.ScriptedPrompter{Confirmed: false}

	outcome, _ := run(t, Params{Context: ctx, Prompter: prompter, Wizard: &fakeWizard{answer: detectionDefaults}})

	if prompter.Confirms != 1 || len(outcome.Report.Links) != 0 {
		t.Errorf("confirms = %d, links = %+v, want asked and declined", prompter.Confirms, outcome.Report.Links)
	}
}

func TestTheWizardsOwnLinkAnswerIsNotAskedTwice(t *testing.T) {
	ctx := fixture(t)
	prompter := &flowtest.ScriptedPrompter{Confirmed: true}
	wizard := &fakeWizard{answer: func(question Question) (domain.InitProjectAnswers, error) {
		answers, _ := detectionDefaults(question)
		answers.EnvLinksAsked, answers.LinkEnv = true, false
		return answers, nil
	}}

	outcome, _ := run(t, Params{Context: ctx, Prompter: prompter, Wizard: wizard})

	if prompter.Confirms != 0 || len(outcome.Report.Links) != 0 {
		t.Errorf("confirms = %d, links = %+v, want the wizard's refusal to stand", prompter.Confirms, outcome.Report.Links)
	}
}

func TestBackingOutOfTheWizardWritesNothing(t *testing.T) {
	ctx := fixture(t)
	wizard := &fakeWizard{answer: func(Question) (domain.InitProjectAnswers, error) {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}}

	outcome, presenter := run(t, Params{Context: ctx, Prompter: &flowtest.ScriptedPrompter{}, Wizard: wizard})

	if !outcome.Aborted || runTOMLExists(ctx) || len(presenter.outcomes) != 0 {
		t.Errorf("aborted = %v, run.toml = %v, conclusions = %d, want an abort that writes and concludes nothing",
			outcome.Aborted, runTOMLExists(ctx), len(presenter.outcomes))
	}
	if len(presenter.Notices) != 1 || !presenter.Notices[0].IsAbort() {
		t.Errorf("notices = %+v, want the abort said like every other flow's", presenter.Notices)
	}
}

func TestAReInitOpensTheWizardOnWhatRunTOMLDeclares(t *testing.T) {
	ctx := fixture(t)
	run(t, Params{Context: ctx, Prompter: flow.Unattended{}, Wizard: &fakeWizard{answer: detectionDefaults}})
	wizard := &fakeWizard{answer: detectionDefaults}

	run(t, Params{Context: ctx, Prompter: &flowtest.ScriptedPrompter{}, Wizard: wizard})

	if len(wizard.questions) != 1 || wizard.questions[0].Prefill == nil {
		t.Fatalf("questions = %+v, want a prefilled re-init", wizard.questions)
	}
	if got := len(wizard.questions[0].Prefill.ScriptIndices); got != 2 {
		t.Errorf("prefilled scripts = %d, want the two configured ones", got)
	}
}

func TestNothingDetectedIsReportedAndWritesNothing(t *testing.T) {
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	ctx := flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm")}

	outcome, presenter := run(t, Params{Context: ctx, Prompter: flow.Unattended{}})

	if !outcome.NothingDetected || runTOMLExists(ctx) || len(presenter.outcomes) != 1 {
		t.Errorf("nothing detected = %v, run.toml = %v, conclusions = %d", outcome.NothingDetected, runTOMLExists(ctx), len(presenter.outcomes))
	}
}
