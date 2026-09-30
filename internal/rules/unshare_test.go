package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const unshareFile = "docker-compose.yml"

func unshareScans() map[string]domain.ComposeScan {
	return map[string]domain.ComposeScan{unshareFile: {
		File:     unshareFile,
		Services: []domain.ComposeService{{Name: "db"}, {Name: "redis"}},
		Bindings: []domain.ComposePortBinding{
			{File: unshareFile, Service: "db", Var: "POSTGRES_PORT", Base: 5432},
			{File: unshareFile, Service: "redis", Var: "REDIS_PORT", Base: 6379},
		},
	}}
}

func sharedComposeJob(service string, port string, base int) domain.JobConfig {
	return domain.JobConfig{
		Name: service, Kind: domain.JobKindService, Cwd: ".",
		Cmd:   "docker compose -f docker-compose.yml up -d " + service,
		Stop:  "docker compose -f docker-compose.yml stop " + service,
		Scope: domain.JobScopeShared,
		Ports: map[string]int{port: base},
	}
}

// Every service of the file shared: the file has no job of its own.
func fullyLiftedConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate", Touches: []string{"db"}},
			sharedComposeJob("db", "POSTGRES_PORT", 5432),
			sharedComposeJob("redis", "REDIS_PORT", 6379),
		},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"db", "redis", "migrate"}}},
		EnvPorts: []domain.EnvPortLink{{File: "apps/api/.env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"}},
	}
}

func unshareAnswers(shared ...domain.SharedComposeService) domain.InitProjectAnswers {
	return domain.InitProjectAnswers{
		DockerComposeCmd:   "docker compose",
		DockerComposeFiles: []string{unshareFile},
		ScopesAsked:        true,
		SharedServices:     shared,
		Scans:              unshareScans(),
	}
}

func resolveUnshare(existing domain.RunConfig, answers domain.InitProjectAnswers) DetectedPortsOutcome {
	return ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  answers,
		Existing: existing,
		Plan:     ComposePortPlan{Declared: map[string][]domain.ComposePortBinding{unshareFile: unshareScans()[unshareFile].Bindings}},
	})
}

// The verified case: un-sharing every service of a file left run.toml holding
// migrate alone — the stack, the touches and the .env link all gone, and the
// recap silent about it.
func TestUnsharingEveryServiceGivesTheFileItsJobBack(t *testing.T) {
	got := resolveUnshare(fullyLiftedConfig(), unshareAnswers())

	file, found := findJob(got.Config, "docker-compose")
	if !found {
		t.Fatalf("the file's job was not rebuilt; jobs = %+v", got.Config.Jobs)
	}
	if file.Cmd != "docker compose -f docker-compose.yml up -d" {
		t.Errorf("cmd = %q, want the whole file started", file.Cmd)
	}
	if file.Stop != "docker compose -f docker-compose.yml down --remove-orphans" {
		t.Errorf("stop = %q, want the whole file torn down", file.Stop)
	}
	if file.Ports["POSTGRES_PORT"] != 5432 || file.Ports["REDIS_PORT"] != 6379 {
		t.Errorf("ports = %v, want both services' ports back on the file's job", file.Ports)
	}
	for _, name := range []string{"db", "redis"} {
		if _, still := findJob(got.Config, name); still {
			t.Errorf("the un-shared job %q survived", name)
		}
	}

	migrate, _ := findJob(got.Config, "migrate")
	if !slices.Equal(migrate.Touches, []string{"docker-compose"}) {
		t.Errorf("migrate touches %v, want the file's job", migrate.Touches)
	}
	if len(got.Config.EnvPorts) != 1 || got.Config.EnvPorts[0].Job != "docker-compose" {
		t.Errorf("env ports = %+v, want DATABASE_URL following the file's job", got.Config.EnvPorts)
	}
	if !slices.Equal(got.Config.Profiles[0].Jobs, []string{"docker-compose", "migrate"}) {
		t.Errorf("profile = %v, want the file's job once, in place of both", got.Config.Profiles[0].Jobs)
	}

	slices.Sort(got.Removed)
	if !slices.Equal(got.Removed, []string{"db", "redis"}) {
		t.Errorf("removed = %v, want both withdrawn jobs reported", got.Removed)
	}
	if _, errs := ValidateRun(got.Config); len(errs) != 0 {
		t.Errorf("the config no longer loads: %v", errs)
	}
}

