package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// twoJobsOnePortName is the shape the flattened name -> base map could not
// express: each app binds its own PORT, so the name alone names two bases.
func twoJobsOnePortName() domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"PORT": 3001}},
		{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web", Ports: map[string]int{"PORT": 5173}},
	}}
}

func TestEnvPortBasesKeepsBothJobsPorts(t *testing.T) {
	bases := EnvPortBases(twoJobsOnePortName())

	if got := bases[domain.PortRef{Job: "api-dev", Name: "PORT"}]; got != 3001 {
		t.Errorf("api-dev PORT = %d, want 3001", got)
	}
	if got := bases[domain.PortRef{Job: "web-dev", Name: "PORT"}]; got != 5173 {
		t.Errorf("web-dev PORT = %d, want 5173 — the other job's base overwrote it", got)
	}
}

func TestEnvPortCandidatesNamesTheJobEachKeyFollows(t *testing.T) {
	// Every one of these four keys holds a port one of the two jobs declares.
	// Under the flattened map, half of them matched nothing.
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/api/.env": {
				{Kind: domain.EnvLinePair, Key: "PORT", Value: "3001"},
				{Kind: domain.EnvLinePair, Key: "CORS_ORIGIN", Value: "http://localhost:5173"},
			},
			"apps/web/.env": {
				{Kind: domain.EnvLinePair, Key: "PORT", Value: "5173"},
				{Kind: domain.EnvLinePair, Key: "VITE_API_URL", Value: "http://localhost:3001"},
			},
		},
		Bases: EnvPortBases(twoJobsOnePortName()),
	})

	if len(got) != 4 {
		t.Fatalf("expected all four keys to be offered, got %+v", got)
	}

	want := map[string]string{
		"apps/api/.env|PORT":         "api-dev",
		"apps/api/.env|CORS_ORIGIN":  "web-dev",
		"apps/web/.env|PORT":         "web-dev",
		"apps/web/.env|VITE_API_URL": "api-dev",
	}
	for _, link := range got {
		if job := want[link.File+"|"+link.Key]; link.Job != job {
			t.Errorf("%s · %s follows job %q, want %q", link.File, link.Key, link.Job, job)
		}
	}
}

func TestPlanEnvPortsResolvesEachLinkAgainstItsOwnJob(t *testing.T) {
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: "apps/api/.env", Key: "PORT", Job: "api-dev", Port: "PORT"},
			{File: "apps/web/.env", Key: "PORT", Job: "web-dev", Port: "PORT"},
		},
		Bases:  EnvPortBases(twoJobsOnePortName()),
		Offset: 10,
		Lines: map[string][]domain.EnvLine{
			"apps/api/.env": {{Kind: domain.EnvLinePair, Key: "PORT", Value: "3001"}},
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: "PORT", Value: "5173"}},
		},
	})

	byFile := map[string]domain.EnvPortEntry{}
	for _, e := range plan.Entries {
		byFile[e.File] = e
	}
	if got := byFile["apps/api/.env"].Resolved; got != 3011 {
		t.Errorf("api resolved to %d, want 3011", got)
	}
	if got := byFile["apps/web/.env"].Resolved; got != 5183 {
		t.Errorf("web resolved to %d, want 5183", got)
	}
}

func TestPlanEnvPortsRefusesALinkWithNoJob(t *testing.T) {
	// Every port belongs to exactly one job, so a link that names none is
	// incomplete rather than lenient. Resolving it on the name alone is how a key
	// came to follow the wrong port.
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "PORT", Port: "PORT"}},
		Bases:  EnvPortBases(twoJobsOnePortName()),
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": {{Kind: domain.EnvLinePair, Key: "PORT", Value: "3001"}}},
	})

	if len(plan.Entries) != 0 {
		t.Errorf("a link naming no job must not be resolved by guesswork: %+v", plan.Entries)
	}
}

func TestValidateRefusesALinkWithNoJobAndNamesTheOneToWrite(t *testing.T) {
	cfg := twoJobsOnePortName()
	cfg.EnvPorts = []domain.EnvPortLink{{File: "apps/api/.env", Key: "PORT", Port: "PORT"}}

	errs := ValidateRunPorts(cfg)

	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", errs)
	}
	for _, want := range []string{"job is required", "api-dev", "web-dev"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("error is missing %q: %s", want, errs[0])
		}
	}
}

func TestValidateAcceptsALinkNamingItsJob(t *testing.T) {
	cfg := twoJobsOnePortName()
	cfg.EnvPorts = []domain.EnvPortLink{{File: "apps/api/.env", Key: "PORT", Job: "api-dev", Port: "PORT"}}

	if errs := ValidateRunPorts(cfg); len(errs) != 0 {
		t.Errorf("a complete link must validate, got %v", errs)
	}
}
