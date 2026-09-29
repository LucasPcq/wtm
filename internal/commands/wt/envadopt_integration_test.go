package wt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const legacyTemplate = "COMPOSE_PROJECT_NAME=placeholder-name\nWEB_PORT=3000\nREALM=\n"

// legacyWorktree is a worktree as v0.27 left it: its .env copied from main,
// and a meta.json that never recorded an isolation nor an ordinal.
func legacyWorktree(t *testing.T) (dir, path string) {
	t.Helper()
	dir = isolationRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")

	project := "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\ntemplate = \".env.example\"\n\n[hooks]\non_create = []\n"
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d db", Ports: map[string]int{"DB_PORT": 5432}},
		},
		EnvPorts:  []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM", Job: "web", Value: "app-{worktree}"}},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(legacyTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".env.example")
	gitRun(t, dir, "commit", "-q", "-m", "template")

	path = filepath.Join(dir, "..", ".trees", "feat-old")
	gitRun(t, dir, "worktree", "add", "-q", "-b", "feat/old", path)
	if err := os.WriteFile(filepath.Join(path, ".env"), []byte(mainEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	metaDir := rules.WorktreeMetaDir(stateDir, "feat/old")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyMeta := `{"source_branch": "main", "created_at": "2026-01-01T00:00:00Z", "env_strategy": "main"}`
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte(legacyMeta), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func legacyMeta(t *testing.T, dir string) domain.WorktreeMetadata {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rules.WorktreeMetaDir(filepath.Join(dir, ".git", "wtm"), "feat/old"), domain.MetaFileName))
	if err != nil {
		t.Fatal(err)
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatal(err)
	}
	return meta
}

func envResult(t *testing.T, out string) domain.EnvSyncResult {
	t.Helper()
	var result domain.EnvSyncResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode env result: %v\n%s", err, out)
	}
	return result
}

// G2: a worktree that never chose its isolation keeps its source's ports and
// compose project under --yes; only the keys are reconciled, and the run says
// how to adopt isolation.
func TestEnvYesLeavesALegacyWorktreesRunValuesAlone(t *testing.T) {
	dir, path := legacyWorktree(t)

	out, _, err := runWtCmd(t, domain.CmdEnv, "feat/old", "--yes", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("env: %v", err)
	}

	body, _ := os.ReadFile(filepath.Join(path, ".env"))
	if strings.Contains(string(body), domain.EnvComposeProjectName) || !strings.Contains(string(body), "WEB_PORT=3000") || !strings.Contains(string(body), "REALM=main-realm") {
		t.Errorf(".env = %q, want its ports, realm and compose project untouched", body)
	}
	if meta := legacyMeta(t, dir); meta.Ordinal != 0 || meta.Isolation != "" {
		t.Errorf("meta = %+v, want no ordinal and no isolation recorded", meta)
	}

	result := envResult(t, out)
	if result.IsolationAdoption != domain.IsolationNotAdopted || result.Isolation != "" {
		t.Errorf("isolation_adoption = %q, isolation = %q, want %q and none", result.IsolationAdoption, result.Isolation, domain.IsolationNotAdopted)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "wtm env feat/old --isolation isolated") {
		t.Errorf("warnings = %v, want the command adopting isolation", result.Warnings)
	}

	human, _, err := runWtCmd(t, domain.CmdEnv, "feat/old", "--yes")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if !strings.Contains(human, "--isolation isolated") || strings.Contains(human, "Settled") {
		t.Errorf("report = %q, want the adoption hint and no settled value", human)
	}
}

// --isolation isolated is the explicit adoption: the ports move, the compose
// project is written, and the report names it.
func TestEnvIsolationIsolatedAdoptsALegacyWorktree(t *testing.T) {
	dir, path := legacyWorktree(t)

	out, _, err := runWtCmd(t, domain.CmdEnv, "feat/old", "--yes", "--"+domain.FlagIsolation, "isolated", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("env: %v", err)
	}

	want := rules.ComposeProjectName(rules.ComposeProjectNameParams{Project: filepath.Base(dir), Worktree: "feat-old"})
	body, _ := os.ReadFile(filepath.Join(path, ".env"))
	if !strings.Contains(string(body), domain.EnvComposeProjectName+"="+want) || strings.Contains(string(body), "WEB_PORT=3000") {
		t.Errorf(".env = %q, want %s=%s and the port shifted", body, domain.EnvComposeProjectName, want)
	}
	if meta := legacyMeta(t, dir); meta.Isolation != domain.IsolationIsolated {
		t.Errorf("recorded isolation = %q, want %q", meta.Isolation, domain.IsolationIsolated)
	}
	if result := envResult(t, out); result.IsolationAdoption != domain.IsolationAdopted {
		t.Errorf("isolation_adoption = %q, want %q", result.IsolationAdoption, domain.IsolationAdopted)
	}
}

func TestEnvReportNamesTheComposeProjectItWrites(t *testing.T) {
	dir, _ := legacyWorktree(t)

	human, _, err := runWtCmd(t, domain.CmdEnv, "feat/old", "--yes", "--"+domain.FlagIsolation, "isolated")
	if err != nil {
		t.Fatalf("env: %v", err)
	}

	want := rules.ComposeProjectName(rules.ComposeProjectNameParams{Project: filepath.Base(dir), Worktree: "feat-old"})
	if !strings.Contains(human, domain.EnvComposeProjectName) || !strings.Contains(human, want) {
		t.Errorf("report = %q, want %s %s named", human, domain.EnvComposeProjectName, want)
	}
	if strings.Contains(human, "placeholder-name") {
		t.Errorf("report = %q, want no placeholder offered for a key wtm writes", human)
	}
	if strings.Contains(human, domain.EnvFileKeysInSyncMessage) {
		t.Errorf("report = %q, want no file claiming its values still move after they were settled", human)
	}
}

// Adding main's COMPOSE_PROJECT_NAME is a "safe addition" to the reconciliation
// and the one write that would wire a legacy worktree to main's stack.
func TestEnvYesNeverHandsALegacyWorktreeAComposeProject(t *testing.T) {
	dir, path := legacyWorktree(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(domain.EnvComposeProjectName+"=main-stack\n"+mainEnv+"NEW_KEY=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/old", "--yes"); err != nil {
		t.Fatalf("env: %v", err)
	}

	body, _ := os.ReadFile(filepath.Join(path, ".env"))
	if strings.Contains(string(body), domain.EnvComposeProjectName) || !strings.Contains(string(body), "NEW_KEY=1") {
		t.Errorf(".env = %q, want the new key added and no compose project", body)
	}
}
