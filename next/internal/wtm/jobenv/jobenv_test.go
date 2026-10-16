package jobenv

import (
	"maps"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

func TestAWorktreeLearnsItsIdentityItsOffsetAndItsComposeProject(t *testing.T) {
	cases := []struct {
		name   string
		params Params
		want   map[string]string
	}{
		{
			name:   "the main branch on ordinal 0 keeps the base ports",
			params: Params{Branch: "main", ProjectDir: "/src/myproject", Run: run.Config{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "main", "WTM_BRANCH": "main", "WTM_ORDINAL": "0", "WTM_PORT_OFFSET": "0", "COMPOSE_PROJECT_NAME": "myproject-main", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "the offset is the ordinal times the block",
			params: Params{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 3, Run: run.Config{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "3", "WTM_PORT_OFFSET": "30", "COMPOSE_PROJECT_NAME": "myproject-feat-x", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "no block falls back to the default",
			params: Params{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 2},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "2", "WTM_PORT_OFFSET": "20", "COMPOSE_PROJECT_NAME": "myproject-feat-x", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "the repository is made a host label and a slug",
			params: Params{Branch: "feat/x", ProjectDir: "/src/My.App", Ordinal: 1, Run: run.Config{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "1", "WTM_PORT_OFFSET": "10", "COMPOSE_PROJECT_NAME": "my-app-feat-x", "WTM_PROJECT": "my-app", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "verbatim runs on the base ports with no compose project made up",
			params: Params{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 3, Isolation: run.Verbatim, Run: run.Config{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "3", "WTM_PORT_OFFSET": "0", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "verbatim"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, JobEnv(tc.params))
		})
	}
}

func TestALinkedWorktreesJobsRunOffsetByItsNumber(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "launched-from-another-worktree")

	env := JobEnv(shopParams("feat/a", 2, ""))

	assert.Equal(t, "20", env["WTM_PORT_OFFSET"])
	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
	assert.Equal(t, "3020", env["http"])
}

func TestAVerbatimWorktreesJobsRunOnTheBasePorts(t *testing.T) {
	assert.Equal(t, "3000", JobEnv(shopParams("feat/a", 2, run.Verbatim))["http"])
}

func TestTheKeysAreEveryKeyAJobEnvironmentHoldsBesideItsPorts(t *testing.T) {
	env := JobEnv(Params{Branch: "feat/x", ProjectDir: "/src/app"})

	assert.ElementsMatch(t, Keys(), slices.Collect(maps.Keys(env)))
}
