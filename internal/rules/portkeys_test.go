package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func webJobConfig() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{{
		Name:  "shop-web-dev",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm run dev",
		Cwd:   "apps/shop/web",
		Ports: map[string]int{"VITE_PORT": 5173},
	}}}
}

func webEnvFiles() []domain.EnvFile {
	return []domain.EnvFile{{Target: "apps/shop/web/.env", Template: "apps/shop/web/.env.example"}}
}

func TestPortKeyWritesProposesTheMissingKeyInTheJobsOwnFile(t *testing.T) {
	writes := PortKeyWrites(PortKeyWritesParams{Config: webJobConfig(), EnvFiles: webEnvFiles()})

	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: %+v", len(writes), writes)
	}
	want := domain.PortKeyWrite{
		Job: "shop-web-dev", Port: "VITE_PORT", Base: 5173,
		File: "apps/shop/web/.env", Template: "apps/shop/web/.env.example",
	}
	if writes[0] != want {
		t.Fatalf("got %+v, want %+v", writes[0], want)
	}
}

func TestPortKeyWritesLeavesAKeyTheEnvAlreadyHolds(t *testing.T) {
	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   webJobConfig(),
		EnvFiles: webEnvFiles(),
		ScansByDir: map[string]domain.EnvPortScan{"apps/shop/web": {
			Ports:       map[string]int{"VITE_PORT": 5173},
			SourceByVar: map[string]string{"VITE_PORT": "apps/shop/web/.env"},
		}},
	})

	if len(writes) != 0 {
		t.Fatalf("the key is already there: %+v", writes)
	}
}

func TestPortKeyWritesProposesTheMissingEnvTarget(t *testing.T) {
	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   webJobConfig(),
		EnvFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})

	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: %+v", len(writes), writes)
	}
	got := writes[0]
	if !got.AddTarget {
		t.Fatal("a file nothing provisions must carry the target to add")
	}
	if got.File != "apps/shop/web/.env" || got.Template != "apps/shop/web/.env.example" {
		t.Fatalf("the proposed target follows the job's own directory: %+v", got)
	}
}

func TestPortKeyWritesHonoursTheRoutedPorts(t *testing.T) {
	cfg := webJobConfig()
	cfg.Jobs = append(cfg.Jobs, domain.JobConfig{
		Name: "shop-api-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev",
		Cwd: "apps/shop/api", Ports: map[string]int{"PORT": 4001},
	})

	writes := PortKeyWrites(PortKeyWritesParams{
		Config: cfg,
		Ports:  []domain.PortRef{{Job: "shop-web-dev", Name: "VITE_PORT"}},
		EnvFiles: append(webEnvFiles(), domain.EnvFile{
			Target: "apps/shop/api/.env", Template: "apps/shop/api/.env.example",
		}),
	})

	if len(writes) != 1 || writes[0].Job != "shop-web-dev" {
		t.Fatalf("only the routed declaration is written: %+v", writes)
	}
}

func TestPortKeyWritesLeavesTheSameJobsOtherPortOnItsCommand(t *testing.T) {
	cfg := webJobConfig()
	cfg.Jobs[0].Ports = map[string]int{"VITE_PORT": 5173, "METRICS_PORT": 9100}

	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   cfg,
		Ports:    []domain.PortRef{{Job: "shop-web-dev", Name: "VITE_PORT"}},
		EnvFiles: webEnvFiles(),
	})

	if len(writes) != 1 || writes[0].Port != "VITE_PORT" {
		t.Fatalf("a route is answered port by port: %+v", writes)
	}
}

func TestPortKeyWritesIgnoresAPortCarriedOnlyByTheTemplate(t *testing.T) {
	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   webJobConfig(),
		EnvFiles: webEnvFiles(),
		ScansByDir: map[string]domain.EnvPortScan{"apps/shop/web": {
			Ports:       map[string]int{"VITE_PORT": 5173},
			SourceByVar: map[string]string{"VITE_PORT": "apps/shop/web/.env.example"},
		}},
	})

	if len(writes) != 1 {
		t.Fatalf("a key only the committed template holds reaches no worktree under the parent or main strategies: %+v", writes)
	}
}

