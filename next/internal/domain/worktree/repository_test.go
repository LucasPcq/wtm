package worktree

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestANewWorktreeLandsUnderTheBasePathFromTheMainCheckout(t *testing.T) {
	repo := Repository{MainPath: "/repo/main", Config: project.Config{Worktrees: project.Worktrees{BasePath: "../.trees"}}}

	assert.Equal(t, filepath.Join("/repo", ".trees", PathSegment("feat/x")), repo.PathOf("feat/x"))
}

func TestOnlyARunThatNamesWorktreesMakesTheirNamesClash(t *testing.T) {
	live := Snapshot{Worktrees: []Worktree{{Path: "/repo", State: OnBranch{Branch: "main"}}}}
	cases := map[string]struct {
		run    project.Run
		runErr error
		want   []string
	}{
		"no_jobs_nor_profiles": {project.Run{}, nil, nil},
		"jobs":                 {project.Run{Jobs: []project.Job{{Name: "web"}}}, nil, []string{"main"}},
		"profiles":             {project.Run{Profiles: []project.Profile{{}}}, nil, []string{"main"}},
		"run_failed":           {project.Run{}, errors.New("bad run.toml"), []string{"main"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			repo := Repository{Run: c.run, RunErr: c.runErr, Snapshot: live}
			assert.Equal(t, c.want != nil, repo.NamesWorktrees())
			assert.Equal(t, c.want, repo.NamedLive())
		})
	}
}

func TestTheBaseBranchReadsAsTheDefaultSource(t *testing.T) {
	repo := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}}

	assert.Equal(t, "main (default)", repo.SourceLabel("main"))
	assert.Empty(t, repo.SourceLabel("feat/a"))
	assert.Empty(t, Repository{}.SourceLabel(""))
}

func TestOnlyABranchOfALinkedWorktreeIsCountedAgainstAnExistingBase(t *testing.T) {
	repo := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}, Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main"}}}}}
	missing := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "trunk"}}}

	assert.True(t, repo.CountedAgainstBase(Worktree{State: OnBranch{Branch: "feat/a"}}))
	assert.False(t, repo.CountedAgainstBase(Worktree{Main: true, State: OnBranch{Branch: "main"}}))
	assert.False(t, repo.CountedAgainstBase(Worktree{State: Detached{}}))
	assert.False(t, missing.CountedAgainstBase(Worktree{State: OnBranch{Branch: "feat/a"}}))
	base, gone := missing.MissingBase()
	assert.Equal(t, "trunk", base)
	assert.True(t, gone)
	_, gone = repo.MissingBase()
	assert.False(t, gone)
	_, gone = Repository{}.MissingBase()
	assert.False(t, gone, "no base branch configured is none missing")
}
