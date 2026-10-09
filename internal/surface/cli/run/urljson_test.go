package run

import (
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunURLJSONListsEveryPublishedJob(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"PG_PORT": 5432}},
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run url --output json: %v", err)
	}

	var entries []domain.JobURLEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("unmarshal %q: %v", stdout, err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %v, want only the two jobs that publish", entries)
	}
	// JSON never picks: an ambiguity is the caller's to resolve, so every
	// published job is listed rather than one chosen for them.
	if entries[0].Job != "web" || entries[1].Job != "api" {
		t.Errorf("entries = %v, want them in declaration order", entries)
	}
}
