package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func writePortsFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func portsParams(dir string) EnvPortsParams {
	return EnvPortsParams{
		WorktreePath: dir,
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "DATABASE_URL", Job: "svc", Port: "POSTGRES_PORT"},
			{File: "apps/web/.env", Key: "VITE_API_URL", Job: "svc", Port: "API_PORT"},
		},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "POSTGRES_PORT"}: 5432, {Job: "svc", Name: "API_PORT"}: 3000},
		Offset: 10,
		Block:  10,
	}
}

func TestApplyEnvPortsWritesEveryLinkedFile(t *testing.T) {
	dir := t.TempDir()
	writePortsFile(t, dir, ".env", "# db\nDATABASE_URL=postgres://u:pw@localhost:5432/app\nOTHER=keep\n")
	writePortsFile(t, dir, "apps/web/.env", "VITE_API_URL=http://localhost:3000\n")

	plan, err := ApplyEnvPorts(portsParams(dir))
	if err != nil {
		t.Fatalf("ApplyEnvPorts() error = %v", err)
	}
	if len(rules.EnvPortRewrites(plan)) != 2 {
		t.Fatalf("plan rewrote %d values, want 2: %+v", len(rules.EnvPortRewrites(plan)), plan.Entries)
	}

	if got, want := readFile(t, filepath.Join(dir, ".env")), "# db\nDATABASE_URL=postgres://u:pw@localhost:5442/app\nOTHER=keep\n"; got != want {
		t.Errorf(".env =\n%q\nwant\n%q", got, want)
	}
	if got, want := readFile(t, filepath.Join(dir, "apps/web/.env")), "VITE_API_URL=http://localhost:3010\n"; got != want {
		t.Errorf("apps/web/.env =\n%q\nwant\n%q", got, want)
	}
}

func TestApplyEnvPortsLeavesUnlinkedFilesAlone(t *testing.T) {
	dir := t.TempDir()
	writePortsFile(t, dir, ".env", "DATABASE_URL=postgres://localhost:5432/app\n")
	writePortsFile(t, dir, "apps/web/.env", "VITE_API_URL=http://localhost:3000\n")
	writePortsFile(t, dir, "unrelated.env", "PORT=5432\n")

	if _, err := ApplyEnvPorts(portsParams(dir)); err != nil {
		t.Fatalf("ApplyEnvPorts() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "unrelated.env")); got != "PORT=5432\n" {
		t.Errorf("a file no link names was rewritten: %q", got)
	}
}

// A link naming a file the worktree does not have must stay visible rather than
// vanish: it is a mistake in run.toml the user can only fix if told.
func TestComputeEnvPortsReportsAMissingFile(t *testing.T) {
	dir := t.TempDir()
	writePortsFile(t, dir, ".env", "DATABASE_URL=postgres://localhost:5432/app\n")

	plan, err := ComputeEnvPorts(portsParams(dir))
	if err != nil {
		t.Fatalf("ComputeEnvPorts() error = %v", err)
	}

	anomalies := rules.EnvPortAnomalies(plan)
	if len(anomalies) != 1 || anomalies[0].Key != "VITE_API_URL" || anomalies[0].Status != domain.EnvPortStatusMissingKey {
		t.Errorf("Anomalies() = %+v, want the link into the absent file", anomalies)
	}
}

func TestComputeEnvPortsWritesNothing(t *testing.T) {
	dir := t.TempDir()
	const content = "DATABASE_URL=postgres://localhost:5432/app\n"
	writePortsFile(t, dir, ".env", content)

	if _, err := ComputeEnvPorts(portsParams(dir)); err != nil {
		t.Fatalf("ComputeEnvPorts() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dir, ".env")); got != content {
		t.Errorf("ComputeEnvPorts() wrote to the file: %q", got)
	}
}

func TestEnvValueRefsForNarrowsToOneTarget(t *testing.T) {
	got := EnvValueRefsFor(portsParams(t.TempDir()), "apps/web/.env")
	if len(got) != 1 || got["VITE_API_URL"].Base != 3000 {
		t.Errorf("EnvValueRefsFor() = %v, want only the web target's key", got)
	}
}

func TestApplyEnvPortsKeepsQuotesCommentsAndCRLF(t *testing.T) {
	dir := t.TempDir()
	body := "# db\r\nDATABASE_URL='postgres://u:pa$w0rd@localhost:5432/app' # main db\r\nOTHER=\"keep\"\r\n"
	writePortsFile(t, dir, ".env", body)
	writePortsFile(t, dir, "apps/web/.env", "export VITE_API_URL=\"http://localhost:3000\" # api\n")

	if _, err := ApplyEnvPorts(portsParams(dir)); err != nil {
		t.Fatalf("ApplyEnvPorts() error = %v", err)
	}
	if got, want := readFile(t, filepath.Join(dir, ".env")), "# db\r\nDATABASE_URL='postgres://u:pa$w0rd@localhost:5442/app' # main db\r\nOTHER=\"keep\"\r\n"; got != want {
		t.Errorf(".env =\n%q\nwant\n%q", got, want)
	}
	if got, want := readFile(t, filepath.Join(dir, "apps/web/.env")), "export VITE_API_URL=\"http://localhost:3010\" # api\n"; got != want {
		t.Errorf("apps/web/.env =\n%q\nwant\n%q", got, want)
	}
}
