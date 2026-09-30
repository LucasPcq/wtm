package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func bindings() map[string][]domain.ComposePortBinding {
	return map[string][]domain.ComposePortBinding{
		"docker-compose.yml": {
			{File: "docker-compose.yml", Service: "db", Var: "POSTGRES_PORT", Base: 5432},
			{File: "docker-compose.yml", Service: "web", Var: "WEB_PORT", Base: 8080},
		},
	}
}

// A lifted service and its file's job both carry the same `-f <file>` flag, so
// a backfill matching on that alone gave each of them every port of the file.
// Both then declared the same base, which reads as a collision on every
// variable and had the whole file pruned of its ports — the flagship case, a
// shared db beside an ordinary web, losing port isolation entirely.
func TestBackfillGivesALiftedServiceOnlyItsOwnPorts(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"docker-compose.yml"},
		Scans: map[string]domain.ComposeScan{"docker-compose.yml": {Services: []domain.ComposeService{
			{Name: "db", Image: "postgres:16"}, {Name: "web", Image: "nginx"},
		}}},
		Shared: []domain.SharedComposeService{{File: "docker-compose.yml", Service: "db"}},
	})

	got := BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      cfg,
		PortsByFile: map[string]map[string]int{"docker-compose.yml": {"POSTGRES_PORT": 5432, "WEB_PORT": 8080}},
		Declared:    bindings(),
		Shared:      []domain.SharedComposeService{{File: "docker-compose.yml", Service: "db"}},
	})

	shared, _ := findJob(got.Config, "db")
	if len(shared.Ports) != 1 || shared.Ports["POSTGRES_PORT"] != 5432 {
		t.Errorf("shared job ports = %v, want POSTGRES_PORT alone", shared.Ports)
	}

	file, _ := findJob(got.Config, "docker-compose")
	if len(file.Ports) != 1 || file.Ports["WEB_PORT"] != 8080 {
		t.Errorf("file job ports = %v, want WEB_PORT alone", file.Ports)
	}

	// The whole point: no collision, so nothing downstream prunes them away.
	if errs := ValidateRunPorts(got.Config); len(errs) != 0 {
		t.Errorf("ValidateRunPorts = %v, want none", errs)
	}
}

// A project that shares nothing must be backfilled exactly as it was before
// sharing existed.
func TestBackfillUnchangedWithoutALiftedService(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"docker-compose.yml"},
	})

	got := BackfillDockerPorts(BackfillDockerPortsParams{
		Config:      cfg,
		PortsByFile: map[string]map[string]int{"docker-compose.yml": {"POSTGRES_PORT": 5432, "WEB_PORT": 8080}},
		Declared:    bindings(),
	})

	file, _ := findJob(got.Config, "docker-compose")
	if len(file.Ports) != 2 {
		t.Errorf("ports = %v, want both", file.Ports)
	}
}
