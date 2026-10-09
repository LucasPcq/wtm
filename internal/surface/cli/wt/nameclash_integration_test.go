package wt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The refusal comes before `git worktree add`: nothing is left to clean up.
func TestCreateRefusesAClashingName(t *testing.T) {
	dir := isolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "main", "--yes"); err != nil {
		t.Fatalf("create feat/x: %v", err)
	}

	_, _, err := runWtCmd(t, domain.CmdCreate, "feat.x", "--from", "main", "--yes")
	if !errors.Is(err, domain.ErrWorktreeNameTaken) {
		t.Fatalf("create feat.x = %v, want ErrWorktreeNameTaken", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "..", ".trees", "feat.x")); !os.IsNotExist(statErr) {
		t.Errorf("feat.x was created anyway: %v", statErr)
	}
}
