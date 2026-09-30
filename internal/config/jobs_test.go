package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestLoadRunRejectsTypoedSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, domain.RunFileName)
	body := `
[[job]]
name = "dev"
kind = "service"
cmd = "pnpm dev"

[[profiles]]
name = "full"
jobs = ["dev"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected error for typoed [[profiles]] section")
	}
	if !strings.Contains(err.Error(), "profiles") {
		t.Errorf("expected error to mention 'profiles', got: %v", err)
	}
}

func writeRunFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, domain.RunFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadRunReadsPorts(t *testing.T) {
	dir := writeRunFile(t, `
port_offset_block = 100

[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
  [job.ports]
  PORT = 3000

[[job]]
name = "db"
kind = "service"
cmd = "docker compose up -d"
ports = { DB_PORT = 5432 }
`)

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PortOffsetBlock != 100 {
		t.Errorf("got block %d, want 100", cfg.PortOffsetBlock)
	}
	if cfg.Jobs[0].Ports["PORT"] != 3000 {
		t.Errorf("sub-table form: got %v", cfg.Jobs[0].Ports)
	}
	// The encoder only ever writes the sub-table form, but a hand-written inline
	// table is the same TOML and must keep loading.
	if cfg.Jobs[1].Ports["DB_PORT"] != 5432 {
		t.Errorf("inline form: got %v", cfg.Jobs[1].Ports)
	}
}

func TestLoadRunRejectsCollidingPorts(t *testing.T) {
	dir := writeRunFile(t, `
[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 3000, ADMIN = 3010 }
`)

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected the colliding bases to be refused at load")
	}
	for _, want := range []string{"PORT", "ADMIN", "3000", "3010"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// The structural checks stay on write: a file with a duplicate job name must
// still load, or `run job rm` could not repair it.
func TestLoadRunAcceptsDuplicateJobName(t *testing.T) {
	dir := writeRunFile(t, `
[[job]]
name = "dev"
kind = "service"
cmd = "pnpm dev"

[[job]]
name = "dev"
kind = "service"
cmd = "echo dup"
`)

	if _, err := LoadRun(dir); err != nil {
		t.Fatalf("structural errors must not block a read: %v", err)
	}
}

func TestLoadRunRejectsNegativeBlock(t *testing.T) {
	dir := writeRunFile(t, `
port_offset_block = -10

[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
`)

	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected a negative port_offset_block to be refused at load")
	}
	if !strings.Contains(err.Error(), "port_offset_block") {
		t.Errorf("the error should name the key, got: %v", err)
	}
}

// Zero is how an absent block decodes, so it must read as "use the default"
// rather than as a block of nothing — which would put every worktree on the
// main checkout's ports.
func TestLoadRunAcceptsZeroBlock(t *testing.T) {
	dir := writeRunFile(t, `
port_offset_block = 0

[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 3000 }
`)

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatalf("zero must load as the default block: %v", err)
	}
	if got := rules.EffectivePortOffsetBlock(cfg); got != domain.PortOffsetBlock {
		t.Errorf("got block %d, want the default %d", got, domain.PortOffsetBlock)
	}
}

func TestLoadRunReadsJobURL(t *testing.T) {
	dir := t.TempDir()
	content := `
[[job]]
name  = "web"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 3000 }
url   = { port = "PORT" }

[[job]]
name  = "api"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 4000 }
url   = { port = "PORT", host = "api.app-1" }

[[job]]
name  = "db"
kind  = "service"
cmd   = "docker compose up"
ports = { PG_PORT = 5432 }
`
	if err := os.WriteFile(filepath.Join(dir, domain.RunFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRun(dir)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if cfg.Jobs[0].URL == nil || cfg.Jobs[0].URL.Port != "PORT" || cfg.Jobs[0].URL.Host != "" {
		t.Errorf("web url = %+v, want port PORT and no host", cfg.Jobs[0].URL)
	}
	if cfg.Jobs[1].URL == nil || cfg.Jobs[1].URL.Host != "api.app-1" {
		t.Errorf("api url = %+v, want host api.app-1", cfg.Jobs[1].URL)
	}
	if cfg.Jobs[2].URL != nil {
		t.Errorf("db must carry no url, got %+v", cfg.Jobs[2].URL)
	}
}

// A misspelt kind read as neither a service nor a task would run the job under
// rules nobody chose; it is refused where the file is read, naming the job.
func TestLoadRunRejectsAnUnknownKind(t *testing.T) {
	dir := writeRunFile(t, `
[[job]]
name = "api"
kind = "servce"
cmd = "pnpm dev"
`)
	_, err := LoadRun(dir)
	if err == nil {
		t.Fatal("expected kind = \"servce\" to be refused")
	}
	if !strings.Contains(err.Error(), `"api"`) || !strings.Contains(err.Error(), `"servce"`) {
		t.Errorf("error should name the job and the kind, got: %v", err)
	}
}
