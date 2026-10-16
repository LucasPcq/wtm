package wt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func batchRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	return dir
}

func TestWtCreateBatchJSON(t *testing.T) {
	batchRepo(t)
	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "feat/b", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var got struct {
		OK    bool `json:"ok"`
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode: %v (%q)", err, stdout)
	}
	if !got.OK || len(got.Items) != 2 || got.Items[0].Status != "done" || got.Items[1].Status != "done" {
		t.Errorf("envelope = %+v, want two items done", got)
	}
}

func TestWtCreateSingleBranchJSONIsAnEnvelopeToo(t *testing.T) {
	batchRepo(t)
	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created := decodeCreated(t, stdout); created.Branch != "feat/a" {
		t.Errorf("results[0] = %+v", created)
	}
}

// A branch whose folder is taken makes the request invalid: the batch is refused
// whole before anything is created, the entry named.
func TestWtCreateBatchWithATakenFolderIsRefusedWhole(t *testing.T) {
	dir := batchRepo(t)
	occupied := filepath.Join(dir, "../.trees", rules.SanitizeBranchName("feat/b"))
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "feat/b", "feat/c", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)

	if code := fieldCodes(t, err)["branches[1]"]; code != create.CodePathExists {
		t.Errorf("feat/b refused on %q, want %q", code, create.CodePathExists)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "../.trees", rules.SanitizeBranchName("feat/a"))); !os.IsNotExist(statErr) {
		t.Errorf("feat/a was created anyway: %v", statErr)
	}
}