// A partial un-share redirected nothing: the references to the withdrawn job
// were deleted when the file's job was right there to take them.
func TestPartialUnshareRedirectsTheReferences(t *testing.T) {
	existing := fullyLiftedConfig()
	existing.Jobs = append(existing.Jobs, domain.JobConfig{
		Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".",
		Cmd:  "docker compose -f docker-compose.yml up -d --no-deps web",
		Stop: "docker compose -f docker-compose.yml rm -s -f web",
	})
	scans := unshareScans()
	scan := scans[unshareFile]
	scan.Services = append(scan.Services, domain.ComposeService{Name: "web"})
	scans[unshareFile] = scan
	answers := unshareAnswers(domain.SharedComposeService{File: unshareFile, Service: "redis"})
	answers.Scans = scans

	got := resolveUnshare(existing, answers)

	file, _ := findJob(got.Config, "docker-compose")
	if !strings.HasSuffix(file.Cmd, "up -d --no-deps db web") {
		t.Errorf("cmd = %q, want db back in the file's stack", file.Cmd)
	}
	if file.Stop != "docker compose -f docker-compose.yml rm -s -f db web" {
		t.Errorf("stop = %q, want db back in what the file's job stops", file.Stop)
	}
	migrate, _ := findJob(got.Config, "migrate")
	if !slices.Equal(migrate.Touches, []string{"docker-compose"}) {
		t.Errorf("migrate touches %v, want the reference redirected", migrate.Touches)
	}
	if got.Config.EnvPorts[0].Job != "docker-compose" {
		t.Errorf("DATABASE_URL follows %q, want the file's job", got.Config.EnvPorts[0].Job)
	}
	if _, still := findJob(got.Config, "redis"); !still {
		t.Error("redis is still shared and must stay")
	}
	if _, errs := ValidateRun(got.Config); len(errs) != 0 {
		t.Errorf("the config no longer loads: %v", errs)
	}
}

// A value reading the namespace of a service that no longer has one cannot be
// redirected: it is dropped, and said.
func TestUnshareDropsAndReportsTheNamespaceValues(t *testing.T) {
	existing := fullyLiftedConfig()
	existing.Jobs[1].Namespace = &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "createdb"}
	existing.EnvValues = []domain.EnvValueLink{{File: "apps/api/.env", Key: "DB_NAME", Job: "db", Value: "{namespace}"}}

	got := resolveUnshare(existing, unshareAnswers())

	if len(got.Config.EnvValues) != 0 {
		t.Errorf("env values = %+v, want the namespace link dropped", got.Config.EnvValues)
	}
	if !slices.Equal(got.Unlinked, []string{"DB_NAME"}) {
		t.Errorf("unlinked = %v, want DB_NAME reported", got.Unlinked)
	}
}

// Sharing compose `db` while a script job already answers to `db` took the
// script over: it became a shared service running pnpm.
func TestSharingAServiceNeverTakesOverAJobOfTheSameName(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Kind: domain.JobKindTask, Cmd: "pnpm db:seed"},
		{
			Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".",
			Cmd:   "docker compose -f docker-compose.yml up -d",
			Stop:  "docker compose -f docker-compose.yml down --remove-orphans",
			Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379},
		},
	}}
	answers := unshareAnswers(domain.SharedComposeService{File: unshareFile, Service: "db"})

	got := resolveUnshare(existing, answers)

	script, _ := findJob(got.Config, "db")
	if script.Cmd != "pnpm db:seed" || IsShared(script) {
		t.Errorf("the script job was taken over: %+v", script)
	}
	lifted, found := findJob(got.Config, "db-2")
	if !found || !IsShared(lifted) || !strings.HasSuffix(lifted.Cmd, "up -d db") {
		t.Fatalf("the service was not lifted under a free name; jobs = %+v", got.Config.Jobs)
	}
	if lifted.Ports["POSTGRES_PORT"] != 5432 {
		t.Errorf("lifted ports = %v, want POSTGRES_PORT", lifted.Ports)
	}
	if len(got.Renamed) != 1 || !strings.Contains(got.Renamed[0], "db-2") {
		t.Errorf("renamed = %v, want the new name said", got.Renamed)
	}
	if _, errs := ValidateRun(got.Config); len(errs) != 0 {
		t.Errorf("the config no longer loads: %v", errs)
	}

	again := resolveUnshare(got.Config, answers)
	if len(again.Config.Jobs) != len(got.Config.Jobs) || len(again.Renamed) != 0 {
		t.Errorf("a second run churned: %+v, renamed %v", again.Config.Jobs, again.Renamed)
	}

	withdrawn := resolveUnshare(got.Config, unshareAnswers())
	if _, still := findJob(withdrawn.Config, "db-2"); still {
		t.Error("un-sharing did not find the renamed lifted job")
	}
	if script, _ := findJob(withdrawn.Config, "db"); script.Cmd != "pnpm db:seed" {
		t.Errorf("un-sharing reached the script job: %+v", script)
	}
}

func TestComposeSharingLinesNamesEachChange(t *testing.T) {
	got := ComposeSharingLines(ComposeSharingLinesParams{
		Renamed:  []string{"renamed line"},
		Unlinked: []string{"DB_NAME"},
	})
	if len(got) != 2 || got[0] != "renamed line" || !strings.HasPrefix(got[1], "DB_NAME") {
		t.Errorf("lines = %v, want the rename then the unlinked key", got)
	}
}