func TestPortKeyWritesStillAddsTheTargetOfAFileAlreadyHoldingTheKey(t *testing.T) {
	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   webJobConfig(),
		EnvFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
		ScansByDir: map[string]domain.EnvPortScan{"apps/shop/web": {
			Ports:       map[string]int{"VITE_PORT": 5173},
			SourceByVar: map[string]string{"VITE_PORT": "apps/shop/web/.env"},
		}},
	})

	if len(writes) != 1 || !writes[0].AddTarget {
		t.Fatalf("the key is there but nothing provisions the file, so no worktree has it: %+v", writes)
	}
}

func TestPortKeyWritesSkipsAComposeBackedJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "stack", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: ".",
		Ports: map[string]int{"POSTGRES_PORT": 5432},
	}}}

	writes := PortKeyWrites(PortKeyWritesParams{
		Config:   cfg,
		EnvFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})

	if len(writes) != 0 {
		t.Fatalf("a compose stack reads its ports from the file it was templated into: %+v", writes)
	}
}

func TestPortKeyWritesSkipsATask(t *testing.T) {
	cfg := webJobConfig()
	cfg.Jobs[0].Kind = domain.JobKindTask

	if writes := PortKeyWrites(PortKeyWritesParams{Config: cfg, EnvFiles: webEnvFiles()}); len(writes) != 0 {
		t.Fatalf("a one-shot task binds nothing: %+v", writes)
	}
}

func TestPortKeyLinksFollowTheirJob(t *testing.T) {
	links := PortKeyLinks(PortKeyLinksParams{Writes: []domain.PortKeyWrite{{
		Job: "shop-web-dev", Port: "VITE_PORT", Base: 5173, File: "apps/shop/web/.env",
	}}})

	want := domain.EnvPortLink{File: "apps/shop/web/.env", Key: "VITE_PORT", Job: "shop-web-dev", Port: "VITE_PORT"}
	if len(links) != 1 || links[0] != want {
		t.Fatalf("got %+v, want %+v", links, want)
	}
}

func TestPortKeyTargetsNamesOnlyTheOnesToAdd(t *testing.T) {
	targets := PortKeyTargets(PortKeyTargetsParams{
		Writes: []domain.PortKeyWrite{
			{File: "apps/shop/web/.env", Template: "apps/shop/web/.env.example", AddTarget: true},
			{File: "apps/shop/web/.env", Template: "apps/shop/web/.env.example", AddTarget: true},
			{File: ".env", Template: ".env.example"},
		},
		Existing: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})

	want := []domain.EnvFile{{Target: "apps/shop/web/.env", Template: "apps/shop/web/.env.example"}}
	if len(targets) != 1 || targets[0] != want[0] {
		t.Fatalf("got %+v, want %+v", targets, want)
	}
}

func TestPortKeyTargetsIgnoresOneAlreadyDeclared(t *testing.T) {
	targets := PortKeyTargets(PortKeyTargetsParams{
		Writes:   []domain.PortKeyWrite{{File: ".env", Template: ".env.example", AddTarget: true}},
		Existing: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})

	if len(targets) != 0 {
		t.Fatalf("got %+v", targets)
	}
}

