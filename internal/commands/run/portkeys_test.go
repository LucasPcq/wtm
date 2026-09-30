package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// portedProject is a monorepo whose web app declares a port in run.toml and
// carries it in no .env — the project that has not acquired the habit yet.
func portedProject(t *testing.T) string {
	t.Helper()
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"pnpm -r dev"}}`)
	writeProjectFile(t, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")
	writeProjectFile(t, "apps/web/package.json", `{"name":"web","scripts":{"dev":"vite"}}`)
	writeProjectFile(t, "apps/web/.env", "VITE_API_URL=http://localhost:4001\n")

	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{{
		Name:  "web-dev",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm run dev",
		Cwd:   "apps/web",
		Ports: map[string]int{"VITE_PORT": 5173},
	}}})

	return stateDir
}

func projectFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("WTM_PROJECT_DIR"), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestRunInitWritesThePortKeyUnderTheFlag(t *testing.T) {
	stateDir := portedProject(t)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagWritePortKeys); err != nil {
		t.Fatalf("run init: %v", err)
	}

	if got := projectFile(t, "apps/web/.env"); !strings.Contains(got, "VITE_PORT=5173") {
		t.Fatalf("apps/web/.env = %q, want it to carry VITE_PORT=5173", got)
	}
	if got := projectFile(t, "apps/web/.env"); !strings.Contains(got, "VITE_API_URL=http://localhost:4001") {
		t.Fatalf("the lines it did not touch must survive: %q", got)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	want := domain.EnvPortLink{File: "apps/web/.env", Key: "VITE_PORT", Job: "web-dev", Port: "VITE_PORT"}
	found := false
	for _, link := range cfg.EnvPorts {
		if link == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("links = %+v, want %+v", cfg.EnvPorts, want)
	}
}

func TestRunInitWritesNoPortKeyWithoutTheFlag(t *testing.T) {
	portedProject(t)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	if got := projectFile(t, "apps/web/.env"); strings.Contains(got, "VITE_PORT") {
		t.Fatalf("a committed template is never written without being asked: %q", got)
	}
}

func TestRunInitAddsTheEnvTargetItNeeds(t *testing.T) {
	stateDir := portedProject(t)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagWritePortKeys); err != nil {
		t.Fatalf("run init: %v", err)
	}

	project, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	for _, file := range project.Env.Files {
		if file.Target == "apps/web/.env" {
			return
		}
	}
	t.Fatalf("env files = %+v, want apps/web/.env — a key nothing provisions never reaches a worktree", project.Env.Files)
}

func TestRunInitLeavesTheTemplateItDidNotFind(t *testing.T) {
	portedProject(t)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagWritePortKeys); err != nil {
		t.Fatalf("run init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(os.Getenv("WTM_PROJECT_DIR"), "apps/web/.env.example")); !os.IsNotExist(err) {
		t.Fatal("wtm must not create a committed template nobody asked for")
	}
}

func TestRunInitPortKeysAreIdempotent(t *testing.T) {
	stateDir := portedProject(t)

	for i := 0; i < 2; i++ {
		if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagWritePortKeys); err != nil {
			t.Fatalf("run init #%d: %v", i+1, err)
		}
	}

	if got := strings.Count(projectFile(t, "apps/web/.env"), "VITE_PORT="); got != 1 {
		t.Fatalf("the key must be written once, got %d", got)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	links := 0
	for _, link := range cfg.EnvPorts {
		if link.Key == "VITE_PORT" {
			links++
		}
	}
	if links != 1 {
		t.Fatalf("links = %+v, want exactly one for VITE_PORT", cfg.EnvPorts)
	}

	project, err := config.LoadProjectRaw(stateDir)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	targets := 0
	for _, file := range project.Env.Files {
		if file.Target == "apps/web/.env" {
			targets++
		}
	}
	if targets != 1 {
		t.Fatalf("env files = %+v, want exactly one apps/web/.env", project.Env.Files)
	}
}

// commandRoutedProject declares a port that only the job's command carries: the
// shape whose report used to recommend the very flag that had just fixed it.
func commandRoutedProject(t *testing.T) string {
	t.Helper()
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"pnpm -r dev"}}`)
	writeProjectFile(t, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")
	writeProjectFile(t, "apps/web/package.json", `{"name":"web","scripts":{"dev":"vite"}}`)
	writeProjectFile(t, "apps/web/.env", "VITE_API_URL=http://localhost:4001\n")

	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{{
		Name:  "web-dev",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm exec vite --port ${VITE_PORT}",
		Cwd:   "apps/web",
		Ports: map[string]int{"VITE_PORT": 5173},
	}}})

	return stateDir
}

func TestRunInitNotesTheCommandRouteWhenItWroteNothing(t *testing.T) {
	commandRoutedProject(t)

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	if !strings.Contains(stdout, domain.PortCommandOnlyTitle) {
		t.Fatalf("a job whose port only travels through its command must be named:\n%s", stdout)
	}
}

func TestRunInitDropsTheNoteAboutAJobItJustFixed(t *testing.T) {
	commandRoutedProject(t)

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagWritePortKeys)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	if strings.Contains(stdout, domain.PortCommandOnlyTitle) {
		t.Fatalf("the run wrote that job's key, so telling the reader to run --write-port-keys is telling them to redo what just happened:\n%s", stdout)
	}
	if got := projectFile(t, "apps/web/.env"); !strings.Contains(got, "VITE_PORT=5173") {
		t.Fatalf("apps/web/.env = %q", got)
	}
}
