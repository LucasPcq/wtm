package runjobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/rules"
)

func TestTracesNamesWhatEachWorktreeLeftOnDisk(t *testing.T) {
	stateDir := t.TempDir()
	dir := rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: stateDir, Branch: "feat/x"})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, rules.JobLogFileName("web")), []byte("ready\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logged := Traces(TracesParams{StateDir: stateDir, Branches: []string{"feat/x", "idle"}})
	if !logged["feat/x"]["web"] {
		t.Errorf("logged = %v, want web traced in feat/x", logged)
	}
	if len(logged["idle"]) != 0 {
		t.Errorf("logged[idle] = %v, want nothing for a worktree that ran nothing", logged["idle"])
	}
}
