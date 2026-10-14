package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// — What git worktree list shows —

func TestWorktreesListsTheMainWorktreeFirstOnItsBranch(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []git.Worktree{
		{Path: repo.Dir, Head: head(t, repo.Dir, "main"), Main: true, State: git.OnBranch{Branch: "main"}},
		{Path: linked, Head: head(t, repo.Dir, "feature"), State: git.OnBranch{Branch: "feature"}},
	}, worktrees)
}

func TestWorktreesKeepsAPathWithASpace(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "with a space")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "spaced", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{repo.Dir, linked}, paths(worktrees))
}

func TestWorktreesReadsADetachedHead(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "detached")
	gittest.Git(t, repo.Dir, "worktree", "add", "--detach", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	require.Len(t, worktrees, 2)
	assert.Equal(t, git.Detached{}, worktrees[1].State)
	assert.Equal(t, head(t, repo.Dir, "main"), worktrees[1].Head)
}

func TestWorktreesMarksALockedAndAPrunableWorktree(t *testing.T) {
	repo := newRepo(t)
	locked := filepath.Join(gittest.TempDir(t), "locked")
	gone := filepath.Join(gittest.TempDir(t), "gone")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "locked", locked)
	gittest.Git(t, repo.Dir, "worktree", "lock", "--reason", "on a stick", locked)
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "gone", gone)
	require.NoError(t, os.RemoveAll(gone))

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	require.Len(t, worktrees, 3)
	assert.True(t, worktrees[1].Locked)
	assert.False(t, worktrees[1].Prunable)
	assert.True(t, worktrees[2].Prunable)
	assert.Equal(t, git.OnBranch{Branch: "gone"}, worktrees[2].State)
}

func TestWorktreesReadsABareRepository(t *testing.T) {
	bare := gittest.TempDir(t)
	gittest.Git(t, bare, "init", "--bare", "-b", "main")

	worktrees, err := git.Repo{Dir: bare, Remote: remote}.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []git.Worktree{{Path: bare, Main: true, State: git.Bare{}}}, worktrees)
}

// — A rebase in progress: HEAD is detached, the branch is in the admin dir —

func TestWorktreesReadsTheBranchOfARebaseInProgress(t *testing.T) {
	cases := []struct {
		name    string
		backend string
		linked  bool
	}{
		{name: "merge backend, in a linked worktree", backend: "--merge", linked: true},
		{name: "apply backend, in the main worktree", backend: "--apply", linked: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newRepo(t)
			commitFile(t, repo.Dir, "file", "base")
			gittest.CreateBranch(t, repo.Dir, "feature")
			commitFile(t, repo.Dir, "file", "main")
			where := checkOutFeature(t, repo.Dir, c.linked)
			commitFile(t, where, "file", "feature")
			rebase(t, where, c.backend)

			worktrees, err := repo.Worktrees(t.Context())

			require.NoError(t, err)
			rebasing := worktrees[len(worktrees)-1]
			assert.Equal(t, where, rebasing.Path)
			assert.Equal(t, git.Rebasing{Branch: "feature"}, rebasing.State)
		})
	}
}

// — The main worktree, from anywhere —

func TestMainWorktreeIsTheFirstEvenSeenFromALinkedOne(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	main, err := git.Repo{Dir: linked, Remote: remote}.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, repo.Dir, main.Path)
	assert.True(t, main.Main)
}

func TestWorktreesFailsOutsideARepository(t *testing.T) {
	repo := git.Repo{Dir: t.TempDir(), Remote: remote}

	_, err := repo.Worktrees(t.Context())
	_, mainErr := repo.MainWorktree(t.Context())

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, 128, gitErr.Exit)
	require.ErrorAs(t, mainErr, &gitErr)
}

// — Where the repository is —

