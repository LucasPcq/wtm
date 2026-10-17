package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func decodeCreated(t *testing.T, stdout string) domain.CreateResult {
	t.Helper()
	var got domain.CreateBatchResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results = %+v, want exactly one", got.Results)
	}
	return got.Results[0]
}

func TestWtCreateBatchJSON(t *testing.T) {
	batchRepo(t)
	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "feat/b", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var got domain.CreateBatchResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode: %v (%q)", err, stdout)
	}
	if len(got.Results) != 2 || got.Failed == nil || len(got.Failed) != 0 {
		t.Errorf("envelope = %+v, want two results and an empty failed array", got)
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

func TestWtCreateBatchPartialFailureJSON(t *testing.T) {
	dir := batchRepo(t)
	occupied := filepath.Join(dir, "../.trees", rules.SanitizeBranchName("feat/b"))
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "feat/b", "feat/c", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want the run reported as failed", err)
	}
	if code := rules.ExitCode(err); code != domain.ExitCodeWorktreeExists {
		t.Errorf("exit code = %d, want the first failure's", code)
	}
	// runWtCmd's root does not set SilenceUsage, so cobra appends its usage block
	// after the returned error; the envelope is the first value.
	var got domain.CreateBatchResult
	if err := json.NewDecoder(strings.NewReader(stdout)).Decode(&got); err != nil {
		t.Fatalf("stdout must still start with one valid envelope: %v (%q)", err, stdout)
	}
	if len(got.Results) != 2 || len(got.Failed) != 1 || got.Failed[0].Branch != "feat/b" {
		t.Errorf("envelope = %+v", got)
	}
}

func TestWtCreateBatchHumanReadout(t *testing.T) {
	batchRepo(t)
	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/a", "feat/b", "--from", "main", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, want := range []string{"feat/a", "feat/b", "2 created"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q should contain %q", stdout, want)
		}
	}
}
