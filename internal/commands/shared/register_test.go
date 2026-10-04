package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func TestLoadConfigRegistersTheRepository(t *testing.T) {
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	state := filepath.Join(dir, ".git", domain.StateDirName)
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, domain.ConfigFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(&cobra.Command{}, dir); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	root, _ := filepath.EvalSymlinks(dir)
	if repos, _ := events.Registered(); len(repos) != 1 || repos[0].Root != root {
		t.Fatalf("registered %+v", repos)
	}
}

// A state dir moved out of the repository (tests, CI) is one the registry
// cannot check is initialized: it would add and prune it on every command.
func TestLoadConfigWithAMovedStateDirRegistersNothing(t *testing.T) {
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, domain.ConfigFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(domain.EnvStateDir, state)

	if _, err := LoadConfig(&cobra.Command{}, dir); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	path, err := infra.RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the registry was written: %v", err)
	}
}

func TestAnUncorrelatedCommandIgnoresTheCallersID(t *testing.T) {
	t.Setenv(domain.EnvCorrelationID, "popup-1")
	plain := &cobra.Command{}
	dashboard := &cobra.Command{Annotations: map[string]string{domain.AnnotationUncorrelated: domain.AnnotationOn}}

	if got := CorrelationID(plain); got != "popup-1" {
		t.Errorf("plain command = %q", got)
	}
	if got := CorrelationID(dashboard); got != "" {
		t.Errorf("uncorrelated command = %q", got)
	}
}