func TestToplevelIsTheRootOfTheWorktreeAsGitWritesIt(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.Symlinked(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	inside := filepath.Join(linked, "sub")
	require.NoError(t, os.Mkdir(inside, 0o755))

	top, err := git.Repo{Dir: inside, Remote: remote}.Toplevel(t.Context())
	common, commonErr := git.Repo{Dir: inside, Remote: remote}.GitCommonDir(t.Context())

	require.NoError(t, err)
	assert.Equal(t, gittest.Resolved(t, linked), top)
	require.NoError(t, commonErr)
	assert.Equal(t, filepath.Join(repo.Dir, ".git"), common)
}

func TestLocateResolvesWhatExistsAndKeepsTheRest(t *testing.T) {
	link := gittest.Symlinked(t)
	require.NoError(t, os.WriteFile(filepath.Join(link, "file"), nil, 0o644))
	repo := git.Repo{Dir: link, Remote: remote}

	file, fileErr := repo.Locate(t.Context(), "file")
	absent, absentErr := repo.Locate(t.Context(), filepath.Join(link, "a", "b"))

	require.NoError(t, fileErr)
	assert.Equal(t, git.Location{Path: filepath.Join(gittest.Resolved(t, link), "file"), Exists: true}, file)
	require.NoError(t, absentErr)
	assert.Equal(t, git.Location{Path: filepath.Join(gittest.Resolved(t, link), "a", "b")}, absent)
}

// — Branches —

func TestBranchesSplitsLocalFromRemoteAndDropsTheRemoteHead(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	gittest.PushBranch(t, repo.Dir, "feature")
	gittest.CreateBranch(t, repo.Dir, "local-only")
	gittest.Git(t, repo.Dir, "remote", "set-head", "origin", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/fork/elsewhere", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin-fork/elsewhere", "main")

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, git.Branches{Local: []string{"feature", "local-only", "main"}, Remote: []string{"feature", "main"}}, branches)
}

func TestBranchesFollowsTheRemoteItIsGiven(t *testing.T) {
	repo := newRepo(t)
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/upstream/feature", "main")
	gittest.Git(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	repo.Remote = "upstream"

	branches, err := repo.Branches(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"feature"}, branches.Remote)
}

// — RefExists: one form, an absence is not an error —

func TestRefExistsTellsAPresentRefFromAnAbsentOne(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.CreateBranch(t, repo.Dir, "local-only")
	cases := []struct {
		name   string
		ref    git.Ref
		exists bool
	}{
		{name: "local branch", ref: git.LocalBranch{Name: "local-only"}, exists: true},
		{name: "absent local branch", ref: git.LocalBranch{Name: "nope"}},
		{name: "remote branch", ref: git.RemoteBranch{Name: "main"}, exists: true},
		{name: "local branch is not on the remote", ref: git.RemoteBranch{Name: "local-only"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exists, err := repo.RefExists(t.Context(), c.ref)

			require.NoError(t, err)
			assert.Equal(t, c.exists, exists)
		})
	}
}

func TestRefExistsFailsRatherThanAnswerAbsent(t *testing.T) {
	outside := git.Repo{Dir: t.TempDir(), Remote: remote}

	_, outsideErr := outside.RefExists(t.Context(), git.LocalBranch{Name: "main"})
	_, noRefErr := newRepo(t).RefExists(t.Context(), nil)

	var gitErr *git.Error
	require.ErrorAs(t, outsideErr, &gitErr)
	assert.Equal(t, 128, gitErr.Exit)
	require.Error(t, noRefErr)
}

// — AheadBehind —

func TestAheadBehindCountsBothSides(t *testing.T) {
	repo := newRepo(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	commitFile(t, repo.Dir, "main-only", "1")
	gittest.Git(t, repo.Dir, "checkout", "feature")
	commitFile(t, repo.Dir, "feature-only", "1")
	commitFile(t, repo.Dir, "feature-only", "2")

	counts, err := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: git.LocalBranch{Name: "feature"}, Against: git.LocalBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, git.AheadBehind{Ahead: 2, Behind: 1}, counts)
}

func TestAheadBehindAgainstTheRemoteBranch(t *testing.T) {
	repo, origin := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	advanceOnOrigin(t, origin, "main")
	require.NoError(t, repo.FetchRef(t.Context(), "main"))

	counts, err := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: git.LocalBranch{Name: "main"}, Against: git.RemoteBranch{Name: "main"}})

	require.NoError(t, err)
	assert.Equal(t, git.AheadBehind{Ahead: 1, Behind: 1}, counts)
}

func TestAheadBehindFailsOnAnUnknownRefRatherThanCountZero(t *testing.T) {
	repo := newRepo(t)
	main := git.LocalBranch{Name: "main"}

	_, unknownErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: main, Against: git.RemoteBranch{Name: "main"}})
	_, noOfErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Against: main})
	_, noAgainstErr := repo.AheadBehind(t.Context(), git.AheadBehindParams{Of: main})

	var gitErr *git.Error
	require.ErrorAs(t, unknownErr, &gitErr)
	require.Error(t, noOfErr)
	require.Error(t, noAgainstErr)
}
