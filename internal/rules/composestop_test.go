package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func composeConfig(stop string) domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{
			Name: "docker-compose", Kind: domain.JobKindService,
			Cmd:  "docker compose -f docker-compose.yml up -d --no-deps redis minio",
			Stop: stop,
		},
		{
			Name: "postgres", Kind: domain.JobKindService, Scope: domain.JobScopeShared,
			Cmd:  "docker compose -f docker-compose.yml up -d postgres",
			Stop: "docker compose -f docker-compose.yml stop postgres",
		},
	}}
}

// A config written before the fix keeps a `down` that removes the shared
// postgres in main: it is named, with the stop to put instead.
func TestComposeStopsOverSharedNamesTheOldDown(t *testing.T) {
	fixes := rules.ComposeStopsOverShared(composeConfig("docker compose -f docker-compose.yml down --remove-orphans"))

	if len(fixes) != 1 {
		t.Fatalf("fixes = %+v, want the file job", fixes)
	}
	if fixes[0].Job != "docker-compose" || fixes[0].Shared != "postgres" ||
		fixes[0].Stop != "docker compose -f docker-compose.yml rm -s -f redis minio" {
		t.Errorf("fix = %+v", fixes[0])
	}
}

func TestComposeStopsOverSharedLeavesAFixedConfigAlone(t *testing.T) {
	if fixes := rules.ComposeStopsOverShared(composeConfig("docker compose -f docker-compose.yml rm -s -f redis minio")); len(fixes) != 0 {
		t.Errorf("fixes = %+v, want none", fixes)
	}
}

// Another file's `down` removes another project: nothing shared lives there.
func TestComposeStopsOverSharedIgnoresAnotherFile(t *testing.T) {
	cfg := composeConfig("docker compose -f infra.yml down --remove-orphans")
	cfg.Jobs[0].Cmd = "docker compose -f infra.yml up -d --no-deps redis"
	if fixes := rules.ComposeStopsOverShared(cfg); len(fixes) != 0 {
		t.Errorf("fixes = %+v, want none", fixes)
	}
}
