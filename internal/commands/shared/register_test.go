package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
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
