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
		"no jobs nor profiles": {project.Run{}, nil, nil},
		"jobs":                 {project.Run{Jobs: []project.Job{{Name: "web"}}}, nil, []string{"main"}},
		"profiles":             {project.Run{Profiles: []project.Profile{{}}}, nil, []string{"main"}},
		"a run that failed":    {project.Run{}, errors.New("bad run.toml"), []string{"main"}},
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
