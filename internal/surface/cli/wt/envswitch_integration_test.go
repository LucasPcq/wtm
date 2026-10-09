package wt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// composeIsolationRepo adds a compose job to isolationRepo, so an isolated
// worktree's .env also carries a COMPOSE_PROJECT_NAME wtm wrote.
func composeIsolationRepo(t *testing.T) string {
	t.Helper()
	dir := isolationRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Jobs = append(cfg.Jobs, domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up"})
	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Force: true, Config: cfg}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func recordedIsolation(dir, branch string) domain.Isolation {
	return worktree.RecordedIsolation(worktree.WorktreeRef{
		ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Branch: branch,
	})
}

func worktreeEnvPath(dir, branch string) string {
	return filepath.Join(dir, "..", ".trees", rules.SanitizeBranchName(branch), ".env")
}

// G5: switching an isolated worktree to verbatim puts back the values wtm
// wrote — the port, the [[env]] value, the compose project — and nothing else.
func TestEnvVerbatimPutsTheOwnedValuesBack(t *testing.T) {
	dir := composeIsolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/i", "--from", "main", "--yes"); err != nil {
		t.Fatalf("create: %v", err)
	}
	isolated := worktreeEnv(t, dir, "feat/i")
	if !strings.Contains(isolated, domain.EnvComposeProjectName) {
		t.Fatalf("setup: .env = %q, want a compose project written", isolated)
	}
	if err := os.WriteFile(worktreeEnvPath(dir, "feat/i"), []byte(isolated+"MINE='kept $1'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runWtCmd(t, domain.CmdEnv, "feat/i", "--yes", "--output", domain.OutputJSON, "--"+domain.FlagIsolation, "verbatim")
	if err != nil {
		t.Fatalf("env --isolation verbatim: %v", err)
	}
	if got, want := worktreeEnv(t, dir, "feat/i"), mainEnv+"MINE='kept $1'\n"; got != want {
		t.Errorf(".env = %q, want %q", got, want)
	}
	if got := recordedIsolation(dir, "feat/i"); got != domain.IsolationVerbatim {
		t.Errorf("recorded isolation = %q, want verbatim", got)
	}

	var result domain.EnvSyncResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !result.IsolationChanged || result.Isolation != domain.IsolationVerbatim {
		t.Errorf("isolation = %q changed=%v, want verbatim, changed", result.Isolation, result.IsolationChanged)
	}
	restored := map[string]domain.EnvRestoredEntry{}
	for _, entry := range result.Restored {
		restored[entry.Key] = entry
	}
	if len(restored) != 3 || !slices.Equal(restored["WEB_PORT"].Ports, []int{3000}) || restored["REALM"].Key == "" || !restored[domain.EnvComposeProjectName].Removed {
		t.Errorf("restored = %+v, want WEB_PORT, REALM and COMPOSE_PROJECT_NAME", result.Restored)
	}

	if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/i", "--yes", "--"+domain.FlagIsolation, "verbatim"); err != nil {
		t.Fatalf("second env: %v", err)
	}
	if got, want := worktreeEnv(t, dir, "feat/i"), mainEnv+"MINE='kept $1'\n"; got != want {
		t.Errorf("second run: .env = %q, want it unchanged", got)
	}
}

func TestEnvVerbatimReportsTheSwitch(t *testing.T) {
	isolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/i", "--from", "main", "--yes"); err != nil {
		t.Fatalf("create: %v", err)
	}
	human, _, err := runWtCmd(t, domain.CmdEnv, "feat/i", "--yes", "--"+domain.FlagIsolation, "verbatim")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if strings.Contains(human, domain.EnvNothingWrittenMessage) || !strings.Contains(human, "WEB_PORT") {
		t.Errorf("report = %q, want the restored keys named and no 'no changes'", human)
	}
}

// The record follows the .env, never precedes it: a run that fails leaves the
// worktree recorded as it was.
func TestEnvIsolationIsNotRecordedWhenTheRunFails(t *testing.T) {
	cases := []struct {
		name   string
		create string
		target domain.Isolation
	}{
		{"isolated to verbatim", "isolated", domain.IsolationVerbatim},
		{"verbatim to isolated", "verbatim", domain.IsolationIsolated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := isolationRepo(t)
			if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/f", "--from", "main", "--yes", "--"+domain.FlagIsolation, tc.create); err != nil {
				t.Fatalf("create: %v", err)
			}
			path := worktreeEnvPath(dir, "feat/f")
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

			if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/f", "--yes", "--"+domain.FlagIsolation, string(tc.target)); err == nil {
				t.Fatal("expected the unreadable .env to fail the run")
			}
			if got := recordedIsolation(dir, "feat/f"); got != domain.Isolation(tc.create) {
				t.Errorf("recorded isolation = %q, want %q kept", got, tc.create)
			}
		})
	}
}
