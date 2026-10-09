package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type occupiedRepo struct {
	dir    string
	config domain.Config
}

func newOccupiedRepo(t *testing.T) occupiedRepo {
	t.Helper()
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	return occupiedRepo{dir: gittest.InitRepo(t), config: config}
}

func (r occupiedRepo) create(t *testing.T, branchName string, ifNotExists bool) (domain.CreateResult, error) {
	t.Helper()
	return Create(t.Context(), domain.CreateParams{
		ProjectDir: r.dir, StateDir: filepath.Join(r.dir, ".git", "wtm"), Branch: branchName, FromBranch: "main",
		SourceBranch: "main", Config: r.config, SkipHooks: true, IfNotExists: ifNotExists,
	})
}

// feat/x and feat-x share a folder: the worktree already there belongs to the
// other branch, and handing out its path as an idempotent success put an agent
// to work in the wrong checkout.
func TestCreateRefusesAPathAnotherBranchHolds(t *testing.T) {
	for _, ifNotExists := range []bool{false, true} {
		repo := newOccupiedRepo(t)
		if _, err := repo.create(t, "feat-x", false); err != nil {
			t.Fatal(err)
		}

		result, err := repo.create(t, "feat/x", ifNotExists)

		if !errors.Is(err, domain.ErrWorktreeNameTaken) {
			t.Errorf("if-not-exists=%v: err = %v, want ErrWorktreeNameTaken", ifNotExists, err)
		}
		if result.Path != "" || result.AlreadyExists {
			t.Errorf("if-not-exists=%v: result = %+v, want nothing of feat-x's", ifNotExists, result)
		}
	}
}

func TestCreateIfNotExistsReturnsTheBranchOwnWorktree(t *testing.T) {
	repo := newOccupiedRepo(t)
	created, err := repo.create(t, "feat/x", false)
	if err != nil {
		t.Fatal(err)
	}

	again, err := repo.create(t, "feat/x", true)

	if err != nil || !again.AlreadyExists || infra.ResolvePath(again.Path) != infra.ResolvePath(created.Path) {
		t.Fatalf("result = %+v, err = %v, want feat/x's own worktree as already existing", again, err)
	}
}

// A directory no worktree is registered at is nobody's checkout to hand back.
func TestCreateRefusesAStrayDirectoryEvenIfNotExists(t *testing.T) {
	repo := newOccupiedRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.dir, repo.config.Project.Worktrees.BasePath, rules.SanitizeBranchName("feat/x")), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := repo.create(t, "feat/x", true)

	if !errors.Is(err, domain.ErrWorktreePathExists) || result.AlreadyExists {
		t.Fatalf("result = %+v, err = %v, want ErrWorktreePathExists", result, err)
	}
}