func TestPortRouteRowsPreFillsTheEnvRouteByDefault(t *testing.T) {
	rows := PortRouteRows(PortRouteRowsParams{Config: webJobConfig(), EnvFiles: webEnvFiles()})

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Route != domain.PortRouteEnv {
		t.Fatalf("the recommended route is the pre-filled one, got %q", rows[0].Route)
	}
	if rows[0].File != "apps/shop/web/.env" || rows[0].AddTarget {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestPortRouteRowsPreFillsTheCommandRouteWhenTheCommandNamesTheVar(t *testing.T) {
	cfg := webJobConfig()
	cfg.Jobs[0].Cmd = "pnpm run dev -- --port ${VITE_PORT}"

	rows := PortRouteRows(PortRouteRowsParams{Config: cfg, EnvFiles: webEnvFiles()})

	if rows[0].Route != domain.PortRouteCommand {
		t.Fatalf("a command already naming the variable has settled its route, got %q", rows[0].Route)
	}
}

func TestPortRouteRowsPreFillsTheEnvRouteWhenTheEnvAlreadyCarriesIt(t *testing.T) {
	cfg := webJobConfig()
	cfg.Jobs[0].Cmd = "pnpm run dev -- --port ${VITE_PORT}"

	rows := PortRouteRows(PortRouteRowsParams{
		Config:   cfg,
		EnvFiles: webEnvFiles(),
		ScansByDir: map[string]domain.EnvPortScan{"apps/shop/web": {
			Ports:       map[string]int{"VITE_PORT": 5173},
			SourceByVar: map[string]string{"VITE_PORT": "apps/shop/web/.env"},
		}},
	})

	if rows[0].Route != domain.PortRouteEnv {
		t.Fatalf("the value was detected in that .env, so the job demonstrably reads it: %q", rows[0].Route)
	}
}

func TestPortRouteRowsMarksAFileNothingProvisions(t *testing.T) {
	rows := PortRouteRows(PortRouteRowsParams{
		Config:   webJobConfig(),
		EnvFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
	})

	if !rows[0].AddTarget || rows[0].File != "apps/shop/web/.env" {
		t.Fatalf("got %+v", rows[0])
	}
}

func TestPortRouteRowsExemptsAComposeStack(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "stack", Kind: domain.JobKindService, Cmd: "docker compose up -d", Cwd: ".",
		Ports: map[string]int{"POSTGRES_PORT": 5432},
	}}}

	rows := PortRouteRows(PortRouteRowsParams{
		Config:      cfg,
		ComposeJobs: []string{"stack"},
		EnvFiles:    []domain.EnvFile{{Target: ".env"}},
	})

	if len(rows) != 0 {
		t.Fatalf("a stack reads its ports from the file wtm templated: %+v", rows)
	}
}

func TestPortRouteEnvPortsIsNilWhenTheStepNeverRan(t *testing.T) {
	if refs := PortRouteEnvPorts(domain.InitProjectAnswers{}); refs != nil {
		t.Fatalf("got %+v — a run that never asked means every port, which is the flag's meaning", refs)
	}
}

func TestPortRouteEnvPortsIsEmptyWhenEveryJobChoseItsCommand(t *testing.T) {
	refs := PortRouteEnvPorts(domain.InitProjectAnswers{
		PortRoutesAsked: true,
		PortRoutes:      map[domain.PortRef]domain.PortRoute{{Job: "web", Name: "PORT"}: domain.PortRouteCommand},
	})

	if refs == nil || len(refs) != 0 {
		t.Fatalf("got %+v — asked and refused is not the same as never asked", refs)
	}
}

func TestPortRouteEnvPortsNamesTheRoutedOnes(t *testing.T) {
	refs := PortRouteEnvPorts(domain.InitProjectAnswers{
		PortRoutesAsked: true,
		PortRoutes: map[domain.PortRef]domain.PortRoute{
			{Job: "web", Name: "PORT"}:    domain.PortRouteEnv,
			{Job: "api", Name: "PORT"}:    domain.PortRouteCommand,
			{Job: "adm", Name: "PORT"}:    domain.PortRouteEnv,
			{Job: "adm", Name: "UI_PORT"}: domain.PortRouteEnv,
		},
	})

	want := []domain.PortRef{{Job: "adm", Name: "PORT"}, {Job: "adm", Name: "UI_PORT"}, {Job: "web", Name: "PORT"}}
	if len(refs) != 3 {
		t.Fatalf("got %+v, want %+v", refs, want)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Fatalf("got %+v, want %+v", refs, want)
		}
	}
}

