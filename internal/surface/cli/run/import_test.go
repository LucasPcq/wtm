package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestReadImportSourceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "layout.json")
	content := []byte(`{"job":[],"profile":[]}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readImportSource([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestReadImportSourceMissingFile(t *testing.T) {
	_, err := readImportSource([]string{"/nonexistent/path/layout.json"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestImportReplacesTheWholeConfig(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		PortOffsetBlock: 50,
		Jobs:            []domain.JobConfig{{Name: "old", Kind: domain.JobKindService, Cmd: "true"}},
		EnvPorts:        []domain.EnvPortLink{{File: ".env", Key: "OLD_PORT", Job: "old", Port: "PORT"}},
	})

	payload := filepath.Join(t.TempDir(), "layout.json")
	body := `{"port_offset_block":10,"job":[{"name":"web","kind":"service","cmd":"pnpm dev","ports":{"PORT":3000}}],` +
		`"profile":[{"name":"front","jobs":["web"],"default":false}],` +
		`"env_port":[{"file":".env","key":"WEB_PORT","job":"web","port":"PORT"}]}`
	if err := os.WriteFile(payload, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, domain.CmdImport, payload, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run import: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "web" {
		t.Errorf("Jobs = %+v, want only the incoming web job", cfg.Jobs)
	}
	if cfg.PortOffsetBlock != 10 {
		t.Errorf("PortOffsetBlock = %d, want the payload's 10", cfg.PortOffsetBlock)
	}
	if len(cfg.EnvPorts) != 1 || cfg.EnvPorts[0].Key != "WEB_PORT" {
		t.Errorf("EnvPorts = %+v, want only the payload's link", cfg.EnvPorts)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "front" {
		t.Errorf("Profiles = %+v, want only the payload's profile", cfg.Profiles)
	}
}

func TestImportJSONRequiresYes(t *testing.T) {
	setupTestProject(t)

	payload := filepath.Join(t.TempDir(), "layout.json")
	if err := os.WriteFile(payload, []byte(`{"job":[],"profile":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCmd(t, domain.CmdImport, payload, "--"+domain.FlagOutput, domain.OutputJSON)
	if err == nil {
		t.Fatal("expected --output json without --yes to be refused")
	}
	if !strings.Contains(err.Error(), domain.FlagYes) {
		t.Errorf("error %q must name --%s", err, domain.FlagYes)
	}
}
