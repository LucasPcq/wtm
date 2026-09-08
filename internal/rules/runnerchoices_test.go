package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// monorepoConfig is the shape the step exists for: two root scripts that fan
// out, and the app scripts they start.
func monorepoConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: ".", Ports: map[string]int{"DB_PORT": 5432}},
		{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "pnpm run dev:crm", Cwd: "."},
		{Name: "dev:shop", Kind: domain.JobKindService, Cmd: "pnpm run dev:shop", Cwd: "."},
		{Name: "crm-web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/crm/web", Ports: map[string]int{"VITE_PORT": 5175}},
		{Name: "seed", Kind: domain.JobKindTask, Cmd: "pnpm run seed", Cwd: "apps/crm/api"},
	}}
}

// runnerRowOf is the row the step offered for one job, failing rather than
// indexing: the rows now cover the candidates too, so their order is no longer
// something a test may assume.
func runnerRowOf(t *testing.T, choices []domain.JobRunnerChoice, job string) domain.JobRunnerChoice {
	t.Helper()
	for _, choice := range choices {
		if choice.Job == job {
			return choice
		}
	}
	t.Fatalf("no row for %q in %+v", job, choices)
	return domain.JobRunnerChoice{}
}

func setRunner(t *testing.T, choices []domain.JobRunnerChoice, job, runner string) {
	t.Helper()
	for i, choice := range choices {
		if choice.Job != job {
			continue
		}
		choices[i].Runners = nil
		if runner != "" {
			choices[i].Runners = []string{runner}
		}
		return
	}
	t.Fatalf("no row for %q", job)
}

func TestRunnerCandidatesAreTheRootServicesHoldingNoPort(t *testing.T) {
	names := RunnerCandidates(RunnerChoicesParams{Config: monorepoConfig(), ComposeJobs: []string{"docker-compose"}})

	if len(names) != 2 || names[0] != "dev:crm" || names[1] != "dev:shop" {
		t.Fatalf("got %+v", names)
	}
}

func TestRunnerChoicesOfferOneRowPerNestedService(t *testing.T) {
	choices := RunnerChoices(RunnerChoicesParams{Config: monorepoConfig(), ComposeJobs: []string{"docker-compose"}})

	// A task binds nothing and a compose stack is a tree of its own, so neither
	// is a row. A root runner is: `dev` fanning out to `dev:crm` and `dev:shop`
	// is the shape a turborepo takes, and it cannot be said any other way.
	if len(choices) != 3 {
		t.Fatalf("got %+v, want a row per nested service and per candidate", choices)
	}
	byJob := map[string]domain.JobRunnerChoice{}
	for _, choice := range choices {
		byJob[choice.Job] = choice
	}
	app, offered := byJob["crm-web-dev"]
	if !offered {
		t.Fatal("the service the step exists for has no row")
	}
	if len(app.Runners) != 0 {
		t.Fatal("no relation is the default: which command fans out is the one thing wtm refuses to infer")
	}
	if len(app.Options) != 3 || app.Options[0] != "" {
		t.Fatalf("options = %+v, want none first then both candidates", app.Options)
	}
	// A candidate is never offered itself, so its row is one shorter.
	if got := byJob["dev:crm"].Options; len(got) != 2 || got[1] != "dev:shop" {
		t.Fatalf("dev:crm options = %+v, want none and the other candidate", got)
	}
}

func TestRunnerChoicesNeverOfferARunnerAChainAlreadyReaches(t *testing.T) {
	cfg := monorepoConfig()
	// A third root, so dev:shop still has something to be offered once it runs
	// dev:crm — otherwise its row disappears for want of any answer at all.
	cfg.Jobs = append(cfg.Jobs, domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "."})
	cfg.Jobs[2].Runs = []string{"dev:crm"} // dev:shop runs dev:crm

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	for _, choice := range choices {
		if choice.Job != "dev:shop" {
			continue
		}
		for _, option := range choice.Options {
			if option == "dev:crm" {
				t.Fatal("a runner offered a parent it already starts would write a cycle run.toml refuses")
			}
		}
		return
	}
	t.Fatal("dev:shop has no row")
}

