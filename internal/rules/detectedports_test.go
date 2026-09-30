package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func composeAnswers(files ...string) domain.InitProjectAnswers {
	return domain.InitProjectAnswers{
		DockerComposeFiles: files,
		DockerComposeCmd:   "docker compose",
		PatchCompose:       true,
	}
}

func frozenAt(file, service, name string, base int) domain.ComposePortBinding {
	return domain.ComposePortBinding{
		File: file, Service: service, Status: domain.ComposePortFrozen,
		Var: name, Base: base, Token: "x", Replacement: "y",
	}
}

// TestResolveDetectedPortsNeverRewritesForAPortItThenWithdraws is the ordering
// the whole resolver exists for: two bases a block apart lose their
// declarations, so neither mapping may be rewritten in the project's file.
func TestResolveDetectedPortsNeverRewritesForAPortItThenWithdraws(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "storefront", "STOREFRONT_PORT", 3000),
			frozenAt(file, "admin", "ADMIN_PORT", 3010),
		}}},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers: composeAnswers(file),
		Plan:    plan,
	})

	if len(got.Dropped) != 2 {
		t.Fatalf("both bases must be withdrawn, got %+v", got.Dropped)
	}
	if len(got.Patches) != 0 {
		t.Errorf("no file may be rewritten for a port that is not declared, got %v", got.Patches)
	}
	if len(got.Written) != 0 {
		t.Errorf("written = %v, want nothing", got.Written)
	}
}

func TestResolveDetectedPortsRewritesOnlyTheSurvivingMapping(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
			frozenAt(file, "admin", "ADMIN_PORT", 3010),
		}}},
	})

	// A hand-written 3000 outranks the detected 3010 and takes it down with it.
	existing := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
	}}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  composeAnswers(file),
		Existing: existing,
		Plan:     plan,
	})

	if len(got.Patches[file]) != 1 || got.Patches[file][0].Var != "POSTGRES_PORT" {
		t.Fatalf("only the surviving mapping is rewritten, got %+v", got.Patches[file])
	}
	if len(got.Dropped) != 1 || got.Dropped[0].Port.Name != "ADMIN_PORT" {
		t.Errorf("dropped = %+v", got.Dropped)
	}
}

func TestResolveDetectedPortsWithdrawsAFileThatChangedUnderIt(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
		}}},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:      composeAnswers(file),
		Plan:         plan,
		Unverifiable: map[string]string{file: "the file changed since it was read"},
	})

	if len(got.Patches) != 0 || len(got.Written) != 0 {
		t.Errorf("a stale scan contributes nothing, got patches=%v written=%v", got.Patches, got.Written)
	}
	if got.Changed[file] == "" {
		t.Error("the reason must reach the report")
	}
	// The run still does its job: the compose file gets its runner.
	if len(got.Config.Jobs) != 1 {
		t.Errorf("jobs = %+v, want the run to carry on", got.Config.Jobs)
	}
}

// TestResolveDetectedPortsReportsAFileWithNoJob covers the silent case: a job
// name already taken by another file leaves the new one with no runner, so its
// ports have nowhere to go.
func TestResolveDetectedPortsReportsAFileWithNoJob(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "postgres", "POSTGRES_PORT", 5432),
		}}},
	})

	existing := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService, Cmd: "docker compose -f infra/docker-compose.yml up -d"},
	}}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  composeAnswers(file),
		Existing: existing,
		Plan:     plan,
	})

	if len(got.Orphaned) != 1 || got.Orphaned[0] != file {
		t.Fatalf("orphaned = %v, want %s named", got.Orphaned, file)
	}
	if len(got.Patches) != 0 {
		t.Errorf("a file whose ports go nowhere must not be rewritten, got %v", got.Patches)
	}
	if len(got.Written) != 0 {
		t.Errorf("written = %v, want nothing", got.Written)
	}
}

