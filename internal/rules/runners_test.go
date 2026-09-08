package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// turboConfig is the shape this relation exists for: one root script that fans
// out into three apps, each holding its own port.
func turboConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{
			Name: "dev:crm", Kind: domain.JobKindService, Cmd: "pnpm run dev:crm",
			Runs: []string{"crm-web-dev", "crm-api-dev"},
		},
		{Name: "crm-web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/crm/web", Ports: map[string]int{"VITE_PORT": 5175}},
		{Name: "crm-api-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/crm/api", Ports: map[string]int{"PORT": 4002}},
	}}
}

func TestRunnerChildrenNamesWhatTheRunnerStarts(t *testing.T) {
	children := RunnerChildren(turboConfig(), "dev:crm")

	if len(children) != 2 || children[0] != "crm-web-dev" || children[1] != "crm-api-dev" {
		t.Fatalf("got %+v", children)
	}
}

func TestRunnerChildrenFollowsTheRelationTransitively(t *testing.T) {
	cfg := turboConfig()
	cfg.Jobs[1].Runs = []string{"crm-api-dev"}

	if children := RunnerChildren(cfg, "dev:crm"); len(children) != 2 {
		t.Fatalf("got %+v", children)
	}
}

func TestRunnersOfNamesWhatWouldStartThisJob(t *testing.T) {
	if runners := RunnersOf(turboConfig(), "crm-web-dev"); len(runners) != 1 || runners[0] != "dev:crm" {
		t.Fatalf("got %+v", runners)
	}
}

func TestEffectiveJobPortsGivesARunnerItsChildrensPorts(t *testing.T) {
	cfg := turboConfig()

	ports := EffectiveJobPorts(cfg, cfg.Jobs[0])
	if ports["VITE_PORT"] != 5175 || ports["PORT"] != 4002 {
		t.Fatalf("got %+v", ports)
	}
}

func TestEffectiveJobPortsLeavesAnAmbiguousNameUnresolved(t *testing.T) {
	cfg := turboConfig()
	cfg.Jobs[1].Ports = map[string]int{"PORT": 5175}

	ports := EffectiveJobPorts(cfg, cfg.Jobs[0])
	if _, resolved := ports["PORT"]; resolved {
		t.Fatalf("two children declare PORT at different bases, so the name has no answer: %+v", ports)
	}
}

func TestEffectiveJobPortsKeepsTheRunnersOwnDeclaration(t *testing.T) {
	cfg := turboConfig()
	cfg.Jobs[0].Ports = map[string]int{"VITE_PORT": 9999}

	if ports := EffectiveJobPorts(cfg, cfg.Jobs[0]); ports["VITE_PORT"] != 9999 {
		t.Fatalf("what the reader wrote about this job outranks what it inherits: %+v", ports)
	}
}

func TestEffectiveJobPortsLeavesAnOrdinaryJobAlone(t *testing.T) {
	cfg := turboConfig()

	if ports := EffectiveJobPorts(cfg, cfg.Jobs[1]); ports["VITE_PORT"] != 5175 || len(ports) != 1 {
		t.Fatalf("got %+v", ports)
	}
}

func TestJobBindsNothingCoversARunnerAndADeclaredOne(t *testing.T) {
	cfg := turboConfig()
	if !JobBindsNothing(cfg, cfg.Jobs[0]) {
		t.Fatal("a runner holding no port of its own binds nothing")
	}
	if JobBindsNothing(cfg, cfg.Jobs[1]) {
		t.Fatal("a job with a port binds it")
	}

	watch := domain.JobConfig{Name: "ui-watch", Kind: domain.JobKindService, Cmd: "tsc --watch", BindsNoPort: true}
	if !JobBindsNothing(domain.RunConfig{Jobs: []domain.JobConfig{watch}}, watch) {
		t.Fatal("a build in watch mode said so itself")
	}
}

func TestStartConflictsCatchesARunnerStartedWithItsChild(t *testing.T) {
	conflicts := StartConflicts(StartConflictsParams{
		Config:   turboConfig(),
		Starting: []string{"dev:crm", "crm-web-dev"},
	})

	if len(conflicts) != 1 || conflicts[0].Job != "crm-web-dev" || conflicts[0].Runner != "dev:crm" {
		t.Fatalf("got %+v", conflicts)
	}
}

func TestStartConflictsIsSilentOnAChildStartedAlone(t *testing.T) {
	conflicts := StartConflicts(StartConflictsParams{
		Config:   turboConfig(),
		Starting: []string{"crm-web-dev", "crm-api-dev"},
	})

	if len(conflicts) != 0 {
		t.Fatalf("two children are what the runner would have started anyway: %+v", conflicts)
	}
}

func TestServicesWithoutPortsIgnoresAServiceThatSaidItBindsNothing(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "ui-watch", Kind: domain.JobKindService, Cmd: "tsc --watch", BindsNoPort: true},
		{Name: "web", Kind: domain.JobKindService, Cmd: "vite"},
	}}

	jobs := ServicesWithoutPorts(cfg)
	if len(jobs) != 1 || jobs[0] != "web" {
		t.Fatalf("got %+v — a job that answered is not a job that forgot", jobs)
	}
}

