package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreesListsTheMainWorktreeFirstOnItsBranch(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []worktree.Worktree{
		{Path: repo.Dir, Main: true, State: worktree.OnBranch{Branch: "main"}},
		{Path: linked, State: worktree.OnBranch{Branch: "feature"}},
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
	assert.Equal(t, worktree.Detached{}, worktrees[1].State)
}

func TestWorktreesReadsAWorktreeWhoseDirectoryIsGoneAsOnItsBranch(t *testing.T) {
	repo := newRepo(t)
	gone := filepath.Join(gittest.TempDir(t), "gone")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "gone", gone)
	require.NoError(t, os.RemoveAll(gone))

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	require.Len(t, worktrees, 2)
	assert.Equal(t, worktree.OnBranch{Branch: "gone"}, worktrees[1].State)
}

func TestTheFakeAndGitListALockedWorktreeAndAGoneOneUnlockedAsPrunable(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			worktrees, err := impl.port.Worktrees(t.Context())

			require.NoError(t, err)
			var locked, prunable []string
			for _, w := range worktrees {
				if w.Locked {
					locked = append(locked, w.Path)
				}
				if w.Prunable {
					prunable = append(prunable, w.Path)
				}
			}
			assert.Equal(t, []string{impl.at("locked"), impl.at("locked-gone")}, locked)
			assert.Equal(t, []string{impl.at("gone")}, prunable)
		})
	}
}

func TestWorktreesReadsABareRepository(t *testing.T) {
	bare := gittest.TempDir(t)
	gittest.Git(t, bare, "init", "--bare", "-b", "main")

	worktrees, err := git.Repo{Dir: bare}.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []worktree.Worktree{{Path: bare, Main: true, State: worktree.Bare{}}}, worktrees)
}

func TestWorktreesReadsTheBranchOfARebaseInProgress(t *testing.T) {
	cases := []struct {
		name    string
		backend string
		linked  bool
	}{
		{name: "merge_backend_linked", backend: "--merge", linked: true},
		{name: "apply_backend_main", backend: "--apply", linked: false},
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
			assert.Equal(t, worktree.Rebasing{Branch: "feature"}, rebasing.State)
		})
	}
}

func TestARebaseBegunDetachedIsOnNoBranch(t *testing.T) {
	repo := newRepo(t)
	commitFile(t, repo.Dir, "file", "base")
	gittest.CreateBranch(t, repo.Dir, "feature")
	commitFile(t, repo.Dir, "file", "main")
	where := checkOutFeature(t, repo.Dir, true)
	commitFile(t, where, "file", "feature")
	gittest.Git(t, where, "checkout", "-q", "--detach")
	rebase(t, where, "--merge")

	worktrees, err := repo.Worktrees(t.Context())

	require.NoError(t, err)
	assert.Equal(t, worktree.Detached{}, worktrees[len(worktrees)-1].State)
}

func TestWorktreesReadsTheBranchABisectStartedFrom(t *testing.T) {
	cases := []struct {
		name     string
		detached bool
		want     worktree.State
	}{
		{name: "from_branch", want: worktree.Bisecting{Branch: "feature"}},
		{name: "from_detached_head", detached: true, want: worktree.Detached{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newRepo(t)
			gittest.CreateBranch(t, repo.Dir, "feature")
			where := checkOutFeature(t, repo.Dir, true)
			good := head(t, where, "HEAD")
			for _, content := range []string{"1", "2", "3"} {
				commitFile(t, where, "file", content)
			}
			if c.detached {
				gittest.Git(t, where, "checkout", "-q", "--detach")
			}
			gittest.Git(t, where, "bisect", "start", "HEAD", good)

			worktrees, err := repo.Worktrees(t.Context())

			require.NoError(t, err)
			assert.Equal(t, c.want, worktrees[len(worktrees)-1].State)
		})
	}
}

func TestMainWorktreeIsTheFirstEvenSeenFromALinkedOne(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.TempDir(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)

	main, err := git.Repo{Dir: linked}.MainWorktree(t.Context())

	require.NoError(t, err)
	assert.Equal(t, repo.Dir, main.Path)
	assert.True(t, main.Main)
}

func TestWorktreesFailsOutsideARepository(t *testing.T) {
	repo := git.Repo{Dir: t.TempDir()}

	_, err := repo.Worktrees(t.Context())
	_, mainErr := repo.MainWorktree(t.Context())

	var gitErr *git.Error
	require.ErrorAs(t, err, &gitErr)
	assert.Equal(t, 128, gitErr.Exit)
	require.ErrorAs(t, mainErr, &gitErr)
}

