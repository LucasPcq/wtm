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
	if !strings.HasSuffix(file.Cmd, "up -d web") {
		t.Errorf("cmd = %q, want the file's job left with web alone", file.Cmd)
	}
}

// A service taken out of the stack a profile started must keep starting with
// it, or the profile silently stops bringing its database up.
func TestReinitPutsTheLiftedJobInTheProfilesOfItsHost(t *testing.T) {
	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers:  reinitAnswers(domain.SharedComposeService{File: "docker-compose.yml", Service: "db"}),
		Existing: existingComposeConfig(),
	})

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

// A tenant written by hand outranks the recipe the wizard offers: the config
// speaks, detection does not.
func TestReinitKeepsAHandWrittenTenant(t *testing.T) {
	existing := existingComposeConfig()
	existing.Jobs = append(existing.Jobs, domain.JobConfig{
		Name: "db", Kind: domain.JobKindService, Scope: domain.JobScopeShared,
		Cmd:    "docker compose -f docker-compose.yml up -d db",
		Tenant: &domain.JobTenantConfig{Name: "mine_{worktree}", Attach: "my-script"},
	})

	got := ResolveDetectedPorts(ResolveDetectedPortsParams{
		Answers: reinitAnswers(domain.SharedComposeService{
			File: "docker-compose.yml", Service: "db",
			Tenant: &domain.JobTenantConfig{Name: "app_{worktree}", Attach: "recipe"},
		}),
		Existing: existing,
	})

	shared, _ := findJob(got.Config, "db")
	if shared.Tenant == nil || shared.Tenant.Attach != "my-script" {
		t.Errorf("tenant = %+v, want the one already written", shared.Tenant)
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