func TestResolveDetectedPortsIsIdempotent(t *testing.T) {
	file := "docker-compose.yml"
	scans := map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
		{File: file, Service: "postgres", Status: domain.ComposePortTemplated, Var: "DB_PORT", Base: 5432},
	}}}
	plan := PlanComposePorts(PlanComposePortsParams{Files: []string{file}, Patch: true, Scans: scans})

	first := ResolveDetectedPorts(ResolveDetectedPortsParams{Answers: composeAnswers(file), Plan: plan})
	second := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  composeAnswers(file),
		Existing: first.Config,
		Plan:     PlanComposePorts(PlanComposePortsParams{Files: []string{file}, Patch: true, Scans: scans}),
	})

	if len(second.Config.Jobs) != len(first.Config.Jobs) {
		t.Errorf("a second run duplicated jobs: %d → %d", len(first.Config.Jobs), len(second.Config.Jobs))
	}
	if len(second.Written) != 0 {
		t.Errorf("a second run declares nothing new, got %v", second.Written)
	}
	if len(second.Patches) != 0 {
		t.Errorf("a second run rewrites nothing, got %v", second.Patches)
	}
}

// TestResolveDetectedPortsWithholdsAVariableTwoStackedFilesDisagreeOn covers the
// override pattern (-f base.yml -f dev.yml): one job receives both files' ports,
// so the same variable can arrive twice with two bases. Declaring either would
// move the other file's binding.
func TestResolveDetectedPortsWithholdsAVariableTwoStackedFilesDisagreeOn(t *testing.T) {
	base, dev := "docker-compose.yml", "docker-compose.dev.yml"
	stack := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "stack", Kind: domain.JobKindService,
		Cmd: "docker compose " + DockerComposeFileFlag(base) + DockerComposeFileFlag(dev) + "up -d",
	}}}

	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{base, dev},
		Patch: true,
		Scans: map[string]domain.ComposeScan{
			base: {File: base, Bindings: []domain.ComposePortBinding{frozenAt(base, "db", "DB_PORT", 5432)}},
			dev:  {File: dev, Bindings: []domain.ComposePortBinding{frozenAt(dev, "db", "DB_PORT", 5433)}},
		},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  composeAnswers(base, dev),
		Existing: stack,
		Plan:     plan,
	})

	if len(got.Patches) != 0 {
		t.Errorf("neither file may be rewritten, got %v", got.Patches)
	}
	if len(got.Written) != 0 {
		t.Errorf("nothing may be declared, got %v", got.Written)
	}
	if len(got.Withheld) != 2 {
		t.Fatalf("both must be reported, got %+v", got.Withheld)
	}
	for _, b := range got.Withheld {
		if b.Reason == "" || ComposeFixLines(ComposeFixLinesParams{Binding: b, Job: "stack"}) != nil {
			t.Errorf("a conflict needs its own reason and no templating advice, got %+v", b)
		}
	}
}

// TestResolveDetectedPortsLeavesAFileAloneWhenTheBaseIsAlreadyTaken guards the
// other half: wtm must not rewrite a project file for a declaration it did not
// make. The hand-written 9999 would hijack the binding the file spells as 5432.
func TestResolveDetectedPortsLeavesAFileAloneWhenTheBaseIsAlreadyTaken(t *testing.T) {
	file := "docker-compose.yml"
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "docker-compose", Kind: domain.JobKindService,
		Cmd:   "docker compose " + DockerComposeFileFlag(file) + "up -d",
		Ports: map[string]int{"DB_PORT": 9999},
	}}}

	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{
			file: {File: file, Bindings: []domain.ComposePortBinding{frozenAt(file, "db", "DB_PORT", 5432)}},
		},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  composeAnswers(file),
		Existing: existing,
		Plan:     plan,
	})

	if len(got.Patches) != 0 {
		t.Errorf("the base declared (9999) is not the one the mapping has (5432), got %v", got.Patches)
	}
	if got.Config.Jobs[0].Ports["DB_PORT"] != 9999 {
		t.Errorf("the hand-written value must survive, got %v", got.Config.Jobs[0].Ports)
	}
}

