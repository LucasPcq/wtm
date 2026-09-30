package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestApplyEnvPortsWritesAnOwnedKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("POSTGRES_PORT=5432\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := ApplyEnvPorts(EnvPortsParams{
		WorktreePath: dir,
		Owned: []domain.EnvOwnedEntry{
			{File: ".env", Key: domain.EnvComposeProjectName, Value: "repo-feat-x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "COMPOSE_PROJECT_NAME=repo-feat-x"; !strings.Contains(string(content), want) {
		t.Fatalf("got %q, want it to contain %q", content, want)
	}
	if len(plan.Owned) != 1 || !plan.Owned[0].Changed {
		t.Fatalf("the plan must report the write: %+v", plan.Owned)
	}
}

func TestApplyEnvPortsRewritesAnOwnedKeyInPlace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("COMPOSE_PROJECT_NAME=repo\nPOSTGRES_PORT=5432\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ApplyEnvPorts(EnvPortsParams{
		WorktreePath: dir,
		Owned:        []domain.EnvOwnedEntry{{File: ".env", Key: domain.EnvComposeProjectName, Value: "repo-feat-x"}},
	}); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if got := strings.Count(string(content), "COMPOSE_PROJECT_NAME="); got != 1 {
		t.Fatalf("the key must be rewritten, not appended: %q", content)
	}
}

func TestComputeEnvPortsLeavesAnOwnedKeyItAlreadyHolds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("COMPOSE_PROJECT_NAME=repo-feat-x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := ComputeEnvPorts(EnvPortsParams{
		WorktreePath: dir,
		Owned:        []domain.EnvOwnedEntry{{File: ".env", Key: domain.EnvComposeProjectName, Value: "repo-feat-x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Owned) != 1 || plan.Owned[0].Changed {
		t.Fatalf("nothing to change: %+v", plan.Owned)
	}
}

func TestEnvPortsParamsAreNotEmptyWithAnOwnedKey(t *testing.T) {
	params := EnvPortsParams{Owned: []domain.EnvOwnedEntry{{File: ".env", Key: domain.EnvComposeProjectName, Value: "x"}}}
	if params.Empty() {
		t.Fatal("a project with no link still has its compose name to write")
	}
}

func TestApplyOwnedEnvWritesTheIdentityAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("POSTGRES_PORT=5432\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ApplyOwnedEnv(EnvPortsParams{
		WorktreePath: dir,
		Owned:        []domain.EnvOwnedEntry{{File: ".env", Key: domain.EnvComposeProjectName, Value: "repo-feat-x"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if !strings.Contains(string(content), "COMPOSE_PROJECT_NAME=repo-feat-x") {
		t.Fatalf("got %q", content)
	}
	if !strings.Contains(string(content), "POSTGRES_PORT=5432") {
		t.Fatalf("the ports it did not touch must survive: %q", content)
	}
}
