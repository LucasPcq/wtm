package run

// What a second `run init` — and an edit of one job — must leave alone. The
// wizard rebuilds the config from detection every time, so every project-wide
// setting and every hand-written value is one careless merge away from being
// silently regenerated.

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRunInitKeepsAProjectWideSetting(t *testing.T) {
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"vite"}}`)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Concurrency:     domain.ConcurrencyParallel,
		PortOffsetBlock: 20,
		Jobs: []domain.JobConfig{{
			Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm run dev",
		}},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Concurrency != domain.ConcurrencyParallel {
		t.Errorf("concurrency = %q, want %q — a re-init must not drop a project-wide setting", cfg.Concurrency, domain.ConcurrencyParallel)
	}
	if cfg.PortOffsetBlock != 20 {
		t.Errorf("port_offset_block = %d, want 20", cfg.PortOffsetBlock)
	}
}

func TestRunJobEditKeepsAProjectWideSetting(t *testing.T) {
	stateDir := setupTestProject(t)

	writeRunTOML(t, stateDir, domain.RunConfig{
		Concurrency:     domain.ConcurrencyParallel,
		PortOffsetBlock: 20,
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm run dev"},
			{Name: "web", Kind: domain.JobKindService, Cmd: "vite", Cwd: "apps/web"},
		},
	})

	if _, _, err := runCmd(t, domain.CmdJob, "edit", "dev", "--"+domain.FlagRuns, "web"); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Concurrency != domain.ConcurrencyParallel {
		t.Errorf("concurrency = %q, want %q — editing one job must not drop what the file says about the project", cfg.Concurrency, domain.ConcurrencyParallel)
	}
	if cfg.PortOffsetBlock != 20 {
		t.Errorf("port_offset_block = %d, want 20", cfg.PortOffsetBlock)
	}
}

func TestRunInitKeepsAHandEditedCommand(t *testing.T) {
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"pnpm -r dev"}}`)
	writeProjectFile(t, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")
	writeProjectFile(t, "apps/web/package.json", `{"name":"web","scripts":{"dev":"vite"}}`)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	edited := "pnpm exec vite --port ${VITE_PORT}"
	if _, _, err := runCmd(t, domain.CmdJob, "edit", "web-dev", "--"+domain.FlagCmd, edited); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("second run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	job, found := jobByName(cfg, "web-dev")
	if !found {
		t.Fatalf("no web-dev in %+v", cfg.Jobs)
	}
	if job.Cmd != edited {
		t.Errorf("cmd = %q, want %q — a re-init must not undo what the reader wrote", job.Cmd, edited)
	}
}