func TestJobsOnEnvRouteExemptsOnlyAJobWithEveryPortOnIt(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{"PORT": 3000, "METRICS_PORT": 9100}},
		{Name: "api", Ports: map[string]int{"PORT": 4000}},
	}}

	jobs := JobsOnEnvRoute(JobsOnEnvRouteParams{Config: cfg, Routes: map[domain.PortRef]domain.PortRoute{
		{Job: "web", Name: "PORT"}:         domain.PortRouteEnv,
		{Job: "web", Name: "METRICS_PORT"}: domain.PortRouteCommand,
		{Job: "api", Name: "PORT"}:         domain.PortRouteEnv,
	}})

	if len(jobs) != 1 || jobs[0] != "api" {
		t.Fatalf("got %+v — a job with one port left on its command still needs that command amended", jobs)
	}
}

func TestPortKeyLinksSkipsOneTheConfigAlreadyCarries(t *testing.T) {
	existing := domain.EnvPortLink{File: "apps/shop/web/.env", Key: "VITE_PORT", Job: "shop-web-dev", Port: "VITE_PORT"}

	links := PortKeyLinks(PortKeyLinksParams{
		Writes:   []domain.PortKeyWrite{{Job: "shop-web-dev", Port: "VITE_PORT", Base: 5173, File: "apps/shop/web/.env"}},
		Existing: []domain.EnvPortLink{existing},
	})

	if len(links) != 0 {
		t.Fatalf("got %+v", links)
	}
}

func TestPortKeysReportedIncludesAFileOnlyTheTargetChangedFor(t *testing.T) {
	write := domain.PortKeyWrite{Job: "reports-dev", Port: "REPORTS_PORT", Base: 5177, File: "apps/reports/.env", AddTarget: true}

	reported := PortKeysReported(PortKeysReportedParams{
		Writes:  []domain.PortKeyWrite{write},
		Targets: []domain.EnvFile{{Target: "apps/reports/.env"}},
	})

	if len(reported) != 1 {
		t.Fatalf("the file already held the value, but the project now provisions it: %+v", reported)
	}
}

func TestPortKeysReportedSkipsAWriteThatChangedNothing(t *testing.T) {
	write := domain.PortKeyWrite{Job: "web", Port: "PORT", Base: 3000, File: ".env"}

	if reported := PortKeysReported(PortKeysReportedParams{Writes: []domain.PortKeyWrite{write}}); len(reported) != 0 {
		t.Fatalf("got %+v", reported)
	}
}

func TestPortRouteRowLabelsAlignTheirColumns(t *testing.T) {
	rows := []domain.PortRouteRow{
		{Job: "crm-web-dev", Port: "VITE_PORT", Base: 5175, File: "apps/crm/web/.env", Route: domain.PortRouteEnv},
		{Job: "api", Port: "PORT", Base: 4001, File: "apps/api/.env", Route: domain.PortRouteCommand},
	}

	jobWidth, portWidth := PortRouteWidths(rows)
	first := PortRouteRowLabel(rows[0], jobWidth, portWidth)
	second := PortRouteRowLabel(rows[1], jobWidth, portWidth)

	if strings.Index(first, domain.RouteListEnvPrefix) != strings.Index(second, domain.RouteListCommand) {
		t.Fatalf("the route column must start at the same offset:\n%q\n%q", first, second)
	}
}

func TestPortEntryLabelsAlignAnUndeclaredServiceWithTheRest(t *testing.T) {
	entries := []domain.PortEntry{
		{Job: "crm-web-dev", Name: "VITE_PORT", Base: 5175},
		{Job: "api", Name: "PORT"},
	}

	jobWidth, nameWidth := PortEntryWidths(entries)
	first := PortEntryLabel(entries[0], jobWidth, nameWidth)
	second := PortEntryLabel(entries[1], jobWidth, nameWidth)

	if strings.Index(first, "VITE_PORT") != strings.Index(second, "PORT") {
		t.Fatalf("the port column must start at the same offset:\n%q\n%q", first, second)
	}
}