func TestTheCommonDirIsTheMainRepositorysEvenFromInsideALinkedWorktree(t *testing.T) {
	repo := newRepo(t)
	linked := filepath.Join(gittest.Symlinked(t), "feature")
	gittest.Git(t, repo.Dir, "worktree", "add", "-b", "feature", linked)
	inside := filepath.Join(linked, "sub")
	require.NoError(t, os.Mkdir(inside, 0o755))

	common, err := git.Repo{Dir: inside}.CommonDir(t.Context())

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repo.Dir, ".git"), common)
}

func TestLocateResolvesWhatExistsAndKeepsTheRest(t *testing.T) {
	link := gittest.Symlinked(t)
	require.NoError(t, os.WriteFile(filepath.Join(link, "file"), nil, 0o644))
	repo := git.Repo{Dir: link}

	file, fileErr := repo.Locate(t.Context(), "file")
	absent, absentErr := repo.Locate(t.Context(), filepath.Join(link, "a", "b"))

	require.NoError(t, fileErr)
	assert.Equal(t, worktree.Location{Path: filepath.Join(gittest.Resolved(t, link), "file"), Exists: true}, file)
	require.NoError(t, absentErr)
	assert.Equal(t, worktree.Location{Path: filepath.Join(gittest.Resolved(t, link), "a", "b")}, absent)
}

func TestAnAddOnANewBranchStartsFromItWithoutTrackingIt(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	commitFile(t, repo.Dir, "local", "1")
	path := filepath.Join(gittest.TempDir(t), "topic")

	err := repo.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.NewBranch{Name: "topic", From: worktree.RemoteBranch{Name: "main"}}})

	require.NoError(t, err)
	assert.Equal(t, head(t, repo.Dir, "origin/main"), head(t, path, "HEAD"))
	assert.Equal(t, "topic", gittest.Output(t, path, "branch", "--show-current"))
	assert.Empty(t, upstreamOf(t, repo.Dir, "topic"))
}

func TestAnAddOfABranchOnlyTheRemoteHasTracksIt(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.Git(t, repo.Dir, "push", "-q", "origin", "main:remote-only")
	gittest.Git(t, repo.Dir, "fetch", "-q", "origin")
	path := filepath.Join(gittest.TempDir(t), "remote-only")

	err := repo.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.TrackingBranch{Name: "remote-only"}})

	require.NoError(t, err)
	assert.Equal(t, head(t, repo.Dir, "origin/remote-only"), head(t, path, "HEAD"))
	assert.Contains(t, upstreamOf(t, repo.Dir, "remote-only"), "refs/heads/remote-only")
}

func TestAnAddChecksOutAnExistingBranch(t *testing.T) {
	repo := newRepo(t)
	gittest.CreateBranch(t, repo.Dir, "feature")
	path := filepath.Join(gittest.TempDir(t), "feature")

	err := repo.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.ExistingBranch{Name: "feature"}})

	require.NoError(t, err)
	assert.Equal(t, "feature", gittest.Output(t, path, "branch", "--show-current"))
}

func TestAnAddRefusesABranchOnlyTheRemoteHasRatherThanCreateOne(t *testing.T) {
	repo, _ := newRepoWithOrigin(t)
	gittest.Git(t, repo.Dir, "push", "origin", "main:remote-only")
	gittest.Git(t, repo.Dir, "fetch", "origin")

	err := repo.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(gittest.TempDir(t), "remote-only"), Checkout: worktree.ExistingBranch{Name: "remote-only"}})

	require.ErrorIs(t, err, git.ErrNoLocalBranch)
	assert.EqualError(t, err, "no local branch: remote-only")
	assert.False(t, has(t, repo, worktree.LocalBranch{Name: "remote-only"}))
}

func TestAnAddWithoutACheckoutOrAStartPointIsRefused(t *testing.T) {
	repo := newRepo(t)

	noFrom := repo.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(gittest.TempDir(t), "topic"), Checkout: worktree.NewBranch{Name: "topic"}})
	noCheckout := repo.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(gittest.TempDir(t), "none")})

	require.Error(t, noFrom)
	require.Error(t, noCheckout)
}

