package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func composeFileConfig(cmd, stop string) domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "docker-compose", Kind: domain.JobKindService, Cmd: cmd, Stop: stop, Cwd: ".",
	}}}
}

var twoServiceScan = map[string]domain.ComposeScan{
	"docker-compose.yml": {File: "docker-compose.yml", Services: []domain.ComposeService{{Name: "db"}, {Name: "api"}}},
}

// A re-init whose scope answer changes nothing leaves the file's job as the
// config spells it: the detected `docker-compose` binary, or wtm's own
// recipe, is no reason to rewrite a command the user wrote.
func TestApplySharedServicesKeepsAnUnchangedCommand(t *testing.T) {
	cfg := composeFileConfig("docker compose -f docker-compose.yml up -d --build", "docker compose -f docker-compose.yml down --remove-orphans")

	got := ApplySharedServices(ApplySharedServicesParams{
		Config: cfg, Asked: true, Scans: twoServiceScan, ComposeCmd: "docker-compose",
	})

	if got.Jobs[0].Cmd != cfg.Jobs[0].Cmd {
		t.Errorf("cmd = %q, want the configured %q", got.Jobs[0].Cmd, cfg.Jobs[0].Cmd)
	}
}

// When the answer does change the file's job, the compose binary it already
// invokes is the one kept, for it and for the job lifted out of it.
func TestApplySharedServicesKeepsTheConfiguredComposeBinary(t *testing.T) {
	cfg := composeFileConfig("docker compose -f docker-compose.yml up -d", "docker compose -f docker-compose.yml down --remove-orphans")

	got := ApplySharedServices(ApplySharedServicesParams{
		Config: cfg, Asked: true, Scans: twoServiceScan, ComposeCmd: "docker-compose",
		Shared: []domain.SharedComposeService{{File: "docker-compose.yml", Service: "db"}},
	})

	byName := map[string]domain.JobConfig{}
	for _, job := range got.Jobs {
		byName[job.Name] = job
	}
	if want := "docker compose -f docker-compose.yml up -d " + domain.ComposeNoDeps + "api"; byName["docker-compose"].Cmd != want {
		t.Errorf("file job cmd = %q, want %q", byName["docker-compose"].Cmd, want)
	}
	if want := "docker compose -f docker-compose.yml up -d db"; byName["db"].Cmd != want {
		t.Errorf("lifted job cmd = %q, want %q", byName["db"].Cmd, want)
	}
}