func TestRunnerChoicesAreEmptyWithoutACandidate(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "vite", Cwd: "apps/web"},
	}}

	if choices := RunnerChoices(RunnerChoicesParams{Config: cfg}); len(choices) != 0 {
		t.Fatalf("got %+v", choices)
	}
}

func TestRunnerChoicesArePreFilledFromTheConfig(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[1].Runs = []string{"crm-web-dev"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	if got := runnerRowOf(t, choices, "crm-web-dev").Runners; len(got) != 1 || got[0] != "dev:crm" {
		t.Fatalf("a re-init shows what was settled: %+v", got)
	}
}

func TestApplyRunnerChoicesWritesTheRelation(t *testing.T) {
	cfg := monorepoConfig()
	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "dev:crm")

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	crm, _ := FindJob(out, "dev:crm")
	if len(crm.Runs) != 1 || crm.Runs[0] != "crm-web-dev" {
		t.Fatalf("got %+v", crm.Runs)
	}
	if _, errs := ValidateRun(out); len(errs) != 0 {
		t.Fatalf("the relation it writes must load back: %v", errs)
	}
}

func TestApplyRunnerChoicesWithdrawsWhatARowGaveBack(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[1].Runs = []string{"crm-web-dev"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "")

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	if crm, _ := FindJob(out, "dev:crm"); len(crm.Runs) != 0 {
		t.Fatalf("a row set back to none withdraws the relation: %+v", crm.Runs)
	}
}

func TestApplyRunnerChoicesLeavesAJobTheStepNeverOffered(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[0].Runs = []string{"crm-web-dev"}

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})})
	if compose, _ := FindJob(out, "docker-compose"); len(compose.Runs) != 1 {
		t.Fatalf("the compose job was never a candidate, so the step does not speak for it: %+v", compose.Runs)
	}
}

func TestPortEntriesForFollowsTheRelationTheStepJustSettled(t *testing.T) {
	cfg := monorepoConfig()
	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "dev:crm")

	entries := PortEntriesFor(PortEntriesForParams{
		Config:      ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices}),
		ComposeJobs: []string{"docker-compose"},
	})

	for _, entry := range entries {
		if entry.Job == "dev:crm" && !entry.BindsNone {
			t.Fatalf("a runner is not a service that forgot a port: %+v", entry)
		}
		if entry.Job == "dev:shop" && entry.BindsNone {
			t.Fatalf("dev:shop runs nothing, so its row is still a question: %+v", entry)
		}
	}
}

func TestApplyInitAnswersWritesTheAddressingTheStepSettled(t *testing.T) {
	cfg := domain.RunConfig{Addressing: domain.AddressingNames}

	out := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:          cfg,
		Addressing:      domain.AddressingPorts,
		AddressingAsked: true,
	})
	if out.Addressing != domain.AddressingPorts {
		t.Fatalf("got %q", out.Addressing)
	}
}

func TestApplyInitAnswersLeavesTheAddressingAStepNeverAskedAbout(t *testing.T) {
	cfg := domain.RunConfig{Addressing: domain.AddressingPorts}

	if out := ApplyInitAnswers(ApplyInitAnswersParams{Config: cfg}); out.Addressing != domain.AddressingPorts {
		t.Fatalf("got %q — a run that never asked must not write the default over the file", out.Addressing)
	}
}

func TestAddressingChoicesOpenOnTheModeTheProjectIsOn(t *testing.T) {
	if got := AddressingChoices(domain.AddressingNames); got[0] != domain.AddressingNames {
		t.Fatalf("got %+v", got)
	}
	if got := AddressingChoices(domain.AddressingPorts); got[0] != domain.AddressingPorts {
		t.Fatalf("got %+v", got)
	}
}

