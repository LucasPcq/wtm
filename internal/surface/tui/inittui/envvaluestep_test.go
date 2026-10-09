package inittui

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// envValueWizard drives the three steps that feed each other the way the wizard
// does: the compose selection, the scope answer, then the namespace answer. The
// [[env]] step reads all three, so anything less than the chain cannot reproduce
// what a real run shows.
func envValueWizard(t *testing.T, existing domain.RunConfig) []domain.EnvValueField {
	t.Helper()

	const file = "docker-compose.yml"
	detection := domain.InitDetectionResult{
		DockerComposeFiles: []string{file},
		DockerComposeCmd:   "docker compose",
		ComposeScans: map[string]domain.ComposeScan{file: {
			File:     file,
			Services: []domain.ComposeService{{Name: "postgres", Image: "postgres:18"}},
			Bindings: []domain.ComposePortBinding{{
				File: file, Service: "postgres", Var: "POSTGRES_PORT", Base: 5432,
			}},
		}},
	}
	params := addServicesStepsParams{
		Detection: detection,
		Existing:  existing,
		EnvFiles:  []domain.EnvFile{{Target: "apps/api/.env"}},
		EnvLines: map[string][]domain.EnvLine{"apps/api/.env": {
			{Kind: domain.EnvLinePair, Key: "DATABASE_URL", Value: "postgresql://app@localhost:5432/app"},
			{Kind: domain.EnvLinePair, Key: "POSTGRES_SCHEMA", Value: "public"},
		}},
	}

	s := newStepSet()
	s.add(stepDocker, components.Step{})
	addScopeStep(s, params)
	addNamespaceStep(s, params)
	addEnvValueStep(s, params)

	steps := dockerSelection(file)
	for _, id := range []string{stepScopes, stepNamespaces, stepEnvValues} {
		i := s.at(id)
		if i < 0 {
			t.Fatalf("%s was not registered", id)
		}
		step := s.steps[i]
		step.Model = step.Build(steps)
		steps = append(steps, step)
	}

	list, ok := steps[len(steps)-1].Model.(components.EnvValueListModel)
	if !ok {
		t.Fatalf("the [[env]] step built a %T", steps[len(steps)-1].Model)
	}
	return list.Fields()
}

// A re-init on a project that already declares a shared postgres with a
// namespace must offer that project's keys. Showing the step with no rows at all
// is the report saying there is nothing to link while run.toml says otherwise.
func TestEnvValueStepOffersKeysOnAReInit(t *testing.T) {
	existing := domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "postgres", Kind: domain.JobKindService,
			Cmd:   "docker compose -f docker-compose.yml up -d postgres",
			Scope: domain.JobScopeShared,
			Ports: map[string]int{"POSTGRES_PORT": 5432},
			Namespace: &domain.JobNamespaceConfig{
				Name: "app_{worktree}", Create: "createdb $WTM_NAMESPACE",
			},
		}},
	}

	fields := envValueWizard(t, existing)

	if len(fields) == 0 {
		t.Fatal("the step is empty; a re-init must offer the keys of the files wtm manages")
	}
	for _, want := range []string{"DATABASE_URL", "POSTGRES_SCHEMA"} {
		found := false
		for _, field := range fields {
			if field.Key == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s missing from %+v", want, fields)
		}
	}
}
