package inittui

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// The reported bug: the ports step planned on answers carrying no sharing, so
// it offered a lifted service's port on the job it was taken out of — and
// folding that answer back in declared the same base twice, which run.toml
// refuses at the very end of the wizard, losing every answer with it.
func TestPortsStepPlansOnTheSharingJustAnswered(t *testing.T) {
	const file = "docker-compose.dev.yml"
	detection := domain.InitDetectionResult{
		DockerComposeFiles: []string{file},
		DockerComposeCmd:   "docker compose",
		ComposeScans: map[string]domain.ComposeScan{file: {
			File: file,
			Services: []domain.ComposeService{
				{Name: "app"}, {Name: "keycloak"}, {Name: "keycloak_postgres"},
			},
			Bindings: []domain.ComposePortBinding{
				{File: file, Service: "app", Var: "APP_PORT", Base: 3000, Status: domain.ComposePortTemplated},
				{File: file, Service: "keycloak", Var: "KEYCLOAK_PORT", Base: 8080, Status: domain.ComposePortTemplated},
				{File: file, Service: "keycloak_postgres", Var: "KEYCLOAK_POSTGRES_PORT", Base: 5436, Status: domain.ComposePortTemplated},
			},
		}},
	}

	s := newStepSet()
	s.add(stepDocker, components.Step{})
	addScopeStep(s, addServicesStepsParams{Detection: detection})
	addPortsAndProfilesSteps(s, addServicesStepsParams{Detection: detection})

	steps := dockerSelection(file)
	steps = append(steps, components.Step{Model: components.NewScopeList(components.NewScopeListParams{
		Entries: []rules.ServiceScopeChoice{
			{File: file, Service: "app"},
			{File: file, Service: "keycloak", Scope: domain.JobScopeShared},
			{File: file, Service: "keycloak_postgres", Scope: domain.JobScopeShared},
		},
	})})

	at := s.at(stepPorts)
	if at < 0 {
		t.Fatal("the ports step was not registered")
	}
	list, ok := s.steps[at].Build(steps).(components.PortListModel)
	if !ok {
		t.Fatalf("the ports step built a %T", s.steps[at].Build(steps))
	}

	byPort := map[string]string{}
	for _, entry := range list.Entries() {
		byPort[entry.Name] = entry.Job
	}
	if byPort["KEYCLOAK_PORT"] != "keycloak" {
		t.Errorf("KEYCLOAK_PORT offered on %q, want the job it was lifted into", byPort["KEYCLOAK_PORT"])
	}
	if byPort["KEYCLOAK_POSTGRES_PORT"] != "keycloak_postgres" {
		t.Errorf("KEYCLOAK_POSTGRES_PORT offered on %q, want the job it was lifted into", byPort["KEYCLOAK_POSTGRES_PORT"])
	}
	if byPort["APP_PORT"] != "docker-compose-dev" {
		t.Errorf("APP_PORT offered on %q, want the file's own job", byPort["APP_PORT"])
	}
}