func TestAddressingStepSaysWhatNamedUrlsCost(t *testing.T) {
	if !strings.Contains(domain.AddressingStepDesc, "`wtm run`") {
		t.Fatal("the step must say that named urls answer only while wtm runs the job")
	}
}

func TestAnyJobPublishesANameReadsTheConfigAsItWillBeWritten(t *testing.T) {
	bare := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}}}
	if AnyJobPublishesAName(bare) {
		t.Fatal("no job publishes a name, so the mode changes nothing")
	}

	published := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, URL: &domain.JobURLConfig{Port: "PORT"}}}}
	if !AnyJobPublishesAName(published) {
		t.Fatal("a published job is addressed by the mode")
	}
}

func TestScriptsStepDescriptionWarnsOnlyWhereTheTrapExists(t *testing.T) {
	flat := []domain.PackageScript{{Name: "dev"}}
	if strings.Contains(ScriptsStepDescription(flat), domain.MonorepoRootHint) {
		t.Fatal("a single-package repo has no root-versus-app choice to make")
	}

	monorepo := append(flat, domain.PackageScript{Name: "dev", Workspace: "apps/web"})
	if !strings.Contains(ScriptsStepDescription(monorepo), domain.MonorepoRootHint) {
		t.Fatal("the trap is sprung here and the step must say so")
	}

	nestedOnly := []domain.PackageScript{{Name: "dev", Workspace: "apps/web"}}
	if strings.Contains(ScriptsStepDescription(nestedOnly), domain.MonorepoRootHint) {
		t.Fatal("no root script, nothing to warn about")
	}
}

func TestApplyRunnerChoicesKeepsAChildTheStepNeverOffered(t *testing.T) {
	cfg := monorepoConfig()
	// `seed` is a task: the step never rows it, so it never speaks for it.
	cfg.Jobs[1].Runs = []string{"seed"}

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "dev:crm")

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	crm, _ := FindJob(out, "dev:crm")
	if len(crm.Runs) != 2 {
		t.Fatalf("got %+v, want the hand-written child kept alongside the answered one", crm.Runs)
	}
	if crm.Runs[0] != "seed" {
		t.Fatalf("got %+v", crm.Runs)
	}
}

// run.toml has always allowed two roots to start the same app. The step used to
// read one parent and write the row back over both, so a re-init silently took
// the app out of the second runner.
func TestApplyRunnerChoicesKeepsARowsSecondRunner(t *testing.T) {
	cfg := monorepoConfig()
	cfg.Jobs[1].Runs = []string{"crm-web-dev"} // dev:crm
	cfg.Jobs[2].Runs = []string{"crm-web-dev"} // dev:shop

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	if got := runnerRowOf(t, choices, "crm-web-dev").Runners; len(got) != 2 {
		t.Fatalf("row = %+v, want both runners shown", got)
	}

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	for _, runner := range []string{"dev:crm", "dev:shop"} {
		job, _ := FindJob(out, runner)
		if len(job.Runs) != 1 || job.Runs[0] != "crm-web-dev" {
			t.Errorf("%s runs = %+v, want the relation left as it was", runner, job.Runs)
		}
	}
	if _, errs := ValidateRun(out); len(errs) != 0 {
		t.Fatalf("what it writes must load back: %v", errs)
	}
}

func TestApplyRunnerChoicesWritesTheChainARowComposed(t *testing.T) {
	cfg := monorepoConfig()

	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "dev:crm")
	setRunner(t, choices, "dev:crm", "dev:shop")

	out := ApplyRunnerChoices(ApplyRunnerChoicesParams{Config: cfg, Choices: choices})
	if _, errs := ValidateRun(out); len(errs) != 0 {
		t.Fatalf("a chain must load back: %v", errs)
	}
	// What the root holds is read transitively, which is the whole reason a
	// runner may be another's child: one row per level, and the top one starts
	// everything below it.
	children := RunnerChildren(out, "dev:shop")
	if len(children) != 2 || children[0] != "dev:crm" || children[1] != "crm-web-dev" {
		t.Fatalf("children = %+v, want the chain walked to the app", children)
	}
}