func TestServicesWithoutPortsIgnoresARunner(t *testing.T) {
	if jobs := ServicesWithoutPorts(turboConfig()); len(jobs) != 0 {
		t.Fatalf("got %+v — the runner's children hold the ports", jobs)
	}
}

func TestPortEntriesForKeepsTheRowOfAServiceBindingNothing(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "ui-watch", Kind: domain.JobKindService, Cmd: "tsc --watch", BindsNoPort: true},
	}}

	entries := PortEntriesFor(PortEntriesForParams{Config: cfg})
	if len(entries) != 1 || !entries[0].BindsNone {
		t.Fatalf("got %+v — a re-init shows the complete list, pre-filled with what was settled", entries)
	}
	if entries[0].Name == "" {
		t.Fatal("the row must keep something to declare if the answer is taken back")
	}
}

func TestApplyInitAnswersRecordsAndLiftsTheAnswer(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "x"}}}

	answered := ApplyInitAnswers(ApplyInitAnswersParams{
		Config: cfg,
		Ports:  []domain.PortEntry{{Job: "dev:crm", Name: "PORT", BindsNone: true}},
	})
	if !answered.Jobs[0].BindsNoPort {
		t.Fatal("the answer must reach run.toml")
	}

	lifted := ApplyInitAnswers(ApplyInitAnswersParams{
		Config: answered,
		Ports:  []domain.PortEntry{{Job: "dev:crm", Name: "PORT", Base: 3000}},
	})
	if lifted.Jobs[0].BindsNoPort || lifted.Jobs[0].Ports["PORT"] != 3000 {
		t.Fatalf("declaring a port takes the answer back: %+v", lifted.Jobs[0])
	}
}

// The recap warns about what will still collide. A row the reader answered is
// not one of those, which is the whole point of being able to answer it.
func TestTheRecapStopsWarningAboutAnAnsweredService(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "x"}}}

	if len(ServicesWithoutPorts(cfg)) != 1 {
		t.Fatal("unanswered, it is a warning")
	}

	written := ApplyInitAnswers(ApplyInitAnswersParams{
		Config: cfg,
		Ports:  []domain.PortEntry{{Job: "dev:crm", Name: "PORT", BindsNone: true}},
	})
	if warned := ServicesWithoutPorts(written); len(warned) != 0 {
		t.Fatalf("answered, it is not: %+v", warned)
	}
}

func TestTheRecapStopsWarningAboutARunner(t *testing.T) {
	cfg := monorepoConfig()
	choices := RunnerChoices(RunnerChoicesParams{Config: cfg, ComposeJobs: []string{"docker-compose"}})
	setRunner(t, choices, "crm-web-dev", "dev:crm")

	written := ApplyInitAnswers(ApplyInitAnswersParams{Config: cfg, Runners: choices})
	for _, name := range ServicesWithoutPorts(written) {
		if name == "dev:crm" {
			t.Fatal("its children hold the ports")
		}
	}
}

func TestPortEntriesForOffersTheAnswerOnlyWhereItMeansSomething(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"DB_PORT": 5432}},
		{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "y"},
	}}

	for _, entry := range PortEntriesFor(PortEntriesForParams{Config: cfg}) {
		if entry.Job == "api" && entry.CanBindNone {
			t.Fatalf("a job declaring a port cannot also bind none: %+v", entry)
		}
		if entry.Job == "dev:crm" && !entry.CanBindNone {
			t.Fatalf("a job declaring nothing is exactly the one that can answer: %+v", entry)
		}
	}
}

func TestEffectiveJobPortsKeepsAPortTwoChildrenAgreeOn(t *testing.T) {
	cfg := turboConfig()
	cfg.Jobs[1].Ports = map[string]int{"VITE_PORT": 5175, "DB_PORT": 5432}
	cfg.Jobs[2].Ports = map[string]int{"PORT": 4002, "DB_PORT": 5432}

	ports := EffectiveJobPorts(cfg, cfg.Jobs[0])
	if ports["DB_PORT"] != 5432 {
		t.Fatalf("two children naming the same base agree; there is nothing to arbitrate: %+v", ports)
	}
}

func TestRunnerChildrenIgnoresAStaleReference(t *testing.T) {
	cfg := turboConfig()
	cfg.Jobs[0].Runs = []string{"crm-web-dev", "renamed-away"}

	children := RunnerChildren(cfg, "dev:crm")
	if len(children) != 1 || children[0] != "crm-web-dev" {
		t.Fatalf("a name matching no job must not invent a child: %+v", children)
	}
}

func TestJobBindsNothingIsNotEarnedByAStaleReference(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "x", Runs: []string{"renamed-away"}},
	}}

	if JobBindsNothing(cfg, cfg.Jobs[0]) {
		t.Fatal("its children are gone, so it holds no port and nothing holds one for it")
	}
	if len(ServicesWithoutPorts(cfg)) != 1 {
		t.Fatal("and the report must still name it")
	}
}

func TestStartConflictsCatchesARunnerAlreadyUp(t *testing.T) {
	conflicts := StartConflicts(StartConflictsParams{
		Config:   turboConfig(),
		Starting: []string{"crm-web-dev", "dev:crm", "crm-web-dev"},
	})

	if len(conflicts) != 1 {
		t.Fatalf("one pair, named once: %+v", conflicts)
	}
}
