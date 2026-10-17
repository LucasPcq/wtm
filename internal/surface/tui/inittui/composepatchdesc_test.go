package inittui

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestComposePatchDescriptionShowsBothHalves(t *testing.T) {
	got := composePatchDescription(
		map[string][]domain.ComposePortBinding{
			"docker-compose.yml": {{File: "docker-compose.yml", Service: "postgres", Token: `"5432:5432"`, Replacement: `"${POSTGRES_PORT:-5432}:5432"`}},
		},
		map[string][]domain.ComposeAbsoluteName{
			"docker-compose.yml": {{File: "docker-compose.yml", Kind: domain.ComposeNameVolume, Owner: "pgdata", Token: "app-pgdata", Replacement: `"${COMPOSE_PROJECT_NAME:-app}-pgdata"`}},
		},
	)
	t.Log("\n" + got)

	for _, want := range []string{
		domain.ComposePatchStepPortsLead,
		domain.ComposePatchStepNamesLead,
		domain.ComposeNamesVolumeWarning,
		"5432:5432",
		"app-pgdata",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestComposePatchDescriptionOmitsTheEmptyHalf(t *testing.T) {
	got := composePatchDescription(
		map[string][]domain.ComposePortBinding{
			"docker-compose.yml": {{File: "docker-compose.yml", Service: "web", Token: `"3000:3000"`, Replacement: `"${WEB_PORT:-3000}:3000"`}},
		},
		nil,
	)
	if strings.Contains(got, domain.ComposePatchStepNamesLead) {
		t.Error("a file with no pinned name must not show the names half")
	}
	if strings.Contains(got, domain.ComposeNamesVolumeWarning) {
		t.Error("no volume renamed, no volume warning")
	}
}