func TestTheFakeAndGitReadTheSameRepository(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			at := impl.at

			main, mainErr := impl.port.MainWorktree(t.Context())
			common, commonErr := impl.port.CommonDir(t.Context())

			require.NoError(t, mainErr)
			assert.Equal(t, at("main"), main.Path)
			require.NoError(t, commonErr)
			assert.Equal(t, filepath.Join(at("main"), ".git"), common)
			assert.Equal(t, gittest.State{
				Worktrees: []worktree.Worktree{
					{Path: at("main"), Main: true, State: worktree.OnBranch{Branch: "main"}},
					{Path: at("detached"), State: worktree.Detached{}},
					{Path: at("dirty"), State: worktree.OnBranch{Branch: "dirty"}},
					{Path: at("gone"), State: worktree.OnBranch{Branch: "gone"}, Prunable: true},
					{Path: at("linked"), State: worktree.OnBranch{Branch: "linked"}},
					{Path: at("locked"), State: worktree.OnBranch{Branch: "locked"}, Locked: true},
					{Path: at("locked-gone"), State: worktree.OnBranch{Branch: "locked-gone"}, Locked: true},
					{Path: at("rebasing"), State: worktree.Rebasing{Branch: "rebasing"}},
				},
				Branches: worktree.Branches{
					Local:  named("dirty", "diverged", "feature", "gone", "linked", "locked", "locked-gone", "main", "rebasing", "unmerged"),
					Remote: named("diverged", "feature", "main", "remote-only"),
				},
			}, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitLocateAPathAsGitWritesIt(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			cases := []struct {
				path string
				want worktree.Location
			}{
				{path: filepath.Join(impl.root, "main"), want: worktree.Location{Path: impl.at("main"), Exists: true}},
				{path: filepath.Join(impl.root, "taken"), want: worktree.Location{Path: impl.at("taken"), Exists: true}},
				{path: filepath.Join(impl.root, "gone"), want: worktree.Location{Path: impl.at("gone")}},
				{path: filepath.Join(impl.root, "new", "topic"), want: worktree.Location{Path: filepath.Join(impl.at("new"), "topic")}},
				{path: "../linked", want: worktree.Location{Path: impl.at("linked"), Exists: true}},
			}
			for _, c := range cases {
				location, err := impl.port.Locate(t.Context(), c.path)

				require.NoError(t, err)
				assert.Equal(t, c.want, location, c.path)
			}
		})
	}
}

func TestTheFakeAndGitUndoAnAddOnANewBranchToWhereTheyStarted(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			before := gittest.StateOf(t, impl.port)
			path := filepath.Join(impl.root, "topic")

			require.NoError(t, impl.port.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.NewBranch{Name: "topic", From: worktree.RemoteBranch{Name: "main"}}}))
			added := gittest.StateOf(t, impl.port)
			leaveAFile(t, path)
			require.NoError(t, impl.port.RemoveWorktree(t.Context(), path))
			require.NoError(t, impl.port.DeleteBranch(t.Context(), "topic"))

			assert.Equal(t, append(before.Worktrees, worktree.Worktree{Path: impl.at("topic"), State: worktree.OnBranch{Branch: "topic"}}), added.Worktrees)
			assert.Contains(t, gittest.Names(added.Branches.Local), "topic")
			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitUndoAnAddOfAnExistingBranchKeepingTheBranch(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			before := gittest.StateOf(t, impl.port)
			path := filepath.Join(impl.root, "feature")

			require.NoError(t, impl.port.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.ExistingBranch{Name: "feature"}}))
			leaveAFile(t, path)
			require.NoError(t, impl.port.RemoveWorktree(t.Context(), path))

			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitStartATrackingBranchWhereTheRemoteOneIs(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			before := gittest.StateOf(t, impl.port)
			path := filepath.Join(impl.root, "resumed")

			added := impl.port.AddWorktree(t.Context(), worktree.Addition{Path: path, Checkout: worktree.TrackingBranch{Name: "remote-only"}})
			twice := impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "again"), Checkout: worktree.TrackingBranch{Name: "feature"}})

			require.NoError(t, added)
			assert.Equal(t, tip(t, impl.port, worktree.RemoteBranch{Name: "remote-only"}), tip(t, impl.port, worktree.LocalBranch{Name: "remote-only"}))
			assert.Contains(t, paths(gittest.StateOf(t, impl.port).Worktrees), impl.at("resumed"))
			require.Error(t, twice)
			leaveAFile(t, path)
			require.NoError(t, impl.port.RemoveWorktree(t.Context(), path))
			require.NoError(t, impl.port.DeleteBranch(t.Context(), "remote-only"))
			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitDropTheBranchAnAddRefusedForItsPathLeft(t *testing.T) {
	for _, impl := range implementations(t) {
		for _, folder := range []string{"taken", "gone"} {
			t.Run(impl.name+"_"+folder, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, folder), Checkout: worktree.NewBranch{Name: "into-" + folder, From: worktree.LocalBranch{Name: "main"}}})

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}

