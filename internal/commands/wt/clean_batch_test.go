package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func createForClean(t *testing.T, branch string) string {
	t.Helper()
	stdout, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create %s: %v", branch, err)
	}
	return decodeCreated(t, stdout).Path
}

func TestWtCleanBatchJSON(t *testing.T) {
	batchRepo(t)
	createForClean(t, "feat/a")
	createForClean(t, "feat/b")

	stdout, _, err := runWtCmd(t, domain.CmdClean, "feat/a", "feat/b", "--yes", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	var got domain.CleanBatchResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode: %v (%q)", err, stdout)
	}
	if len(got.Results) != 2 || got.Failed == nil || len(got.Failed) != 0 || got.Namespaces == nil {
		t.Errorf("envelope = %+v, want two results and empty arrays, never null", got)
	}
}

func TestWtCleanBatchRefusesWhenOneIsUnsafe(t *testing.T) {
	batchRepo(t)
	safe := createForClean(t, "feat/a")
	dirty := createForClean(t, "feat/b")
	if err := os.WriteFile(filepath.Join(dirty, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := runWtCmd(t, domain.CmdClean, "feat/a", "feat/b", "--yes")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("err = %v, want the batch refused naming --force", err)
	}
	if _, statErr := os.Stat(safe); statErr != nil {
		t.Errorf("nothing may be removed: %v", statErr)
	}
}

func TestWtCleanRepeatedArgumentIsAUsageError(t *testing.T) {
	batchRepo(t)
	createForClean(t, "feat/a")

	if _, _, err := runWtCmd(t, domain.CmdClean, "feat/a", "feat/a", "--yes"); !errors.Is(err, domain.ErrUsage) {
		t.Fatalf("err = %v, want ErrUsage", err)
	}
}

func TestWtCleanBatchHumanReadout(t *testing.T) {
	batchRepo(t)
	createForClean(t, "feat/a")
	createForClean(t, "feat/b")

	stdout, _, err := runWtCmd(t, domain.CmdClean, "feat/a", "feat/b", "--yes")
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	for _, want := range []string{"2 removed", "feat/a, feat/b"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q should contain %q", stdout, want)
		}
	}
}

func TestWtCleanBatchFromInsideOneOfThemCdsOnce(t *testing.T) {
	dir := batchRepo(t)
	goFile := filepath.Join(t.TempDir(), "go-file")
	t.Setenv(domain.EnvGoFile, goFile)
	inside := createForClean(t, "feat/a")
	createForClean(t, "feat/b")

	restore := chdir(t, inside)
	_, _, err := runWtCmd(t, domain.CmdClean, "feat/a", "feat/b", "--yes")
	restore()
	if err != nil {
		t.Fatalf("clean: %v", err)
	}

	got, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("read go-file: %v", err)
	}
	if string(got) != dir {
		t.Errorf("go-file = %q, want the base repo %q, once", string(got), dir)
	}
}
