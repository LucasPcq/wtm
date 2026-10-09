package run

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func published(name string, base int, host string) domain.JobConfig {
	return domain.JobConfig{
		Name:  name,
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev --port ${PORT}",
		Ports: map[string]int{"PORT": base},
		URL:   &domain.JobURLConfig{Port: "PORT", Host: host},
	}
}

func TestRunURLPrintsTheOnlyURL(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up", Ports: map[string]int{"PG_PORT": 5432}},
	}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagRaw)
	if err != nil {
		t.Fatalf("run url: %v", err)
	}
	// The port carries the worktree's offset, so the line is asserted by shape:
	// what matters here is that stdout holds the URL and nothing else.
	if !strings.HasPrefix(stdout, "http://localhost:") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("stdout = %q, want the bare URL and nothing else", stdout)
	}
}

// The proxy is on by default, so the plain form is the name — the port is what
// --raw asks for.
func TestRunURLDefaultsToTheNamedForm(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL)
	if err != nil {
		t.Fatalf("run url: %v", err)
	}
	if !strings.HasPrefix(stdout, "http://web.") || !strings.Contains(stdout, ".localhost:") {
		t.Errorf("stdout = %q, want the job published under its own name", stdout)
	}
}

func TestRunURLRawStaysDirect(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagRaw)
	if err != nil {
		t.Fatalf("run url --raw: %v", err)
	}
	if !strings.HasPrefix(stdout, "http://localhost:") {
		t.Errorf("stdout = %q, want the address no proxy has to serve", stdout)
	}
}

func TestRunURLNamesTheJobWhenAmbiguous(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL)
	if !errors.Is(err, domain.ErrJobAmbiguous) {
		t.Fatalf("err = %v, want ErrJobAmbiguous — a machine surface never falls back to a picker", err)
	}
	// The help promises an error naming --job: the refusal has to say how to answer it.
	if !strings.Contains(err.Error(), "--"+domain.FlagJob) {
		t.Errorf("err = %q, want it to name --%s", err, domain.FlagJob)
	}
}

func TestRunURLNamedJobWins(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{
		published("web", 3000, ""),
		published("api", 4000, ""),
	}})
	fakeTTY(t, false)

	web, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagJob, "web", "--"+domain.FlagRaw)
	if err != nil {
		t.Fatalf("run url web: %v", err)
	}
	api, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagJob, "api", "--"+domain.FlagRaw)
	if err != nil {
		t.Fatalf("run url api: %v", err)
	}

	// Both carry the same worktree offset, so the gap between them is the gap
	// between the bases they declared: the named job's own port, not the first.
	if portOf(t, api)-portOf(t, web) != 1000 {
		t.Errorf("web = %q, api = %q — api must answer on its own declared port", web, api)
	}
}

func portOf(t *testing.T, url string) int {
	t.Helper()
	_, port, found := strings.Cut(strings.TrimSpace(url), "http://localhost:")
	if !found {
		t.Fatalf("url %q is not a direct address", url)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port of %q: %v", url, err)
	}
	return n
}

func TestRunURLUnknownJobIsNotDeclared(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagJob, "nope")
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

func TestPublicProxyPortWithoutDaemon(t *testing.T) {
	shortHome(t)

	if got := process.PublicProxyPort(t.Context(), 4000); got != 4000 {
		t.Errorf("sans daemon ni redirection déclarée, le port de bind est la réponse : got %d", got)
	}
	if got := process.PublicProxyPort(t.Context(), 0); got != 0 {
		t.Errorf("proxy éteint : got %d, want 0", got)
	}
}

// --job narrows the document as it narrows the line: a caller asking for one
// job's address must not have to find it in an array.
func TestRunURLJSONHonoursTheJobFlag(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
			{Name: "api", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 4000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		},
	})

	stdout, _, err := runCmd(t, domain.CmdURL, "--"+domain.FlagJob, "web", "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run url: %v", err)
	}

	var entries []domain.JobURLEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, stdout)
	}
	if len(entries) != 1 || entries[0].Job != "web" {
		t.Errorf("entries = %+v, want only web", entries)
	}
}

// A shared job runs once, in the main checkout, on its declared port: the raw
// address of a linked worktree must not shift it by the worktree's offset.
func TestRunURLRawOfASharedJobKeepsItsDeclaredPort(t *testing.T) {
	stateDir := setupTestProject(t)
	db := published("db", 5432, "")
	db.Scope = domain.JobScopeShared
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{db}})
	fakeTTY(t, false)
	enterWorktree(t, addWorktree(t, os.Getenv(domain.EnvProjectDir), "feat/x"))

	stdout, _, err := runCmd(t, domain.CmdURL, "feat/x", "--"+domain.FlagRaw)
	if err != nil {
		t.Fatalf("run url --raw: %v", err)
	}
	if strings.TrimSpace(stdout) != "http://localhost:5432" {
		t.Errorf("stdout = %q, want the shared job's declared port", stdout)
	}
}