func TestTheFakeAndGitKeepABranchThatExistedBeforeARefusedAdd(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			before := gittest.StateOf(t, impl.port)

			err := impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "taken"), Checkout: worktree.NewBranch{Name: "feature", From: worktree.LocalBranch{Name: "main"}}})

			require.Error(t, err)
			assert.Equal(t, before, gittest.StateOf(t, impl.port))
		})
	}
}

func TestTheFakeAndGitRemoveAWorktreeWithChangesAndOneWhoseDirectoryIsGone(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			require.NoError(t, impl.port.RemoveWorktree(t.Context(), filepath.Join(impl.root, "dirty")))
			require.NoError(t, impl.port.RemoveWorktree(t.Context(), filepath.Join(impl.root, "gone")))

			left := paths(gittest.StateOf(t, impl.port).Worktrees)
			assert.NotContains(t, left, impl.at("dirty"))
			assert.NotContains(t, left, impl.at("gone"))
		})
	}
}

func TestTheFakeAndGitRefuseTheSameWorktreeChanges(t *testing.T) {
	refusals := []struct {
		name string
		call func(t *testing.T, impl implementation) error
	}{
		{name: "branch_checked_out_elsewhere", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.ExistingBranch{Name: "main"}})
		}},
		{name: "rebasing_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.ExistingBranch{Name: "rebasing"}})
		}},
		{name: "remote_only_branch", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.ExistingBranch{Name: "remote-only"}})
		}},
		{name: "existing_branch_taken_folder", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "taken"), Checkout: worktree.ExistingBranch{Name: "feature"}})
		}},
		{name: "new_branch_exists", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.NewBranch{Name: "feature", From: worktree.LocalBranch{Name: "main"}}})
		}},
		{name: "missing_start_point", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.NewBranch{Name: "topic", From: worktree.RemoteBranch{Name: "nope"}}})
		}},
		{name: "existing_branch_missing", call: func(t *testing.T, impl implementation) error {
			return impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "new"), Checkout: worktree.ExistingBranch{Name: "nope"}})
		}},
		{name: "remove_main_worktree", call: func(t *testing.T, impl implementation) error {
			return impl.port.RemoveWorktree(t.Context(), filepath.Join(impl.root, "main"))
		}},
		{name: "remove_non_worktree", call: func(t *testing.T, impl implementation) error {
			return impl.port.RemoveWorktree(t.Context(), filepath.Join(impl.root, "taken"))
		}},
	}
	for _, impl := range implementations(t) {
		for _, refusal := range refusals {
			t.Run(impl.name+"_"+refusal.name, func(t *testing.T) {
				before := gittest.StateOf(t, impl.port)

				err := refusal.call(t, impl)

				require.Error(t, err)
				assert.Equal(t, before, gittest.StateOf(t, impl.port))
			})
		}
	}
}

func TestTheFakeAndGitAddIntoAnEmptyFolder(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			location, err := impl.port.Locate(t.Context(), filepath.Join(impl.root, "empty"))
			require.NoError(t, err)
			require.True(t, location.Exists)

			err = impl.port.AddWorktree(t.Context(), worktree.Addition{Path: filepath.Join(impl.root, "empty"), Checkout: worktree.NewBranch{Name: "into-empty", From: worktree.LocalBranch{Name: "main"}}})

			require.NoError(t, err)
			assert.Contains(t, gittest.StateOf(t, impl.port).Worktrees, worktree.Worktree{Path: impl.at("empty"), State: worktree.OnBranch{Branch: "into-empty"}})
		})
	}
}

func TestTheFakeLocatesARelativePathFromTheRepositoryDirectory(t *testing.T) {
	root := gittest.Symlinked(t)
	params := seed(root)
	params.Dir = filepath.Join(root, "linked")
	fake := gittest.NewFake(params)

	location, err := fake.Locate(t.Context(), "../taken")

	require.NoError(t, err)
	assert.Equal(t, worktree.Location{Path: under(t, root)("taken"), Exists: true}, location)
}
