package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func cfgWithLinks(links ...domain.EnvPortLink) domain.RunConfig {
	return domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "db", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"POSTGRES_PORT": 5432}}},
		EnvPorts: links,
	}
}

func TestValidateRunPortsAcceptsAWellFormedLink(t *testing.T) {
	cfg := cfgWithLinks(domain.EnvPortLink{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"})
	if errs := ValidateRunPorts(cfg); len(errs) > 0 {
		t.Errorf("ValidateRunPorts() = %v, want no error", errs)
	}
}

func TestValidateRunPortsRejectsBadLinks(t *testing.T) {
	cases := []struct {
		name string
		cfg  domain.RunConfig
		want string
	}{
		{
			"port no job declares",
			cfgWithLinks(domain.EnvPortLink{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "GHOST_PORT"}),
			"no job declares",
		},
		{
			"key that is not an environment variable name",
			cfgWithLinks(domain.EnvPortLink{File: ".env", Key: "not-a-var", Job: "db", Port: "POSTGRES_PORT"}),
			"not a valid environment variable name",
		},
		{
			"missing file",
			cfgWithLinks(domain.EnvPortLink{Key: "DATABASE_URL", Port: "POSTGRES_PORT"}),
			"file is required",
		},
		{
			"same key following the same port twice",
			cfgWithLinks(
				domain.EnvPortLink{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
				domain.EnvPortLink{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
			),
			"twice",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := ValidateRunPorts(c.cfg)
			if !strings.Contains(strings.Join(errs, "; "), c.want) {
				t.Errorf("ValidateRunPorts() = %v, want an error mentioning %q", errs, c.want)
			}
		})
	}

	// A CORS_ORIGIN listing two front-ends follows both. Refusing that was what
	// left such a key pinned to the main checkout in every worktree.
	t.Run("same key following two different ports", func(t *testing.T) {
		cfg := domain.RunConfig{
			Jobs: []domain.JobConfig{
				{Name: "web", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"VITE_PORT": 5173}},
				{Name: "admin", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"VITE_PORT": 5174}},
			},
			EnvPorts: []domain.EnvPortLink{
				{File: ".env", Key: "CORS_ORIGIN", Job: "web", Port: "VITE_PORT"},
				{File: ".env", Key: "CORS_ORIGIN", Job: "admin", Port: "VITE_PORT"},
			},
		}
		errs := ValidateRunPorts(cfg)
		if len(errs) != 0 {
			t.Errorf("ValidateRunPorts() = %v, want a key allowed to follow two ports", errs)
		}
	})
}

// The same key may follow different ports in two different files — a monorepo
// where each app has its own .env is the normal case, not a mistake.
func TestValidateRunPortsAllowsTheSameKeyInTwoFiles(t *testing.T) {
	cfg := cfgWithLinks(
		domain.EnvPortLink{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
		domain.EnvPortLink{File: "apps/web/.env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
	)
	if errs := ValidateRunPorts(cfg); len(errs) > 0 {
		t.Errorf("ValidateRunPorts() = %v, want no error", errs)
	}
}

func TestValidateEnvPortTargets(t *testing.T) {
	files := []domain.EnvFile{{Target: ".env"}, {Target: "apps/web/.env"}}

	if errs := ValidateEnvPortTargets([]domain.EnvPortLink{{File: ".env", Key: "K", Port: "P"}}, files); len(errs) > 0 {
		t.Errorf("ValidateEnvPortTargets() = %v, want no error for a configured target", errs)
	}

	errs := ValidateEnvPortTargets([]domain.EnvPortLink{{File: "services/api/.env", Key: "K", Port: "P"}}, files)
	if len(errs) != 1 || !strings.Contains(errs[0], "not a configured env file") {
		t.Errorf("ValidateEnvPortTargets() = %v, want a refusal naming the unconfigured file", errs)
	}
}

func TestEnvPortCandidates(t *testing.T) {
	bases := map[domain.PortRef]int{{Job: "db", Name: "POSTGRES_PORT"}: 5432, {Job: "db", Name: "API_PORT"}: 3000}
	lines := map[string][]domain.EnvLine{
		".env": ParseEnv(strings.Join([]string{
			"DATABASE_URL=postgres://u:pw@localhost:5432/app",
			"API_URL=http://localhost:3000/api",
			"SECRET=nothing-to-see",
			"BOTH=http://localhost:3000?db=5432",
			"ALREADY=postgres://localhost:5432/x",
			"",
		}, "\n")),
	}

	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines:    lines,
		Bases:    bases,
		Existing: []domain.EnvPortLink{{File: ".env", Key: "ALREADY", Port: "POSTGRES_PORT"}},
	})

	// Each candidate names the job whose port it follows: that is what the link
	// needs to resolve to one base rather than to a name two jobs may share. A
	// value holding two declared ports — an origin list, a url carrying a query
	// — follows both: picking one would leave the other pinned to the main
	// checkout in every worktree.
	want := []domain.EnvPortLink{
		{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
		{File: ".env", Key: "API_URL", Job: "db", Port: "API_PORT"},
		{File: ".env", Key: "BOTH", Job: "db", Port: "API_PORT"},
		{File: ".env", Key: "BOTH", Job: "db", Port: "POSTGRES_PORT"},
	}
	if len(got) != len(want) {
		t.Fatalf("EnvPortCandidates() = %+v, want %+v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("candidate %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestEnvPortCandidatesWithoutDeclaredPortsFindsNothing(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{".env": ParseEnv("URL=postgres://localhost:5432/app\n")},
	})
	if len(got) != 0 {
		t.Errorf("EnvPortCandidates() = %+v, want none", got)
	}
}

// A re-run of `wtm run init` merges into the existing config. The links the user
// already confirmed are not the detection's to discard — the same reason
// port_offset_block is carried over explicitly.
func TestMergeRunConfigsKeepsExistingEnvPorts(t *testing.T) {
	existing := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "db", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"POSTGRES_PORT": 5432}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Port: "POSTGRES_PORT"}},
	}
	detected := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "y"}}}

	merged, _ := MergeRunConfigs(existing, detected)
	if len(merged.EnvPorts) != 1 || merged.EnvPorts[0].Key != "DATABASE_URL" {
		t.Errorf("MergeRunConfigs() dropped the existing links: %+v", merged.EnvPorts)
	}
}

func TestMergeRunConfigsDoesNotMutateTheSource(t *testing.T) {
	existing := domain.RunConfig{EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "A", Port: "P"}}}

	merged, _ := MergeRunConfigs(existing, domain.RunConfig{})
	merged.EnvPorts[0].Key = "MUTATED"

	if existing.EnvPorts[0].Key != "A" {
		t.Errorf("MergeRunConfigs() wrote through to its source: %+v", existing.EnvPorts)
	}
}
