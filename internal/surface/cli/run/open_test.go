package run

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// stubOpener records what a run would have handed to the desktop's opener.
func stubOpener(t *testing.T) *string {
	t.Helper()
	var opened string
	previous := openInBrowser
	t.Cleanup(func() { openInBrowser = previous })
	openInBrowser = func(_ context.Context, url string) error {
		opened = url
		return nil
	}
	return &opened
}

// A single published job is the answer, not a question: nothing is asked and
// nothing is refused, whether there is a terminal or not.
func TestRunOpenTakesTheOnlyPublishedJob(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"PG_PORT": 5432}},
	}})
	fakeTTY(t, false)
	opened := stubOpener(t)

	if _, _, err := runCmd(t, domain.CmdOpen); err != nil {
		t.Fatalf("run open: %v", err)
	}
	if !strings.HasPrefix(*opened, "http://web.") {
		t.Errorf("opened %q, want the only job that publishes a url", *opened)
	}
}

// The refusal names the jobs it could have meant rather than the flag alone: a
// caller told to pass --job still has to know what to pass it.
func TestRunOpenRefusesAnAmbiguityByNamingTheJobs(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)
	opened := stubOpener(t)

	_, _, err := runCmd(t, domain.CmdOpen)
	if !errors.Is(err, domain.ErrJobAmbiguous) {
		t.Fatalf("err = %v, want ErrJobAmbiguous", err)
	}
	if !strings.Contains(err.Error(), "web") || !strings.Contains(err.Error(), "api") {
		t.Errorf("the refusal does not name the published jobs: %v", err)
	}
	if *opened != "" {
		t.Errorf("a refused run opened %q", *opened)
	}
}

func TestRunOpenNamedJobWins(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)
	opened := stubOpener(t)

	if _, _, err := runCmd(t, domain.CmdOpen, "--"+domain.FlagJob, "api"); err != nil {
		t.Fatalf("run open --job api: %v", err)
	}
	if !strings.HasPrefix(*opened, "http://api.") {
		t.Errorf("opened %q, want the named job", *opened)
	}
}

// --raw asks for the address the .env answers on, which no proxy has to serve.
func TestRunOpenRawStaysDirect(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)
	opened := stubOpener(t)

	if _, _, err := runCmd(t, domain.CmdOpen, "--"+domain.FlagRaw); err != nil {
		t.Fatalf("run open --raw: %v", err)
	}
	if !strings.HasPrefix(*opened, "http://localhost:") {
		t.Errorf("opened %q, want the direct address", *opened)
	}
}

func TestRunOpenUnknownJobIsNotDeclared(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)
	stubOpener(t)

	_, _, err := runCmd(t, domain.CmdOpen, "--"+domain.FlagJob, "nope")
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

// The shape follows the format here as it does for `run url`: a caller asking
// for a document must not get an empty stdout and a browser tab.
func TestRunOpenJSONWritesTheAddressItOpened(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)
	opened := stubOpener(t)

	stdout, _, err := runCmd(t, domain.CmdOpen, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run open --output json: %v", err)
	}

	var entries []domain.JobURLEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	if len(entries) != 1 || entries[0].Job != "web" {
		t.Fatalf("entries = %+v, want the one job it opened", entries)
	}
	if entries[0].URL != *opened {
		t.Errorf("document says %q, opened %q", entries[0].URL, *opened)
	}
}