// The two detections are pruned in one pass: a compose port and a .env port
// that meet a few worktrees on both have to go, which neither backfill can see
// on its own.
func TestResolveDetectedPortsPrunesAcrossComposeAndEnv(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Patch: true,
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			frozenAt(file, "storefront", "STOREFRONT_PORT", 3000),
		}}},
	})

	answers := composeAnswers(file)
	answers.SelectedPackageScripts = []domain.PackageScript{
		{Name: "dev", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindService},
	}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:        answers,
		PackageManager: domain.PkgManagerPnpm,
		Plan:           plan,
		EnvScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {
				Dir:         "apps/web",
				Ports:       map[string]int{"PORT": 3010},
				SourceByVar: map[string]string{"PORT": "apps/web/.env"},
			},
		},
	})

	if len(got.Dropped) != 2 {
		t.Fatalf("dropped %d, want both sides of the collision: %v", len(got.Dropped), got.Dropped)
	}
	if len(got.Written) != 0 {
		t.Errorf("compose ports = %v, want none to survive", got.Written)
	}
	if len(got.EnvWritten) != 0 {
		t.Errorf("env ports = %v, want none to survive", got.EnvWritten)
	}
	if len(got.Patches) != 0 {
		t.Errorf("a withdrawn declaration must not rewrite the compose file, got %v", got.Patches)
	}
}

func TestResolveDetectedPortsDeclaresEnvPortsAlongsideCompose(t *testing.T) {
	file := "docker-compose.yml"
	plan := PlanComposePorts(PlanComposePortsParams{
		Files: []string{file},
		Scans: map[string]domain.ComposeScan{file: {File: file, Bindings: []domain.ComposePortBinding{
			{File: file, Service: "db", Status: domain.ComposePortTemplated, Var: "DB_PORT", Base: 5432},
		}}},
	})

	answers := composeAnswers(file)
	answers.SelectedPackageScripts = []domain.PackageScript{
		{Name: "dev", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindService},
	}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:        answers,
		PackageManager: domain.PkgManagerPnpm,
		Plan:           plan,
		EnvScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {
				Dir:         "apps/web",
				Ports:       map[string]int{"PORT": 3000},
				SourceByVar: map[string]string{"PORT": "apps/web/.env"},
			},
		},
	})

	if got.Written["docker-compose"]["DB_PORT"] != 5432 {
		t.Errorf("compose ports = %v", got.Written)
	}
	if got.EnvWritten["web-dev"]["PORT"] != 3000 {
		t.Errorf("env ports = %v", got.EnvWritten)
	}
	if got.EnvSources["web-dev"]["PORT"] != "apps/web/.env" {
		t.Errorf("env sources = %v", got.EnvSources)
	}

	job := jobNamed(got.Config, "web-dev")
	if job.Ports["PORT"] != 3000 {
		t.Errorf("web-dev ports = %v, want the declaration written to run.toml", job.Ports)
	}
}

// A hand-written base outranks a detected one: only the detection gives way.
func TestResolveDetectedPortsKeepsAHandWrittenBaseOverAnEnvOne(t *testing.T) {
	answers := domain.InitProjectAnswers{
		SelectedPackageScripts: []domain.PackageScript{
			{Name: "dev", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindService},
		},
	}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:        answers,
		PackageManager: domain.PkgManagerPnpm,
		Existing: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "legacy", Kind: domain.JobKindService, Cmd: "node server.js", Cwd: ".", Ports: map[string]int{"LEGACY_PORT": 3000}},
		}},
		EnvScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {
				Dir:         "apps/web",
				Ports:       map[string]int{"PORT": 3010},
				SourceByVar: map[string]string{"PORT": "apps/web/.env"},
			},
		},
	})

	if jobNamed(got.Config, "legacy").Ports["LEGACY_PORT"] != 3000 {
		t.Error("the hand-written base must survive")
	}
	if len(got.EnvWritten) != 0 {
		t.Errorf("the detected base gives way, got %v", got.EnvWritten)
	}
}

// Le retrait doit avoir lieu avant que les steps suivantes lisent la config :
// elles la recalculent à chaque affichage, et proposeraient sinon un port puis
// une url pour un job sur le point de disparaître.
func TestResolveDetectedPortsRetireLesJobsDecoches(t *testing.T) {
	existing := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"web"}}},
	}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Existing:   existing,
		Deselected: []string{"web"},
	})

	if len(got.Config.Jobs) != 0 {
		t.Errorf("jobs = %+v, want aucun", got.Config.Jobs)
	}
	if len(got.Config.Profiles) != 0 {
		t.Errorf("profils = %+v, want aucun", got.Config.Profiles)
	}
	if len(got.Removed) != 1 || got.Removed[0] != "web" {
		t.Errorf("Removed = %v, want [web]", got.Removed)
	}
}

func jobNamed(cfg domain.RunConfig, name string) domain.JobConfig {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job
		}
	}
	return domain.JobConfig{}
}
