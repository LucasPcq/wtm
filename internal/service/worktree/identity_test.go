package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	g, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("resolve %s: %v", got, err)
	}
	w, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("resolve %s: %v", want, err)
	}
	if g != w {
		t.Fatalf("path = %s, want %s", got, want)
	}
}

func TestTheMainCheckoutIsOrdinalZeroWithNoParent(t *testing.T) {
	repo := newOrdinalRepo(t)

	identity, err := Identity(t.Context(), WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !identity.IsMain || identity.Parent != "" || identity.Ordinal == nil || *identity.Ordinal != 0 {
		t.Fatalf("main identity = %+v", identity)
	}
	assertSamePath(t, identity.Path, repo.dir)
}

func TestAWorktreeNobodyNumberedHasANullOrdinal(t *testing.T) {
	repo := newOrdinalRepo(t)
	path := repo.addWorktree(t, "feat/a")
	if err := writeMetadata(rules.WorktreeMetaDir(repo.stateDir, "feat/a"), domain.WorktreeMetadata{SourceBranch: "main", CreatedAt: "2026-10-03T10:00:00Z", Isolation: domain.IsolationVerbatim}); err != nil {
		t.Fatal(err)
	}
	ref := WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "feat/a"}

	identity, err := Identity(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Ordinal != nil || identity.Parent != "main" || identity.CreatedAt != "2026-10-03T10:00:00Z" || identity.Isolation != domain.IsolationVerbatim || identity.IsMain {
		t.Fatalf("identity = %+v", identity)
	}
	assertSamePath(t, identity.Path, path)

	if _, err := Ordinal(t.Context(), ref); !errors.Is(err, domain.ErrOrdinalUnallocated) {
		t.Fatalf("Ordinal = %v, want ErrOrdinalUnallocated", err)
	}

	repo.ensure(t, "feat/a")
	identity, err = Identity(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Ordinal == nil || *identity.Ordinal != 1 {
		t.Fatalf("ordinal after allocation = %v, want 1", identity.Ordinal)
	}
}

func TestIdentitiesListsMainFirstAndSkipsADetachedWorktree(t *testing.T) {
	repo := newOrdinalRepo(t)
	repo.addWorktree(t, "feat/a")
	git(t, repo.dir, "worktree", "add", "--detach", filepath.Join(t.TempDir(), "detached"))

	identities, err := Identities(t.Context(), IdentitiesParams{ProjectDir: repo.dir, StateDir: repo.stateDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 2 || identities[0].Branch != "main" || identities[1].Branch != "feat/a" {
		t.Fatalf("identities = %+v", identities)
	}
}

func TestTheIdentityOfAnUnknownBranchIsAnError(t *testing.T) {
	repo := newOrdinalRepo(t)
	if _, err := Identity(t.Context(), WorktreeRef{ProjectDir: repo.dir, StateDir: repo.stateDir, Branch: "nope"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestARepoIsKeyedByItsCommonDir(t *testing.T) {
	repo := newOrdinalRepo(t)
	got, err := RepoOf(t.Context(), RepoOfParams{ProjectDir: repo.dir})
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != repo.dir {
		t.Fatalf("root = %s, want %s", got.Root, repo.dir)
	}
	assertSamePath(t, got.CommonDir, filepath.Join(repo.dir, ".git"))
}

// A repository reached through a symlink is still one repository: the key every
// event carries must not depend on how the path was spelled.
func TestARepoHasOneKeyWhateverPathReachesIt(t *testing.T) {
	repo := newOrdinalRepo(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo.dir, link); err != nil {
		t.Fatal(err)
	}

	direct, err := RepoOf(t.Context(), RepoOfParams{ProjectDir: repo.dir})
	if err != nil {
		t.Fatal(err)
	}
	linked, err := RepoOf(t.Context(), RepoOfParams{ProjectDir: link})
	if err != nil {
		t.Fatal(err)
	}
	if direct.CommonDir != linked.CommonDir {
		t.Fatalf("common dir %q through the link, %q directly", linked.CommonDir, direct.CommonDir)
	}
}
