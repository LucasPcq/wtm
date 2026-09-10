package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func existingComposeConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".",
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
		}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"docker-compose", "web"}}},
	}
}

func reinitScans() map[string]domain.ComposeScan {
	return map[string]domain.ComposeScan{"docker-compose.yml": {Services: []domain.ComposeService{
		{Name: "db", Image: "postgres:16"}, {Name: "web", Image: "nginx"},
	}}}
}

func reinitAnswers(shared ...domain.SharedComposeService) domain.InitProjectAnswers {
	return domain.InitProjectAnswers{
		DockerComposeCmd:   "docker compose",
		DockerComposeFiles: []string{"docker-compose.yml"},
		ScopesAsked:        true,
		SharedServices:     shared,
		Scans:              reinitScans(),
	}
}

// The reported bug: re-running init on a project whose compose file already has
// a job never rebuilt it, so marking a service shared reached nothing — the
// run.toml was unchanged and the next init showed it per-worktree again.
func TestReinitLiftsASharedServiceOutOfAnExistingJob(t *testing.T) {
	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  reinitAnswers(domain.SharedComposeService{File: "docker-compose.yml", Service: "db"}),
		Existing: existingComposeConfig(),
	})

	shared, found := findJob(got.Config, "db")
	if !found {
		t.Fatalf("the shared service was never lifted; jobs = %+v", got.Config.Jobs)
	}
	if shared.Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want shared", shared.Scope)
	}
	if !strings.HasSuffix(shared.Cmd, "up -d db") {
		t.Errorf("cmd = %q, want it to start only db", shared.Cmd)
	}

	file, _ := findJob(got.Config, "docker-compose")
	if !strings.HasSuffix(file.Cmd, "up -d --no-deps web") {
		t.Errorf("cmd = %q, want the file's job left with web alone and its deps held back", file.Cmd)
	}
}

// A service taken out of the stack a profile started must keep starting with
// it, or the profile silently stops bringing its database up. The join runs
// after the profiles are settled: the wizard re-proposes them from the config
// on disk, which discarded an insertion made any earlier.
func TestReinitPutsTheLiftedJobInTheProfilesOfItsHost(t *testing.T) {
	shared := []domain.SharedComposeService{{File: "docker-compose.yml", Service: "db"}}
	resolved := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  reinitAnswers(shared...),
		Existing: existingComposeConfig(),
	})
	got := DetectedPortsOutcome{Config: JoinSharedProfiles(JoinSharedProfilesParams{
		Config: resolved.Config, Shared: shared,
	})}

	for _, profile := range got.Config.Profiles {
		if profile.Name != "dev" {
			continue
		}
		if strings.Join(profile.Jobs, ",") != "docker-compose,db,web" {
			t.Errorf("profile jobs = %v, want db right after the job it was lifted from", profile.Jobs)
		}
		return
	}
	t.Fatal("the dev profile disappeared")
}

// The answer goes both ways: unsharing gives the file's job its whole stack
// back and takes the lifted job away.
func TestReinitUnsharesAService(t *testing.T) {
	shared := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  reinitAnswers(domain.SharedComposeService{File: "docker-compose.yml", Service: "db"}),
		Existing: existingComposeConfig(),
	}).Config

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  reinitAnswers(),
		Existing: shared,
	})

	if _, found := findJob(got.Config, "db"); found {
		t.Errorf("the lifted job survived being unshared; jobs = %+v", got.Config.Jobs)
	}
	file, _ := findJob(got.Config, "docker-compose")
	if !strings.HasSuffix(file.Cmd, "up -d") {
		t.Errorf("cmd = %q, want the whole file started again", file.Cmd)
	}
}

// Re-running init and changing nothing must leave the config exactly as it was,
// or every re-init would churn the file.
func TestReinitIsIdempotent(t *testing.T) {
	answers := reinitAnswers(domain.SharedComposeService{File: "docker-compose.yml", Service: "db"})
	once := ResolveDetectedPorts(ResolveDetectedPortsParams{Answers: answers, Existing: existingComposeConfig()}).Config
	twice := ResolveDetectedPorts(ResolveDetectedPortsParams{Answers: answers, Existing: once}).Config

	if len(once.Jobs) != len(twice.Jobs) {
		t.Fatalf("jobs went from %d to %d on a second run", len(once.Jobs), len(twice.Jobs))
	}
	for i := range once.Jobs {
		if once.Jobs[i].Name != twice.Jobs[i].Name || once.Jobs[i].Cmd != twice.Jobs[i].Cmd {
			t.Errorf("job %d changed: %+v then %+v", i, once.Jobs[i], twice.Jobs[i])
		}
	}
	for i := range once.Profiles {
		if strings.Join(once.Profiles[i].Jobs, ",") != strings.Join(twice.Profiles[i].Jobs, ",") {
			t.Errorf("profile %q changed: %v then %v", once.Profiles[i].Name, once.Profiles[i].Jobs, twice.Profiles[i].Jobs)
		}
	}
}

