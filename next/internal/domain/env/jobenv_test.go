package env

import (
	"maps"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"

	"github.com/stretchr/testify/assert"
)

func TestAWorktreeLearnsItsIdentityItsOffsetAndItsComposeProject(t *testing.T) {
	cases := []struct {
		name   string
		params JobParams
		want   map[string]string
	}{
		{
			name:   "main_ordinal_zero_base_ports",
			params: JobParams{Branch: "main", ProjectDir: "/src/myproject", Run: project.Run{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "main", "WTM_BRANCH": "main", "WTM_ORDINAL": "0", "WTM_PORT_OFFSET": "0", "COMPOSE_PROJECT_NAME": "myproject-main", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "offset_ordinal_times_block",
			params: JobParams{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 3, Run: project.Run{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "3", "WTM_PORT_OFFSET": "30", "COMPOSE_PROJECT_NAME": "myproject-feat-x", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "no_block_default",
			params: JobParams{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 2},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "2", "WTM_PORT_OFFSET": "20", "COMPOSE_PROJECT_NAME": "myproject-feat-x", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "repository_host_label_slug",
			params: JobParams{Branch: "feat/x", ProjectDir: "/src/My.App", Ordinal: 1, Run: project.Run{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "1", "WTM_PORT_OFFSET": "10", "COMPOSE_PROJECT_NAME": "my-app-feat-x", "WTM_PROJECT": "my-app", "WTM_ISOLATION": "isolated"},
		},
		{
			name:   "verbatim_base_ports_no_project",
			params: JobParams{Branch: "feat/x", ProjectDir: "/src/myproject", Ordinal: 3, Isolation: project.Verbatim, Run: project.Run{PortOffsetBlock: 10}},
			want:   map[string]string{"WTM_WORKTREE": "feat-x", "WTM_BRANCH": "feat/x", "WTM_ORDINAL": "3", "WTM_PORT_OFFSET": "0", "WTM_PROJECT": "myproject", "WTM_ISOLATION": "verbatim"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, jobEnv(tc.params))
		})
	}
}

func TestALinkedWorktreesJobsRunOffsetByItsNumber(t *testing.T) {
	t.Setenv("COMPOSE_PROJECT_NAME", "launched-from-another-worktree")

	env := jobEnv(shopParams("feat/a", 2, ""))

	assert.Equal(t, "20", env["WTM_PORT_OFFSET"])
	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
	assert.Equal(t, "3020", env["http"])
}

func TestAVerbatimWorktreesJobsRunOnTheBasePorts(t *testing.T) {
	assert.Equal(t, "3000", jobEnv(shopParams("feat/a", 2, project.Verbatim))["http"])
}

func TestTheKeysAreEveryKeyAJobEnvironmentHoldsBesideItsPorts(t *testing.T) {
	env := jobEnv(JobParams{Branch: "feat/x", ProjectDir: "/src/app"})

	assert.ElementsMatch(t, JobKeys(), slices.Collect(maps.Keys(env)))
}

func TestTheOffsetBlockIsTheDeclaredOneElseTheDefault(t *testing.T) {
	cases := []struct {
		name   string
		config project.Run
		want   int
	}{
		{"unset_default", project.Run{}, defaultOffsetBlock},
		{"negative_block_default", project.Run{PortOffsetBlock: -5}, defaultOffsetBlock},
		{"declared_wins", project.Run{PortOffsetBlock: 100}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, OffsetBlock(tc.config))
		})
	}
}

func TestALifecyclePortOfASharedJobIsNotShifted(t *testing.T) {
	config := project.Run{PortOffsetBlock: 10, Jobs: []project.Job{
		{Name: "db", Scope: project.Shared, Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "web", Ports: map[string]int{"WEB_PORT": 3000}},
	}}

	assert.Equal(t, map[string]int{"POSTGRES_PORT": 5432, "WEB_PORT": 3030}, JobParams{Ordinal: 3, Run: config}.ports())
}

func TestAPortNameTwoJobsDeclareHasNoLifecycleAnswer(t *testing.T) {
	config := project.Run{PortOffsetBlock: 10, Jobs: []project.Job{
		{Name: "api", Ports: map[string]int{"PORT": 4000}},
		{Name: "web", Ports: map[string]int{"PORT": 3000, "HMR_PORT": 24678}},
	}}

	assert.Equal(t, map[string]int{"HMR_PORT": 24688}, JobParams{Ordinal: 1, Run: config}.ports())
	assert.Empty(t, JobParams{Ordinal: 1}.ports())
}

func TestAResolvedPortShiftsUnlessTheJobIsShared(t *testing.T) {
	assert.Equal(t, 5452, resolvedPort(resolvedPortParams{Base: 5432, Offset: 20}))
	assert.Equal(t, 5432, resolvedPort(resolvedPortParams{Base: 5432, Offset: 20, Shared: true}))
}

func TestTheSharedJobsAreThoseScopedShared(t *testing.T) {
	assert.Equal(t, map[string]bool{"db": true}, sharedJobs(project.Run{Jobs: []project.Job{{Name: "db", Scope: project.Shared}, {Name: "web"}}}))
}

func TestTheDeclaredComposeProjectIsTheFirstEnvToNameOne(t *testing.T) {
	files := [][]dotenv.Line{dotenv.Parse([]byte("PORT=1\n")), dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=\nCOMPOSE_PROJECT_NAME=later\n"))}

	assert.Empty(t, declaredComposeProject(files))
	assert.Equal(t, "stack", declaredComposeProject(append(files, dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=stack\n")))))
}

func TestTheComposeDirectoriesAreWhereAStackStartsOnceEach(t *testing.T) {
	config := project.Run{Jobs: []project.Job{
		{Name: "db", Cmd: "docker compose up -d db", Cwd: "infra"},
		{Name: "cache", Cmd: "./up.sh", Stop: "docker-compose down", Cwd: "infra"},
		{Name: "root", Cmd: "docker compose up"},
		{Name: "web", Cmd: "pnpm dev", Cwd: "apps/web"},
	}}

	assert.Equal(t, []string{"infra", "."}, composeDirs(config))
	assert.Empty(t, composeDirs(project.Run{}))
}

func TestTheComposeEnvPathsAreTheEnvOfEachComposeDirectoryElseTheRoot(t *testing.T) {
	config := project.Run{Jobs: []project.Job{{Name: "db", Cmd: "docker compose up -d", Cwd: "infra"}, {Name: "root", Cmd: "docker compose up"}}}

	assert.Equal(t, []string{"infra/.env", ".env"}, ComposeEnvPaths(config))
	assert.Equal(t, []string{".env"}, ComposeEnvPaths(project.Run{}))
}

func TestAHookLearnsTheJobEnvironmentOfItsWorktree(t *testing.T) {
	env := HookEnv(shopParams("feat/a", 2, ""), nil)

	assert.Equal(t, "20", env["WTM_PORT_OFFSET"])
	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
}

func TestAHookRunsUnderTheComposeProjectItsWorktreesEnvNames(t *testing.T) {
	env := HookEnv(
		shopParams("feat/a", 2, ""),
		[][]dotenv.Line{dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=renamed\n"))},
	)

	assert.Equal(t, "renamed", env["COMPOSE_PROJECT_NAME"])
}

func TestAHookKeepsTheDerivedComposeProjectWhenNoEnvNamesOne(t *testing.T) {
	env := HookEnv(
		shopParams("feat/a", 2, ""),
		[][]dotenv.Line{dotenv.Parse([]byte("PORT=1\nCOMPOSE_PROJECT_NAME=\n"))},
	)

	assert.Equal(t, shopProject+"-feat-a", env["COMPOSE_PROJECT_NAME"])
}

func TestHooksReadTheRunEnvOnlyOfAWorktreeThatChoseAndAComposeStack(t *testing.T) {
	compose := project.Run{Jobs: []project.Job{{Name: "db", Cmd: "docker compose up -d"}}}
	script := project.Run{Jobs: []project.Job{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}
	cases := []struct {
		name      string
		run       project.Run
		isolation project.Isolation
		want      bool
	}{
		{"isolated_compose_job", compose, project.Isolated, true},
		{"verbatim_compose_job", compose, project.Verbatim, true},
		{"never_chose", compose, "", false},
		{"no_compose_job", script, project.Isolated, false},
		{"no_run_config", project.Run{}, project.Isolated, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, HooksReadRun(tc.run, tc.isolation))
		})
	}
}

func TestAHookEnvReadsEachComposeEnvFileOfItsWorktreeAndFailsWithTheirRead(t *testing.T) {
	job := JobParams{Branch: "feat/a", ProjectDir: "/repo/acme", Ordinal: 1, Isolation: project.Isolated, Run: project.Run{Jobs: []project.Job{{Name: "db", Cmd: "docker compose up", Cwd: "infra"}}}}
	var asked []string
	read := func(path string) ([]dotenv.Line, error) {
		asked = append(asked, path)
		return dotenv.Parse([]byte("COMPOSE_PROJECT_NAME=mine\n")), nil
	}

	got, err := ReadHookEnv(job, "/repo/acme.trees/feat-a", read)
	_, failed := ReadHookEnv(job, "/repo/acme.trees/feat-a", func(string) ([]dotenv.Line, error) { return nil, errUnreadable })

	assert.NoError(t, err)
	assert.Equal(t, []string{"/repo/acme.trees/feat-a/infra/.env"}, asked)
	assert.Equal(t, "mine", got[ComposeProjectKey])
	assert.ErrorIs(t, failed, errUnreadable)
}
