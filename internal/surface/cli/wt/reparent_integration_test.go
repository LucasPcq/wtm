package wt

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// readSourceBranch reads the recorded parent of a worktree from its meta.json.
func readSourceBranch(t *testing.T, stateDir, branch string) string {
	t.Helper()
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, branch), domain.MetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta for %s: %v", branch, err)
	}
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal meta for %s: %v", branch, err)
	}
	return meta.SourceBranch
}

func setupStack(t *testing.T) (dir, stateDir string) {
	t.Helper()
	dir = gittest.InitRepo(t)
	stateDir = filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	for _, step := range []struct{ branch, from string }{
		{"feat", "main"},
		{"dev-a", "feat"},
		{"dev-b", "dev-a"},
	} {
		if _, _, err := runWtCmd(t, domain.CmdCreate, step.branch, "--from", step.from, "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
			t.Fatalf("create %s: %v", step.branch, err)
		}
	}
	return dir, stateDir
}

func TestReparentUpdatesMetadata(t *testing.T) {
	_, stateDir := setupStack(t)

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "dev-a" {
		t.Fatalf("precondition: dev-b parent = %q, want dev-a", got)
	}

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "feat", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("reparent: %v", err)
	}

	results := unmarshalReparent(t, stdout)
	if len(results) != 1 {
		t.Fatalf("expected 1 reparent result, got %d: %+v", len(results), results)
	}
	if results[0].OldParent != "dev-a" || results[0].NewParent != "feat" {
		t.Errorf("unexpected result: %+v", results[0])
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "feat" {
		t.Errorf("dev-b parent = %q, want feat", got)
	}
}

// unmarshalReparent decodes the `reparent` JSON payload ({"reparented":[…]}).
func unmarshalReparent(t *testing.T, stdout string) []domain.ReparentResult {
	t.Helper()
	var payload struct {
		Reparented []domain.ReparentResult `json:"reparented"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal reparent result: %v\n%s", err, stdout)
	}
	return payload.Reparented
}

func TestReparentReparentsMultipleWorktrees(t *testing.T) {
	_, stateDir := setupStack(t)

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-a", "dev-b", "--to", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("batch reparent: %v", err)
	}

	results := unmarshalReparent(t, stdout)
	if len(results) != 2 {
		t.Fatalf("expected 2 reparent results, got %d: %+v", len(results), results)
	}

	for _, branch := range []string{"dev-a", "dev-b"} {
		if got := readSourceBranch(t, stateDir, branch); got != "main" {
			t.Errorf("%s parent = %q, want main", branch, got)
		}
	}
}

func TestReparentBatchRejectsCycle(t *testing.T) {
	_, stateDir := setupStack(t)

	// feat and dev-a onto dev-b closes dev-a → dev-b → dev-a; the whole batch must
	// be rejected and nothing written.
	if _, _, err := runWtCmd(t, domain.CmdReparent, "feat", "dev-a", "--to", "dev-b", "--output", domain.OutputJSON, "--"+domain.FlagYes); err == nil {
		t.Fatalf("expected a cycle error for the batch")
	}
	if got := readSourceBranch(t, stateDir, "dev-a"); got != "feat" {
		t.Errorf("dev-a parent = %q, want feat (unchanged after rejected batch)", got)
	}
}

func TestReparentAcceptsRemoteParent(t *testing.T) {
	dir, stateDir := setupStack(t)

	// Fake an origin remote-tracking branch (no real remote needed).
	cmd := exec.Command("git", "update-ref", "refs/remotes/origin/staging", "HEAD")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %s: %v", out, err)
	}

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "origin/staging", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("reparent onto remote parent: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "origin/staging" {
		t.Errorf("dev-b parent = %q, want origin/staging", got)
	}
}

func TestReparentRejectsUnknownParent(t *testing.T) {
	_, _ = setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "does-not-exist", "--output", domain.OutputJSON, "--"+domain.FlagYes); err == nil {
		t.Fatalf("expected an error reparenting onto a missing branch")
	}
}

func TestReparentRequiresParentInJSONMode(t *testing.T) {
	_, _ = setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--output", domain.OutputJSON, "--"+domain.FlagYes); err == nil {
		t.Fatalf("expected a usage error without --to in JSON mode")
	}
}

func TestCleanReparentsChildrenWithFlag(t *testing.T) {
	_, stateDir := setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdClean, "dev-a", "--yes", "--force", "--reparent-children", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean dev-a: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "feat" {
		t.Errorf("dev-b parent = %q, want feat (grandparent)", got)
	}
}

func TestCleanLeavesChildrenOrphanedWithoutFlag(t *testing.T) {
	_, stateDir := setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdClean, "dev-a", "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean dev-a: %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "dev-a" {
		t.Errorf("dev-b parent = %q, want dev-a (unchanged, no opt-in)", got)
	}
}

// TestCleanYesTextModeReparentDoesNotPrompt is the regression guard for the leak
// where clean derived interactivity from the output format alone: in text mode
// (human format, no TTY) --yes must resolve the reparent decision to the safe
// orphan default without prompting — before the fix this reached the standalone
// confirm and hung. Text mode (no --output json) exercises the leaked path.
func TestCleanYesTextModeReparentDoesNotPrompt(t *testing.T) {
	_, stateDir := setupStack(t)

	if _, _, err := runWtCmd(t, domain.CmdClean, "dev-a", "--yes", "--force"); err != nil {
		t.Fatalf("clean dev-a --yes (text mode): %v", err)
	}

	if got := readSourceBranch(t, stateDir, "dev-b"); got != "dev-a" {
		t.Errorf("dev-b parent = %q, want dev-a (orphaned, no prompt under --yes)", got)
	}
}

// The JSON payload is what an agent reads, so its shape is part of the contract:
// the envelope key, the per-result keys, and nothing extra appearing silently.
func TestReparentJSONShape(t *testing.T) {
	_, _ = setupStack(t)

	stdout, _, err := runWtCmd(t, domain.CmdReparent, "dev-b", "--to", "feat", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("reparent: %v", err)
	}

	var payload map[string][]map[string]any
	if jsonErr := json.Unmarshal([]byte(stdout), &payload); jsonErr != nil {
		t.Fatalf("unmarshal: %v\n%s", jsonErr, stdout)
	}
	results, ok := payload["reparented"]
	if !ok {
		t.Fatalf("payload keys = %v, want a \"reparented\" envelope", keysOf(payload))
	}
	if len(results) != 1 {
		t.Fatalf("results = %v, want one", results)
	}

	want := map[string]any{"branch": "dev-b", "old_parent": "dev-a", "new_parent": "feat"}
	for key, value := range want {
		if results[0][key] != value {
			t.Errorf("result[%q] = %v, want %v", key, results[0][key], value)
		}
	}
	for key := range results[0] {
		if _, expected := want[key]; !expected {
			t.Errorf("unexpected key %q in the payload: an agent's parser must not meet a surprise", key)
		}
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}