// A namespace written by hand outranks the recipe the wizard offers: the config
// speaks, detection does not.
func TestReinitKeepsAHandWrittenNamespace(t *testing.T) {
	existing := existingComposeConfig()
	existing.Jobs = append(existing.Jobs, domain.JobConfig{
		Name: "db", Kind: domain.JobKindService, Scope: domain.JobScopeShared,
		Cmd:       "docker compose -f docker-compose.yml up -d db",
		Namespace: &domain.JobNamespaceConfig{Name: "mine_{worktree}", Create: "my-script"},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers: reinitAnswers(domain.SharedComposeService{
			File: "docker-compose.yml", Service: "db",
			Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "recipe"},
		}),
		Existing: existing,
	})

	shared, _ := findJob(got.Config, "db")
	if shared.Namespace == nil || shared.Namespace.Create != "my-script" {
		t.Errorf("namespace = %+v, want the one already written", shared.Namespace)
	}
}

// A run that never put the question leaves the config exactly as it stands.
func TestReinitWithoutTheQuestionChangesNothing(t *testing.T) {
	answers := reinitAnswers()
	answers.ScopesAsked = false

	existing := existingComposeConfig()
	got := ResolveDetectedPorts(ResolveDetectedPortsParams{Answers: answers, Existing: existing})

	file, _ := findJob(got.Config, "docker-compose")
	if file.Cmd != existing.Jobs[0].Cmd {
		t.Errorf("cmd = %q, want it untouched", file.Cmd)
	}
}

// Rewriting the command was not enough: the lifted job and the one it came out
// of both declared the same base, which reads as a collision and had wtm refuse
// to load run.toml at all. The links naming those variables follow, or they
// still point at a job that no longer declares them — the other half of the
// same refusal.
func TestReinitMovesThePortsAndTheLinksWithTheLiftedService(t *testing.T) {
	existing := existingComposeConfig()
	existing.Jobs[0].Ports = map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379}
	existing.EnvPorts = []domain.EnvPortLink{
		{File: ".env", Key: "POSTGRES_PORT", Job: "docker-compose", Port: "POSTGRES_PORT"},
		{File: "apps/api/.env", Key: "DATABASE_URL", Job: "docker-compose", Port: "POSTGRES_PORT"},
		{File: ".env", Key: "REDIS_PORT", Job: "docker-compose", Port: "REDIS_PORT"},
	}

	answers := reinitAnswers(domain.SharedComposeService{File: "docker-compose.yml", Service: "db"})
	answers.Scans = map[string]domain.ComposeScan{"docker-compose.yml": {
		Services: []domain.ComposeService{{Name: "db"}, {Name: "web"}},
		Bindings: []domain.ComposePortBinding{
			{File: "docker-compose.yml", Service: "db", Var: "POSTGRES_PORT", Base: 5432},
			{File: "docker-compose.yml", Service: "web", Var: "REDIS_PORT", Base: 6379},
		},
	}}

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  answers,
		Existing: existing,
		Plan:     ComposePortPlan{Declared: map[string][]domain.ComposePortBinding{"docker-compose.yml": answers.Scans["docker-compose.yml"].Bindings}},
	})

	shared, _ := findJob(got.Config, "db")
	if shared.Ports["POSTGRES_PORT"] != 5432 {
		t.Errorf("the lifted job declares %v, want POSTGRES_PORT", shared.Ports)
	}
	file, _ := findJob(got.Config, "docker-compose")
	if _, still := file.Ports["POSTGRES_PORT"]; still {
		t.Errorf("the file's job still declares POSTGRES_PORT: %v", file.Ports)
	}
	if file.Ports["REDIS_PORT"] != 6379 {
		t.Errorf("a port that stayed was taken away: %v", file.Ports)
	}

	// The config must load, which is the whole point.
	if errs := ValidateRunPorts(got.Config); len(errs) != 0 {
		t.Errorf("ValidateRunPorts = %v, want none", errs)
	}

	for _, link := range got.Config.EnvPorts {
		want := "docker-compose"
		if link.Port == "POSTGRES_PORT" {
			want = "db"
		}
		if link.Job != want {
			t.Errorf("link %s/%s points at %q, want %q", link.File, link.Key, link.Job, want)
		}
	}
}
