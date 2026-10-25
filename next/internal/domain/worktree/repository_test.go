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

func TestOnlyABranchOfALinkedWorktreeIsCountedAgainstAnExistingBase(t *testing.T) {
	repo := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}, Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main"}}}}}
	missing := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "trunk"}}}

	assert.True(t, repo.CountedAgainstBase(Worktree{State: OnBranch{Branch: "feat/a"}}))
	assert.False(t, repo.CountedAgainstBase(Worktree{Main: true, State: OnBranch{Branch: "main"}}))
	assert.False(t, repo.CountedAgainstBase(Worktree{State: Detached{}}))
	assert.False(t, missing.CountedAgainstBase(Worktree{State: OnBranch{Branch: "feat/a"}}))
}

func TestTheBaseBranchIsMissingOnlyWhenConfiguredWithNoLocalBranch(t *testing.T) {
	repo := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}, Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main"}}}}}
	missing := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "trunk"}}}

	base, gone := missing.MissingBase()
	assert.Equal(t, "trunk", base)
	assert.True(t, gone)
	_, gone = repo.MissingBase()
	assert.False(t, gone)
	_, gone = Repository{}.MissingBase()
	assert.False(t, gone, "no base branch configured is none missing")
}

func TestThePlannedIsolationIsNoneOnlyWhenRunTomlIsolatesNothing(t *testing.T) {
	cases := map[string]struct {
		repo Repository
		want project.Isolation
	}{
		"isolates":   {repo: Repository{Run: project.Run{EnvPorts: []project.EnvPortLink{{}}}}, want: project.Verbatim},
		"nothing":    {repo: Repository{}},
		"unreadable": {repo: Repository{RunErr: errors.New("run.toml: line 3")}, want: project.Verbatim},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, c.repo.PlannedIsolation(project.Verbatim))
		})
	}
}

func TestAWorktreeIsCountedAgainstItsParentOrForARootTheBase(t *testing.T) {
	repo := Repository{Config: project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}, Snapshot: Snapshot{Branches: Branches{Local: []Branch{{Name: "main"}, {Name: "feat/a"}, {Name: "feat/a-ui"}, {Name: "root"}, {Name: "x"}, {Name: "y"}}}}}
	on := func(branch string) Worktree { return Worktree{State: OnBranch{Branch: branch}} }
	worktrees := []Worktree{{Main: true, State: OnBranch{Branch: "main"}}, on("feat/a"), on("feat/a-ui"), on("root"), on("gone"), on("x"), on("y")}
	stack := StackOf(worktrees, map[string]Meta{"feat/a": {SourceBranch: "main"}, "feat/a-ui": {SourceBranch: "feat/a"}, "gone": {SourceBranch: "nowhere"}, "x": {SourceBranch: "y"}, "y": {SourceBranch: "x"}}, "main")
	cases := map[string]struct {
		w       Worktree
		against string
		counted bool
	}{
		"child":       {w: on("feat/a-ui"), against: "feat/a", counted: true},
		"root":        {w: on("root"), against: "main", counted: true},
		"main":        {w: worktrees[0], against: "main"},
		"no_branch":   {w: Worktree{State: Detached{}}, against: "main"},
		"cycle":       {w: on("x"), against: "main"},
		"parent_gone": {w: on("gone"), against: "nowhere"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			against, counted := repo.CountedAgainstParent(stack, c.w)

			assert.Equal(t, c.against, against)
			assert.Equal(t, c.counted, counted)
		})
	}
}
